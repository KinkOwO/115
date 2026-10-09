package legion

import (
	"dfolan/internal/catalog"
	"dfolan/internal/catalog/pvf"
	"testing"
	"time"
)

func TestBakalRecoverySourceLadderAndExactEntryDeadline(t *testing.T) {
	o, at := runtimeNative(t, false)
	dgn, loc := uint32(100003157), uint32(26)
	for i, want := range o.rules.RevivalTimes {
		if _, e := o.EnterDungeon(dgn, loc, at); e != nil {
			t.Fatal(e)
		}
		o.LoadingDone(dgn)
		cause := BakalReturnVoluntary
		if i%2 == 0 {
			cause = BakalReturnDeath
		}
		if _, e := o.CampReturnAt(at.Add(300*time.Millisecond), cause); e != nil {
			t.Fatal(e)
		}
		until := time.Unix(at.Unix()+int64(want), 0)
		if !o.RecoveryUntil().Equal(until) {
			t.Fatal("recovery differs from source Unix deadline")
		}
		if _, e := o.EnterDungeon(dgn, loc, until.Add(-time.Nanosecond)); e == nil {
			t.Fatal("entry before deadline accepted")
		}
		at = until
	}
	if _, e := o.EnterDungeon(dgn, loc, at); e != nil {
		t.Fatal("entry at deadline refused", e)
	}
	o.LoadingDone(dgn)
	if _, e := o.CampReturnAt(at, BakalReturnDeath); e != nil {
		t.Fatal(e)
	}
	if o.RecoveryUntil().Sub(at) != time.Duration(o.rules.RevivalTimes[len(o.rules.RevivalTimes)-1])*time.Second {
		t.Fatal("ladder did not cap at source last row")
	}
}

func TestBakalSourceNoPenaltyKickDoesNotAdvanceRecovery(t *testing.T) {
	o, at := runtimeNative(t, false)
	dgn, loc := uint32(100003157), uint32(26)
	if _, e := o.EnterDungeon(dgn, loc, at); e != nil {
		t.Fatal(e)
	}
	o.LoadingDone(dgn)
	q := []bakalSignal{}
	if e := o.executeEvent(catalog.BakalScriptInstruction{Op: "[KICK OUT DUNGEON NO PENALTY]", Args: []pvf.Token{{Type: 0, Value: int32(dgn)}}}, at, &q); e != nil {
		t.Fatal(e)
	}
	if !o.RecoveryUntil().IsZero() || o.penalizedReturns != 0 || !o.NeedsCampReturn() {
		t.Fatal("no-penalty kick added recovery")
	}
	o.AcknowledgeCampReturn()
	if _, e := o.EnterDungeon(dgn, loc, at); e != nil {
		t.Fatal(e)
	}
	o.LoadingDone(dgn)
	if _, e := o.CampReturnAt(at, BakalReturnVoluntary); e != nil {
		t.Fatal(e)
	}
	if o.RecoveryUntil().Sub(at) != time.Duration(o.rules.RevivalTimes[0])*time.Second {
		t.Fatal("no-penalty kick consumed first ladder row")
	}
}

func TestBakalChangedSourceRecoveryAndFailedEventRollback(t *testing.T) {
	o, at := runtimeNative(t, false)
	o.rules.RevivalTimes = []int{7, 11}
	dgn := uint32(100003157)
	if _, e := o.EnterDungeon(dgn, 26, at); e != nil {
		t.Fatal(e)
	}
	o.LoadingDone(dgn)
	old := o.script.Events
	o.script.Events = append(o.script.Events, catalog.BakalScriptEvent{Trigger: []catalog.BakalScriptInstruction{{Op: "[ON GIVEUP DUNGEON]", Args: []pvf.Token{{Type: 0, Value: int32(dgn)}}}}, Behavior: []catalog.BakalScriptInstruction{{Op: "invalid"}}})
	if _, e := o.CampReturnAt(at, BakalReturnDeath); e == nil {
		t.Fatal("bad script accepted")
	}
	if o.penalizedReturns != 0 || !o.RecoveryUntil().IsZero() || o.current != dgn {
		t.Fatal("failed camp transaction partially applied")
	}
	o.script.Events = old
	if _, e := o.CampReturnAt(at, BakalReturnDeath); e != nil {
		t.Fatal(e)
	}
	if o.RecoveryUntil().Sub(at) != 7*time.Second {
		t.Fatal("changed source ignored")
	}
}
