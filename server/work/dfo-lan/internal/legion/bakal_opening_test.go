// BakalOpening replay tests: the state machine is driven along the B-layer
// clear run (241-event timeline 2026-10-05 12:29:29→12:39:42) and every
// deterministic frame is asserted byte-exact against the wire fixtures; the
// frames whose captures carry server-minted member counters (N2285 room
// locations beyond the first, the return-to-camp) are asserted on their
// location prefix only. Failed-run refusal reasons assert against the
// captured texts (testdata/bakal_refusals.json).
package legion

import (
	"bytes"
	"encoding/hex"
	"errors"
	"testing"
	"time"

	"dfolan/internal/catalog"
)

// The handover fixture used state1 for initialization; official capture
// 20261005-015111 uses state0 placement. Preserve source types/locations.
func bakalNativePlacementFixture(t *testing.T) []byte {
	p := append([]byte(nil), bakalFixtureAt(t, "bakal_monsters", "12:29:32.878")...)
	for i := 0; i < int(p[0]); i++ {
		for j := 0; j < 4; j++ {
			p[1+i*19+8+j] = 0
		}
	}
	return p
}

// bakalFixtureAt finds a fixture by name and captured clock time — the
// timeline often repeats a name with different bodies, so occurrence order
// is too fragile to rely on.
func bakalFixtureAt(t *testing.T, name, clock string) []byte {
	t.Helper()
	for _, f := range bakalFixtures(t) {
		if f.Name == name && f.Time == clock {
			got, err := hex.DecodeString(f.PlainHex)
			if err != nil {
				t.Fatalf("fixture %s@%s: bad hex: %v", name, clock, err)
			}
			return got
		}
	}
	t.Fatalf("fixture %s@%s not found", name, clock)
	return nil
}

// bakalAssertFrame checks one state-machine frame against a fixture.
func bakalAssertFrame(t *testing.T, f BakalFrame, name string, want []byte) {
	t.Helper()
	if f.Name != name {
		t.Fatalf("frame name %s, want %s", f.Name, name)
	}
	bakalAssertBytes(t, name, want, f.Body)
}

// bakalAssertLocationPrefix checks the location field of an N2285 party
// frame whose capture carries other raid members' counters — a solo server
// mints the all-unused tail.
func bakalAssertLocationPrefix(t *testing.T, f BakalFrame, name string, location uint32) {
	t.Helper()
	if f.Name != name {
		t.Fatalf("frame name %s, want %s", f.Name, name)
	}
	if len(f.Body) != 173 {
		t.Fatalf("%s: built %d bytes, want the 173B party frame", name, len(f.Body))
	}
	if got := le32(f.Body[5:9]); got != location {
		t.Fatalf("%s: location %d, want %d", name, got, location)
	}
}

// bakalTestRules builds normal-phase rules with the values the catalog tests
// lock against the real PVF (TestImportBakalRaidRules): the replay runs on
// the same numbers the clear run used.
func bakalTestRules() *catalog.BakalRaidRules {
	normal := &catalog.BakalPhaseRules{
		MonsterMaxHP: 10000,
		RaidBuffs: []catalog.BakalBuffGrant{
			{Slot: 0, Count: 2}, {Slot: 1, Count: 2}, {Slot: 2, Count: 2},
			{Slot: 3, Count: 2}, {Slot: 4, Count: 2},
		},
		InitTimers: []catalog.BakalTimerRule{
			{ID: 18, Sub: 0, Secs: 5}, {ID: 19, Sub: 0, Secs: 60},
			{ID: 20, Sub: 0, Secs: 60}, {ID: 27, Sub: 24, Secs: 120},
		},
		AngerInit:               0,
		AngerInitPerSecond:      1,
		AngerEnterPerSecond:     1,
		AngerWindowEndPerSecond: 10,
		AngerWindow:             catalog.BakalTimerRule{ID: 27, Sub: 24, Secs: 120},
		AngerFailThreshold:      8000,
		SettlementTimer:         catalog.BakalTimerRule{ID: 28, Sub: 56, Secs: 180},
		FinalClearDungeon:       100003165,
		SettlementDungeon:       100003149,
		BossHP:                  map[string]int{"bakal": 10000, "sparazzi": 10000, "skasa": 10000, "hisma": 10000},
		EnterBakalDungeon:       100003149,
		EnterBakalTimer:         catalog.BakalTimerRule{ID: 28, Sub: 24, Secs: 600},
	}
	dungeons := make([]catalog.BakalDungeonInfo, 17)
	for i := range dungeons {
		dungeons[i].Index = uint32(100003149 + i)
	}
	dungeons[0].Type = "bakal"    // 149 the Bakal dungeon itself
	dungeons[1].Type = "skasa"    // 150
	dungeons[2].Type = "sparazzi" // 151
	dungeons[3].Type = "hisma"    // 152
	normal.InitMonsters = []catalog.BakalMonsterSpawn{
		{Location: 24, Name: "bakal"}, {Location: 12, Name: "sparazzi"},
		{Location: 26, Name: "skasa"}, {Location: 40, Name: "hisma"},
		{Location: 7, Name: "blona"}, {Location: 47, Name: "nympha"},
		{Location: 23, Name: "basilisk"}, {Location: 25, Name: "gerda"},
	}
	return &catalog.BakalRaidRules{
		StartDelaySecs:    3,
		PhaseTimeOverSecs: 9999,
		NormalPhase:       normal,
		Dungeons:          dungeons,
		Locations: []catalog.BakalLocationInfo{
			{Index: 12, Dungeon: 100003151, Type: "SPARAZZI"},
			{Index: 24, Dungeon: 100003149, Type: "BAKAL"},
			{Index: 56, Dungeon: 100003165},
			{Index: 26, Dungeon: 100003150, Type: "SKASA"},
			{Index: 40, Dungeon: 100003152, Type: "HISMA"},
			{Index: 52, Dungeon: 0, Type: "NORMAL"},
		},
	}
}

func bakalTestDungeonIDs() []uint32 {
	var ids []uint32
	for _, d := range bakalTestRules().Dungeons {
		ids = append(ids, d.Index)
	}
	return ids
}

func TestBakalOpeningReplayTimeline(t *testing.T) {
	o, err := PrepareBakalOpening(bakalTestRules(), false)
	if err != nil {
		t.Fatal(err)
	}
	if o.Remaining() != 9999 {
		t.Fatalf("remaining %d, want the [PHASE TIME OVER] 9999", o.Remaining())
	}
	t0 := time.Unix(1791210000, 145000000) // 12:29:29.145

	// CMD2089 → vote burst (timeline 000–005; the 2089 ACK is dispatch-side).
	frames, err := o.Start("Lansmt", t0)
	if err != nil {
		t.Fatal(err)
	}
	if len(frames) != 5 {
		t.Fatalf("start burst: %d frames, want 5", len(frames))
	}
	bakalAssertFrame(t, frames[0], "bakal_vote_start", bakalFixtureAt(t, "bakal_vote_start", "12:29:29.145"))
	bakalAssertFrame(t, frames[1], "bakal_vote_state", bakalFixtureAt(t, "bakal_vote_state", "12:29:29.145"))
	bakalAssertFrame(t, frames[2], "bakal_real_member_assigned", bakalFixtureAt(t, "bakal_real_member_assigned", "12:29:29.145"))
	bakalAssertFrame(t, frames[3], "bakal_preparing", bakalFixtureAt(t, "bakal_preparing", "12:29:29.145"))
	bakalAssertFrame(t, frames[4], "bakal_countdown", bakalFixtureAt(t, "bakal_countdown", "12:29:29.145"))

	// before the [START DELAY TIME] 3s the run stays preparing.
	if got := o.Tick(t0.Add(2 * time.Second)); len(got) != 0 {
		t.Fatalf("tick inside the start delay published %d frames", len(got))
	}
	if o.Stage() != BakalOpeningPreparing {
		t.Fatalf("stage %d inside the start delay, want preparing", o.Stage())
	}

	// +3s → open burst (timeline 012–017).
	frames = o.Tick(t0.Add(3 * time.Second))
	if len(frames) != 6 {
		t.Fatalf("open burst: %d frames, want 6", len(frames))
	}
	bakalAssertFrame(t, frames[0], "bakal_active", bakalFixtureAt(t, "bakal_active", "12:29:32.878"))
	bakalAssertFrame(t, frames[1], "bakal_party", bakalFixtureAt(t, "bakal_party", "12:29:32.878"))
	bakalAssertFrame(t, frames[2], "bakal_monsters", bakalNativePlacementFixture(t))
	bakalAssertFrame(t, frames[3], "bakal_buffs", bakalFixtureAt(t, "bakal_buffs", "12:29:32.878"))
	bakalAssertFrame(t, frames[4], "bakal_open_dungeons", bakalFixtureAt(t, "bakal_open_dungeons", "12:29:32.878"))
	bakalAssertFrame(t, frames[5], "bakal_remaining", bakalFixtureAt(t, "bakal_remaining", "12:29:32.878"))
	if o.Anger() != 0 {
		t.Fatalf("anger %d at open, want [INCREASE BAKAL ANGER INIT] 0", o.Anger())
	}

	// +1s → the roster repeat (timeline 019, byte-identical to the open
	// roster) and the four boss health rows (timeline 020).
	frames = o.Tick(t0.Add(4 * time.Second))
	if len(frames) != 2 {
		t.Fatalf("health publish: %d frames, want 2", len(frames))
	}
	bakalAssertFrame(t, frames[0], "bakal_script_monsters", bakalNativePlacementFixture(t))
	bakalAssertFrame(t, frames[1], "bakal_script_actor_health", bakalFixtureAt(t, "bakal_script_actor_health", "12:29:33.878"))
	if got := o.Tick(t0.Add(5 * time.Second)); len(got) != 0 {
		t.Fatalf("health publish repeated: %d frames", len(got))
	}

	// anger accrues [INCREASE BAKAL ANGER ENTER] per active second
	// (the open at t0+3s makes t0+45s the 42nd active second).
	o.Tick(t0.Add(45 * time.Second))
	if o.Anger() != 42 {
		t.Fatalf("anger %d after 42 active seconds, want 42", o.Anger())
	}

	// CMD2062 into the field map 100003160 → N2281 + N2285 room 22
	// (timeline 023–024).
	frames, err = o.EnterDungeon(100003160, 22, t0.Add(25*time.Second))
	if err != nil {
		t.Fatal(err)
	}
	if len(frames) != 2 {
		t.Fatalf("dungeon entry: %d frames, want 2", len(frames))
	}
	bakalAssertFrame(t, frames[0], "bakal_portal_ready", bakalFixtureAt(t, "bakal_portal_ready", "12:29:54.548"))
	bakalAssertFrame(t, frames[1], "bakal_room_party_location", bakalFixtureAt(t, "bakal_room_party_location", "12:29:54.554"))

	// scripted warp before the load completes → captured refusal.
	if _, err := o.StageWarp(24); !errors.Is(err, ErrBakalWarpOutsideCombat) {
		t.Fatalf("warp before loading: %v, want the captured refusal", err)
	}
	if err := o.LoadingDone(100003160); err != nil {
		t.Fatal(err)
	}
	if err := o.LoadingDone(100003158); !errors.Is(err, ErrBakalWarpOutsideCombat) {
		t.Fatalf("loading done for a dungeon the member is not inside: %v", err)
	}

	// CMD2070 scripted warp inside the loaded combat → N2285 (timeline 044–045).
	frames, err = o.StageWarp(2)
	if err != nil {
		t.Fatal(err)
	}
	if len(frames) != 1 {
		t.Fatalf("scripted warp: %d frames, want 1", len(frames))
	}
	bakalAssertLocationPrefix(t, frames[0], "bakal_room_party_location", 2)

	// confirmed battle report in the field map → the 20B defeat row
	// (timeline 031) and the captured same-tick buff grant (timeline 032).
	frames, err = o.DefeatMonster(100003160, 19, t0.Add(46*time.Second))
	if err != nil {
		t.Fatal(err)
	}
	if len(frames) != 1 {
		t.Fatalf("field defeat: %d frames, want the defeat row only", len(frames))
	}
	bakalAssertFrame(t, frames[0], "bakal_script_monsters", bakalFixtureAt(t, "bakal_script_monsters", "12:30:15.133"))
	frames = o.GrantBuffs(3)
	if len(frames) != 1 {
		t.Fatalf("explicit buff grant: %d frames, want 1", len(frames))
	}
	bakalAssertFrame(t, frames[0], "bakal_script_buff_inventory", bakalFixtureAt(t, "bakal_script_buff_inventory", "12:30:15.133"))

	// boss clear: hisma map 152 at its raid location → defeat row
	// (timeline 100), N572 single-dungeon state (timeline 102) and the
	// GrantHook's charge grant (timeline 101).
	if _, err := o.EnterDungeon(100003152, 40, t0.Add(170*time.Second)); err != nil {
		t.Fatal(err)
	}
	if err := o.LoadingDone(100003152); err != nil {
		t.Fatal(err)
	}
	o.GrantHook = func(uint32) []int { return []int{0, 1} }
	frames, err = o.DefeatMonster(100003152, 40, t0.Add(177*time.Second))
	if err != nil {
		t.Fatal(err)
	}
	if len(frames) != 3 {
		t.Fatalf("boss clear: %d frames, want defeat + dungeon state + buff grant", len(frames))
	}
	wantDefeat, _ := hex.DecodeString("0100000000280000000100000000000000ffffff") // timeline 100
	bakalAssertFrame(t, frames[0], "bakal_script_monsters", wantDefeat)
	bakalAssertFrame(t, frames[1], "bakal_script_dungeon_states", bakalFixtureAt(t, "bakal_script_dungeon_states", "12:32:26.202"))
	bakalAssertFrame(t, frames[2], "bakal_script_buff_inventory", bakalFixtureAt(t, "bakal_script_buff_inventory", "12:32:26.202"))

	// the cleared boss map no longer authorizes entry.
	if _, err := o.EnterDungeon(100003152, 40, t0.Add(180*time.Second)); err == nil {
		t.Fatal("cleared boss dungeon accepted a second entry")
	}

	// final fight: enter the final dungeon, warp to the Bakal room, HP
	// progress pair (timeline 224–225), the in-room boss row (timeline 228),
	// then the final defeat (timeline 229–232).
	if _, err := o.EnterDungeon(100003149, 24, t0.Add(430*time.Second)); err != nil {
		t.Fatal(err)
	}
	if err := o.LoadingDone(100003149); err != nil {
		t.Fatal(err)
	}
	frames = o.ReportHealth(1, 24, 5000)
	if len(frames) != 2 {
		t.Fatalf("boss HP publish: %d frames, want the row-state pair", len(frames))
	}
	wantProgress, _ := hex.DecodeString("0101000000180000000100000088130000ffffff") // timeline 224
	bakalAssertFrame(t, frames[0], "bakal_script_monsters", wantProgress)
	bakalAssertFrame(t, frames[1], "bakal_script_actor_health", bakalFixtureAt(t, "bakal_script_actor_health", "12:36:34.183"))

	bossRow := BakalFrame{"bakal_source_boss", NotiBakalSourceBoss, BakalSourceBossFrame(1, 5, 0x1001, 109014483, 945, 311)}
	if bossRow.Name != "bakal_source_boss" || len(bossRow.Body) != 34 {
		t.Fatalf("source boss row: %s %dB, want the 34B N2194", bossRow.Name, len(bossRow.Body))
	}
	if bossRow.Body[1] != 1 || bossRow.Body[2] != 5 || le32(bossRow.Body[14:18]) != 100 {
		t.Fatalf("source boss row shape drifted: % x", bossRow.Body)
	}

	finalAt := t0.Add(433 * time.Second) // 12:36:42.047
	frames, err = o.DefeatMonster(100003149, 24, finalAt)
	if err != nil {
		t.Fatal(err)
	}
	if len(frames) != 4 {
		t.Fatalf("final defeat: %d frames, want defeat + final selection + final move + room location", len(frames))
	}
	wantFinalDefeat, _ := hex.DecodeString("0100000000180000000100000000000000ffffff") // timeline 229
	bakalAssertFrame(t, frames[0], "bakal_script_monsters", wantFinalDefeat)
	bakalAssertFrame(t, frames[1], "bakal_final_selection", bakalFixtureAt(t, "bakal_final_selection", "12:36:42.047"))
	bakalAssertFrame(t, frames[2], "bakal_final_move", bakalFixtureAt(t, "bakal_final_move", "12:36:42.047"))
	bakalAssertLocationPrefix(t, frames[3], "bakal_room_party_location", 56) // [SET TIMER] 28 56 180 sub
	if o.Stage() != BakalOpeningFinal {
		t.Fatalf("stage %d after the final defeat, want final", o.Stage())
	}

	// CMD1134 after the settlement window → the final map confirm.
	if err := o.FinalConfirm(BakalFinalMap); err != nil {
		t.Fatal(err)
	}
	if err := o.FinalConfirm(999); err == nil {
		t.Fatal("wrong final map accepted")
	}

	// the settlement window: nothing before +[SET TIMER] 28 56 180.
	if got := o.Tick(finalAt.Add(179 * time.Second)); len(got) != 0 {
		t.Fatalf("settlement published early: %d frames", len(got))
	}
	o.SetRewardInventory([]byte{})
	frames = o.Tick(finalAt.Add(180 * time.Second))
	if len(frames) != 4 {
		t.Fatalf("settlement: %d frames, want return + inventory + clear result + phase ended", len(frames))
	}
	bakalAssertLocationPrefix(t, frames[0], "bakal_script_return_to_camp", BakalCampLocation) // fixture carries other members' counters
	bakalAssertFrame(t, frames[2], "bakal_clear_result", bakalFixtureAt(t, "bakal_clear_result", "12:39:42.954"))
	bakalAssertFrame(t, frames[3], "bakal_script_phase_ended", bakalFixtureAt(t, "bakal_script_phase_ended", "12:39:42.954"))
	if o.Stage() != BakalOpeningEnded {
		t.Fatalf("stage %d after settlement, want ended", o.Stage())
	}
	if got := o.Tick(finalAt.Add(181 * time.Second)); len(got) != 0 {
		t.Fatalf("settlement repeated: %d frames", len(got))
	}

	identity, ok := o.SettlementIdentity(finalAt.Add(180 * time.Second))
	if !ok {
		t.Fatal("settlement identity missing after the settlement burst")
	}
	if identity.Name != "Lansmt" || identity.Hard || identity.FinalDungeon != 100003165 {
		t.Fatalf("settlement identity drifted: %+v", identity)
	}
}

func TestBakalOpeningSettlementSplicesRewardInventory(t *testing.T) {
	o, err := PrepareBakalOpening(bakalTestRules(), false)
	if err != nil {
		t.Fatal(err)
	}
	t0 := time.Unix(1791210000, 0)
	if _, err := o.Start("Lansmt", t0); err != nil {
		t.Fatal(err)
	}
	o.Tick(t0.Add(3 * time.Second))
	o.SetRewardInventory([]byte{0xAA, 0xBB})
	if _, err := o.EnterDungeon(100003149, 24, t0.Add(10*time.Second)); err != nil {
		t.Fatal(err)
	}
	if err := o.LoadingDone(100003149); err != nil {
		t.Fatal(err)
	}
	finalAt := t0.Add(20 * time.Second)
	if _, err := o.DefeatMonster(100003149, 24, finalAt); err != nil {
		t.Fatal(err)
	}
	frames := o.Tick(finalAt.Add(180 * time.Second))
	if len(frames) != 4 {
		t.Fatalf("settlement: %d frames, want return + reward + clear + ended", len(frames))
	}
	if frames[1].Name != "bakal_source_reward_inventory" || !bytes.Equal(frames[1].Body, []byte{0xAA, 0xBB}) {
		t.Fatalf("reward inventory not spliced between return and clear result: %+v", frames[1])
	}
}

func TestBakalOpeningCampReturnRefusals(t *testing.T) {
	o, err := PrepareBakalOpening(bakalTestRules(), false)
	if err != nil {
		t.Fatal(err)
	}
	t0 := time.Unix(1791210000, 0)
	o.Start("Lansmt", t0)
	o.Tick(t0.Add(3 * time.Second))

	// B' failure runs: the native retreat is legal inside an owned loaded
	// dungeon only ("invalid native Bakal camp return" otherwise).
	if _, err := o.CampReturn(); !errors.Is(err, ErrBakalInvalidCampReturn) {
		t.Fatalf("camp return from camp: %v, want the captured refusal", err)
	}
	if _, err := o.EnterDungeon(100003157, 5, t0.Add(10*time.Second)); err != nil {
		t.Fatal(err)
	}
	if _, err := o.CampReturn(); !errors.Is(err, ErrBakalInvalidCampReturn) {
		t.Fatalf("camp return before loading: %v, want the captured refusal", err)
	}
	if err := o.LoadingDone(100003157); err != nil {
		t.Fatal(err)
	}
	frames, err := o.CampReturn()
	if err != nil {
		t.Fatal(err)
	}
	if len(frames) != 1 {
		t.Fatalf("camp return: %d frames, want 1", len(frames))
	}
	bakalAssertLocationPrefix(t, frames[0], "bakal_retreat_to_camp", BakalCampLocation)
}

func TestBakalOpeningGates(t *testing.T) {
	rules := bakalTestRules()
	o, err := PrepareBakalOpening(rules, false)
	if err != nil {
		t.Fatal(err)
	}
	t0 := time.Unix(1791210000, 0)

	// the start requires its owned waiting-room member.
	if _, err := o.Start("", t0); !errors.Is(err, ErrBakalStartRequiresMember) {
		t.Fatalf("anonymous start: %v, want the captured refusal", err)
	}
	if _, err := o.EnterDungeon(100003149, 24, t0); err == nil {
		t.Fatal("dungeon entry accepted before the start")
	}
	if _, err := o.DefeatMonster(100003149, 24, t0); err == nil {
		t.Fatal("defeat report accepted before the start")
	}
	if _, err := o.CampReturn(); !errors.Is(err, ErrBakalInvalidCampReturn) {
		t.Fatalf("camp return before the start: %v", err)
	}
	if _, err := o.Start("Lansmt", t0); err != nil {
		t.Fatal(err)
	}
	if _, err := o.Start("Lansmt", t0); err == nil {
		t.Fatal("second start accepted")
	}

	o.Tick(t0.Add(3 * time.Second))
	if _, err := o.EnterDungeon(100003148, 1, t0.Add(4*time.Second)); err == nil {
		t.Fatal("dungeon outside the opened 17 accepted")
	}
	if _, err := o.EnterDungeon(100003160, 22, t0.Add(4*time.Second)); err != nil {
		t.Fatal(err)
	}
	// defeat reports outside the loaded dungeon stay refused.
	if _, err := o.DefeatMonster(100003160, 19, t0.Add(5*time.Second)); err == nil {
		t.Fatal("defeat report accepted before the load completed")
	}
	if err := o.LoadingDone(100003160); err != nil {
		t.Fatal(err)
	}
	if _, err := o.DefeatMonster(100003159, 19, t0.Add(6*time.Second)); err == nil {
		t.Fatal("defeat report accepted for another dungeon")
	}

	// the raid buffs: charges spend via UseRaidBuff only while active.
	if _, err := o.UseRaidBuff(9); err == nil {
		t.Fatal("out-of-range buff slot accepted")
	}
	frames, err := o.UseRaidBuff(1)
	if err != nil {
		t.Fatal(err)
	}
	if len(frames) != 1 || frames[0].Name != "bakal_script_buff_inventory" || len(frames[0].Body) != 13 {
		t.Fatalf("buff use published %+v", frames)
	}
	if got := frames[0].Body[:5]; !bytes.Equal(got, []byte{2, 1, 2, 2, 2}) {
		t.Fatalf("buff counts after use % x, want 02 01 02 02 02", got)
	}
	if _, err := o.UseRaidBuff(1); err != nil {
		t.Fatalf("second buff use: %v, want the remaining charge to spend", err) // 1→0
	}
	if _, err := o.UseRaidBuff(1); err == nil {
		t.Fatal("empty buff slot spent")
	}

	// rules binding: nil rules → the waiting-room refusal; hard without a
	// hard phase script → explicit error.
	if _, err := PrepareBakalOpening(nil, false); !errors.Is(err, ErrBakalWaitingRoomNotBound) {
		t.Fatalf("nil rules: %v, want the captured waiting-room refusal", err)
	}
	if _, err := PrepareBakalOpening(rules, true); err == nil {
		t.Fatal("hard opening without a hard phase script accepted")
	}
}

func TestBakalOpeningHardPhaseBindsHardRules(t *testing.T) {
	rules := bakalTestRules()
	rules.HardPhase = &catalog.BakalPhaseRules{
		MonsterMaxHP:       10000,
		RaidBuffs:          []catalog.BakalBuffGrant{{Slot: 4, Count: 2}},
		SettlementTimer:    catalog.BakalTimerRule{ID: 28, Sub: 56, Secs: 180},
		FinalClearDungeon:  100003165,
		AngerWindow:        catalog.BakalTimerRule{ID: 27, Sub: 24, Secs: 420},
		AngerFailThreshold: 7000,
	}
	o, err := PrepareBakalOpening(rules, true)
	if err != nil {
		t.Fatal(err)
	}
	t0 := time.Unix(1791210000, 0)
	o.Start("Lansmt", t0)
	frames := o.Tick(t0.Add(3 * time.Second))
	if len(frames) != 6 {
		t.Fatalf("hard open burst: %d frames, want 6", len(frames))
	}
	// hard mode opens a single slot-4 charge: 00 00 00 00 02 …
	want, _ := hex.DecodeString("000000000219000000ffffffff")
	bakalAssertFrame(t, frames[3], "bakal_buffs", want)
	identity, ok := o.SettlementIdentity(t0)
	if ok {
		t.Fatalf("settlement identity before the final clear: %+v", identity)
	}
}

func TestBakalScriptBuildersVsRules(t *testing.T) {
	rules := bakalTestRules()

	// the opening roster must carry exactly the [CREATE MONSTER] locations.
	roster := BakalOpeningRoster(rules.NormalPhase)
	wantLocs := map[uint32]bool{}
	for _, spawn := range rules.NormalPhase.InitMonsters {
		wantLocs[uint32(spawn.Location)] = true
	}
	if len(roster) != len(wantLocs) {
		t.Fatalf("roster rows %d, want the %d create-monster locations", len(roster), len(wantLocs))
	}
	for _, row := range roster {
		if !wantLocs[row.Location] {
			t.Fatalf("roster row %+v outside the create-monster locations", row)
		}
		if row.Count != 0 || row.MaxHP != 10000 {
			t.Fatalf("roster row %+v, want one instance at [MONSTER MAX HP] 10000", row)
		}
	}

	// the boss health rows carry the four boss-dungeon slots with row state 2.
	health := BakalBossHealthRows(rules.NormalPhase)
	if len(health) != 4 {
		t.Fatalf("boss health rows %d, want 4", len(health))
	}
	for _, row := range health {
		if row.Count != 2 {
			t.Fatalf("health row %+v, want row state 2", row)
		}
	}

	// the open buff counts mirror [ADD BAKAL RAID BUFF].
	counts := BakalOpenBuffCounts(rules.NormalPhase)
	if counts != [5]byte{2, 2, 2, 2, 2} {
		t.Fatalf("normal buff counts % x, want 2/2/2/2/2", counts)
	}

	// location resolution goes through the catalog table only.
	if loc, err := BakalLocationOfDungeon(rules, 100003151); err != nil || loc != 12 {
		t.Fatalf("sparazzi location %d err %v, want 12", loc, err)
	}
	if dgn := BakalDungeonOfLocation(rules, 40); dgn != 100003152 {
		t.Fatalf("hisma location resolves dungeon %d, want 100003152", dgn)
	}
	if _, err := BakalLocationOfDungeon(rules, 100003166); err == nil {
		t.Fatal("dungeon outside the raid resolved a location")
	}
}
