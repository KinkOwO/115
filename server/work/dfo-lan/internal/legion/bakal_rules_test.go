package legion

import (
	"dfolan/internal/catalog"
	"encoding/binary"
	"testing"
	"time"
)

func TestBakalRemainingResynchronizesSourceCountdown(t *testing.T) {
	r := bakalTestRules()
	r.PhaseTimeOverSecs = 1234
	o, err := PrepareBakalOpening(r, false)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Unix(1000, 0)
	o.Start("actor", now)
	ready := now.Add(time.Duration(r.StartDelaySecs) * time.Second)
	o.Tick(ready)
	for _, tc := range []struct {
		elapsed int
		want    uint32
	}{{10, 1224}, {20, 1214}} {
		frames := o.Tick(ready.Add(time.Duration(tc.elapsed) * time.Second))
		found := false
		for _, f := range frames {
			if f.ID == 584 {
				found = true
				if binary.LittleEndian.Uint32(f.Body[1:]) != tc.want {
					t.Fatal("countdown ignores source/elapsed time")
				}
			}
		}
		if !found {
			t.Fatal("N584 never resynchronized")
		}
	}
}

func TestBakalRulesDriveFramesAndFailure(t *testing.T) {
	r := bakalTestRules()
	r.PhaseTimeOverSecs = 4321
	r.Dungeons = append(r.Dungeons, catalog.BakalDungeonInfo{Index: 100004999, InitialState: "open"})
	r.NormalPhase.InitMonsters[0].Location = 55
	o, e := PrepareBakalOpening(r, false)
	if e != nil {
		t.Fatal(e)
	}
	now := time.Unix(1000, 0)
	o.Start("Lansmt", now)
	frames := o.Tick(now.Add(3 * time.Second))
	for _, f := range frames {
		if f.ID == 584 && binary.LittleEndian.Uint32(f.Body[1:]) != 4321 {
			t.Fatal("countdown ignores source")
		}
		if f.ID == 572 && f.Body[1] != 18 {
			t.Fatal("open list ignores source")
		}
		if f.ID == 2286 {
			found := false
			for i := 0; i < int(f.Body[0]); i++ {
				if binary.LittleEndian.Uint32(f.Body[5+i*19:]) == 55 {
					found = true
				}
			}
			if !found {
				t.Fatal("roster ignores source placement")
			}
		}
	}
	if !o.HasDungeon(100004999) {
		t.Fatal("authorization still uses fixed ID range")
	}
	o.Tick(now.Add(100 * time.Second))
	before := o.Anger()
	o.Tick(now.Add(130 * time.Second))
	if o.Anger()-before != 93 {
		t.Fatalf("window crossing rate=%d", o.Anger()-before)
	}
	o.Tick(now.Add(1000 * time.Second))
	if o.Stage() != BakalOpeningFailed {
		t.Fatal("anger overflow did not fail source phase")
	}
}

func TestBakalFinalWindowRequiresDurableInventory(t *testing.T) {
	r := bakalTestRules()
	o, e := PrepareBakalOpening(r, false)
	if e != nil {
		t.Fatal(e)
	}
	now := time.Unix(1000, 0)
	o.Start("Lansmt", now)
	o.Tick(now.Add(3 * time.Second))
	o.EnterDungeon(uint32(r.NormalPhase.SettlementDungeon), 24, now)
	o.LoadingDone(uint32(r.NormalPhase.SettlementDungeon))
	o.DefeatMonster(uint32(r.NormalPhase.SettlementDungeon), 24, now)
	due := now.Add(180 * time.Second)
	if f := o.Tick(due); len(f) != 0 || o.Stage() != BakalOpeningFinal {
		t.Fatal("uncommitted settlement published")
	}
	o.SetRewardInventory([]byte{})
	if len(o.Tick(due)) != 4 || o.Stage() != BakalOpeningEnded {
		t.Fatal("committed settlement not published")
	}
}

func TestBakalSourceCombatTimeoutRetainsCampRetry(t *testing.T) {
	r := bakalTestRules()
	r.NormalPhase.AngerFailThreshold = 100000
	o, e := PrepareBakalOpening(r, false)
	if e != nil {
		t.Fatal(e)
	}
	now := time.Unix(1000, 0)
	o.Start("Lansmt", now)
	o.Tick(now.Add(3 * time.Second))
	entered := now.Add(5 * time.Second)
	o.EnterDungeon(uint32(r.NormalPhase.EnterBakalDungeon), 24, entered)
	o.LoadingDone(uint32(r.NormalPhase.EnterBakalDungeon))
	frames := o.Tick(entered.Add(time.Duration(r.NormalPhase.EnterBakalTimer.Secs) * time.Second))
	if !o.NeedsCampReturn() || len(frames) == 0 || frames[len(frames)-1].ID != 2285 {
		t.Fatal("source combat timer did not request camp return")
	}
	if _, err := o.EnterDungeon(100003160, 22, entered); err == nil {
		t.Fatal("accepted an entry before camp transport was restored")
	}
	o.AcknowledgeCampReturn()
	if _, err := o.EnterDungeon(100003160, 22, entered); err != nil {
		t.Fatal(err)
	}
}
