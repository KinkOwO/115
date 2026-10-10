package legion

import (
	"testing"
	"time"
)

func TestBakalNativeThreeUnattendedLanesApplySourceEscapePenalties(t *testing.T) {
	o, ready := runtimeNative(t, false)
	// INIT creates all three attack lanes at +5s; each source route has
	// four 60s steps. Inspect the native scheduled deadlines, not a UI rate.
	o.Tick(ready.Add(244 * time.Second))
	if err := o.EventError(); err != nil {
		t.Fatal(err)
	}
	want := 0
	for _, penalty := range o.script.EscapeAnger {
		if _, exists := o.placements[uint32(penalty.Location)]; !exists {
			t.Fatalf("source escape lane %d has no actor", penalty.Location)
		}
		want += penalty.Amount
	}
	before, rate := o.Anger(), o.angerRate
	o.Tick(ready.Add(245 * time.Second))
	if err := o.EventError(); err != nil {
		t.Fatal(err)
	}
	if got := o.Anger() - before - rate; got != want {
		t.Fatalf("escape delta=%d source sum=%d", got, want)
	}
	for _, penalty := range o.script.EscapeAnger {
		if _, exists := o.placements[uint32(penalty.Location)]; exists {
			t.Fatal("escaped actor remained on route")
		}
	}
	t.Logf("escape sum=%d source fail threshold=%d", want, o.script.AngerFailThreshold)
}

func TestBakalNativeFinalSceneHasSourceSettlementDeadline(t *testing.T) {
	o, ready := runtimeNative(t, false)
	for _, d := range o.rules.Dungeons {
		if d.Type == "" {
			continue
		}
		location, err := BakalLocationOfDungeon(o.rules, d.Index)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := o.EnterDungeon(d.Index, location, ready); err != nil {
			t.Fatal(err)
		}
		if err := o.LoadingDone(d.Index); err != nil {
			t.Fatal(err)
		}
		// Clear the three dragons before the main source clear signal.
		if d.Type != "bakal" {
			if _, err := o.DefeatMonster(d.Index, location, ready); err != nil {
				t.Fatal(err)
			}
		}
	}
	id := uint32(o.script.SettlementDungeon)
	loc, err := BakalLocationOfDungeon(o.rules, id)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := o.EnterDungeon(id, loc, ready); err != nil {
		t.Fatal(err)
	}
	if err := o.LoadingDone(id); err != nil {
		t.Fatal(err)
	}
	if _, err := o.DefeatMonster(id, loc, ready); err != nil {
		t.Fatal(err)
	}
	due := ready.Add(time.Duration(o.script.SettlementTimer.Secs) * time.Second)
	if o.Stage() != BakalOpeningFinal || o.SettleDue(due.Add(-time.Nanosecond)) || !o.SettleDue(due) {
		t.Fatal("source final deadline missing or early")
	}
	o.SetRewardInventory([]byte{})
	frames := o.Tick(due)
	if o.Stage() != BakalOpeningEnded || len(frames) != 4 {
		t.Fatal("source deadline did not end final phase after reward preparation")
	}
	t.Logf("native final fallback=%ds", o.script.SettlementTimer.Secs)
}
