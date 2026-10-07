package legion

import (
	"dfolan/internal/catalog"
	"dfolan/internal/catalog/pvf"
	"encoding/binary"
	"testing"
	"time"
)

func runtimeNative(t *testing.T, hard bool) (*BakalOpening, time.Time) {
	t.Helper()
	r, e := catalog.ImportBakalRaid(catalog.OpenNativeArchive(t))
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { r.DungeonCatalog.CloseMapSource() })
	o, e := PrepareBakalOpening(r, hard)
	if e != nil {
		t.Fatal(e)
	}
	now := time.Unix(1800000000, 0)
	o.Start("owned", now)
	ready := now.Add(time.Duration(r.StartDelaySecs) * time.Second)
	o.Tick(ready)
	if e := o.EventError(); e != nil {
		t.Fatal(e)
	}
	return o, ready
}
func TestBakalRuntimeInitializationNeverPublishesDeadBoss(t *testing.T) {
	o, now := runtimeNative(t, false)
	if o.runtime == nil {
		t.Fatal("native interpreter not active")
	}
	for _, row := range o.eventRoster() {
		if row.MaxHP == 0 {
			t.Fatal("source INIT published zero health")
		}
		for _, id := range row.Buffs {
			if id > 25 {
				t.Fatal("unknown buff slot")
			}
		}
	}
	for _, n := range o.buffs {
		if n != 2 {
			t.Fatal("INIT duplicated commander inventory")
		}
	}
	o.Tick(now.Add(time.Second))
	if o.EventError() != nil {
		t.Fatal(o.EventError())
	}
}
func TestBakalRuntimeNormalAndHardAwakeningSchedules(t *testing.T) {
	for _, hard := range []bool{false, true} {
		o, ready := runtimeNative(t, hard)
		at := ready.Add(time.Second)
		if _, e := o.EnterDungeon(100003157, 23, at); e != nil {
			t.Fatal(e)
		}
		o.Tick(at)
		if o.EventError() != nil {
			t.Fatal(o.EventError())
		}
		awake := 0
		for name, v := range o.symbolValues {
			if len(name) > 8 && name[len(name)-8:] == " AWAKEN]" && v == 1 {
				awake++
			}
		}
		want := 1
		if hard {
			want = 3
		}
		if awake != want {
			t.Fatalf("hard%v awake%d want%d", hard, awake, want)
		}
		before := len(o.runtime.timers)
		o.EnterDungeon(100003154, 7, at.Add(time.Second))
		if len(o.runtime.timers) != before {
			t.Fatal("second hatchery rescheduled awakenings")
		}
		if !hard {
			count := 0
			for key, due := range o.runtime.timers {
				if key[0] == 21 || key[0] == 22 || key[0] == 23 {
					if due != at.Add(900*time.Second) && due != at.Add(1800*time.Second) {
						t.Fatal("wrong source awakening delay")
					}
					count++
				}
			}
			if count != 2 {
				t.Fatal("not two distinct pending dragons")
			}
		}
	}
}
func TestBakalRuntimeRespawnBuffExpiryAndIdempotence(t *testing.T) {
	o, ready := runtimeNative(t, false)
	// Isolate reward/respawn timing from unattended reinforcements reaching
	// camp. The changed source penalty must be consumed by the executor.
	for i := range o.script.Events {
		for j := range o.script.Events[i].Behavior {
			ins := &o.script.Events[i].Behavior[j]
			if ins.Op == "[INCREASE BAKAL ANGER]" {
				ins.Args[1].Value = 0
			}
		}
	}
	o.Tick(ready.Add(10 * time.Second))
	if o.EventError() != nil {
		t.Fatal(o.EventError())
	}
	loc := uint32(0)
	for p, k := range o.placements {
		if k == "swan" {
			loc = p
		}
	}
	if loc == 0 {
		t.Fatal("no reserved swan")
	}
	d := BakalDungeonOfLocation(o.rules, loc)
	at := ready.Add(20 * time.Second)
	o.EnterDungeon(d, loc, at)
	o.LoadingDone(d)
	picked := append([]string(nil), o.runtime.selected[loc]...)
	frames, e := o.DefeatMonster(d, loc, at)
	if e != nil {
		t.Fatal(e)
	}
	found := false
	for _, f := range frames {
		if f.ID == 2285 {
			for i := 0; i < 20; i++ {
				if binary.LittleEndian.Uint32(f.Body[13+8*i:]) != 25 && binary.LittleEndian.Uint32(f.Body[17+8*i:]) > uint32(at.Unix()) {
					found = true
				}
			}
		}
	}
	if !found {
		t.Fatal("no immune/damage lease in native party frame")
	}
	if len(picked) != 2 {
		t.Fatal("source selected buffs not owned")
	}
	repeat, e := o.DefeatMonster(d, loc, at)
	if e != nil || len(repeat) != 0 {
		t.Fatal("duplicate death grants buff")
	}
	o.Tick(at.Add(299 * time.Second))
	for _, k := range o.placements {
		if k == "swan" {
			t.Fatal("early source respawn")
		}
	}
	o.Tick(at.Add(300 * time.Second))
	if o.EventError() != nil {
		t.Fatal(o.EventError())
	}
	born := false
	for p, k := range o.placements {
		if k == "swan" {
			born = true
			if o.defeatedLocations[p] {
				t.Fatal("source recreation retained death suppression")
			}
		}
	}
	if !born {
		t.Fatalf("source fixed leader never respawned: stage=%v placements=%v tasks=%v symbols=%v", o.stage, o.placements, o.runtime.tasks, o.symbolValues)
	}
	o.Tick(at.Add(481 * time.Second))
	if o.EventError() != nil {
		t.Fatal(o.EventError())
	}
	for name := range o.runtime.party {
		if name == "SkasaDragonImmune" || name == "HismaDragonImmune" {
			t.Fatal("expired resistance retained")
		}
	}
}
func TestBakalRuntimeMovingWaveAndRollback(t *testing.T) {
	o, ready := runtimeNative(t, false)
	o.Tick(ready.Add(5 * time.Second))
	if o.EventError() != nil {
		t.Fatal(o.EventError())
	}
	kind := o.placements[8]
	if kind != "brute" && kind != "steel dragon" {
		t.Fatal("source weighted wave absent")
	}
	o.Tick(ready.Add(65 * time.Second))
	if o.EventError() != nil {
		t.Fatal(o.EventError())
	}
	if o.placements[9] != kind {
		t.Fatal("wave did not follow source MOVE")
	}
	for _, k := range o.placements {
		if k == "zamir" {
			kind = "zamir"
		}
	}
	if kind != "zamir" {
		t.Fatal("source5+60s wave not generated")
	}
	before := o.anger
	_, e := o.runEvent(bakalSignal{op: "[ON CHANGE SYMBOL]", name: "[BAKAL ANGER]"}, ready)
	if e != nil {
		t.Fatal(e)
	}
	old := o.script.Events
	o.script.Events = append(o.script.Events, catalog.BakalScriptEvent{Trigger: []catalog.BakalScriptInstruction{{Op: "[ON ENTER DUNGEON]", Args: []pvf.Token{{Type: 0, Value: 100003157}}}}, Behavior: []catalog.BakalScriptInstruction{{Op: "invalid"}}})
	_, e = o.runEvent(bakalSignal{op: "[ON ENTER DUNGEON]", id: 100003157}, ready)
	o.script.Events = old
	if e == nil || o.anger != before {
		t.Fatal("failed source event partially applied")
	}
}

func TestBakalRuntimeCommanderCooldownAndOwnedLease(t *testing.T) {
	o, ready := runtimeNative(t, false)
	at := ready.Add(time.Second)
	frames, e := o.UseRaidBuffAt(3, at)
	if e != nil {
		t.Fatal(e)
	}
	seen := false
	for _, f := range frames {
		if f.ID == 2288 {
			if binary.LittleEndian.Uint32(f.Body[5:]) != 3 || f.Body[3] != 1 {
				t.Fatal("wrong used index/inventory")
			}
			seen = true
		}
	}
	if !seen || o.runtime.party["RaidBuffInvincible"] != at.Add(10*time.Second) {
		t.Fatal("source duration not applied")
	}
	before := o.buffs
	if _, e = o.UseRaidBuffAt(3, at.Add(time.Second)); e == nil || o.buffs != before {
		t.Fatal("cooldown request changed state")
	}
	if _, e = o.UseRaidBuffAt(2, at.Add(time.Second)); e == nil {
		t.Fatal("shared source cooldown ignored")
	}
	if _, e = o.UseRaidBuffAt(2, at.Add(3*time.Second)); e != nil {
		t.Fatal(e)
	}
	o.Tick(at.Add(10 * time.Second))
	if o.EventError() != nil {
		t.Fatal(o.EventError())
	}
	if _, ok := o.runtime.party["RaidBuffInvincible"]; ok {
		t.Fatal("expired commander lease retained")
	}
	// A changed source cooldown changes enforcement without a Go time table.
	d := o.rules.BuffDefinitions["RaidBuffInvincible"]
	d.Cooltime = 5
	o.rules.BuffDefinitions[d.Kind] = d
	o.runtime.cooldown[d.Kind] = time.Time{}
	if _, e = o.UseRaidBuffAt(3, at.Add(11*time.Second)); e != nil {
		t.Fatal(e)
	}
	if o.runtime.cooldown[d.Kind] != at.Add(16*time.Second) {
		t.Fatal("changed source cooldown ignored")
	}
}
