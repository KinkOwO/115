package raid_test

import (
	"dfolan/internal/catalog"
	"dfolan/internal/catalog/pvf"
	"dfolan/internal/raid"
	"math"
	"reflect"
	"testing"
)

func rawTimerFixture(t *testing.T) (*catalog.DelezieRaidRules, *catalog.DelezieModeRules) {
	t.Helper()
	makeTree := func(path string, rows []pvf.CTPRecord) *pvf.CTPTree {
		tree, err := pvf.NewCTPTree(&pvf.CTPTable{Path: path, Records: rows})
		if err != nil {
			t.Fatal(err)
		}
		return tree
	}
	name := func(v string) pvf.CTPCell { return pvf.CTPCell{Tag: 1, Kind: "name", Name: v} }
	timerRows := []pvf.CTPRecord{}
	for i, main := range []string{"[A]", "[B]"} {
		start := len(timerRows)
		timerRows = append(timerRows,
			pvf.CTPRecord{Index: start, Parent: -1, Name: "[TIMER]", Refs: []pvf.CTPRef{{Name: "[MAIN TYPE]", Values: []uint64{uint64(start + 1)}}, {Name: "[SUB TYPE]", Values: []uint64{uint64(start + 2)}}}},
			pvf.CTPRecord{Index: start + 1, Parent: start, Name: "[MAIN TYPE]", Cells: []pvf.CTPCell{scalarNumber(int32(i + 1)), name(main)}},
			pvf.CTPRecord{Index: start + 2, Parent: start, Name: "[SUB TYPE]", Cells: []pvf.CTPCell{scalarNumber(0), name("[NONE]")}},
		)
	}
	phaseRows := []pvf.CTPRecord{
		{Index: 0, Parent: -1, Name: "[SET TIMER]", Cells: []pvf.CTPCell{name("[A]"), name("[NONE]"), scalarNumber(5)}},
		{Index: 1, Parent: -1, Name: "[SET TIMER]", Cells: []pvf.CTPCell{name("[B]"), name("[NONE]"), scalarNumber(3)}},
		{Index: 2, Parent: -1, Name: "[CHECK TIMER END]", Cells: []pvf.CTPCell{name("[A]"), name("[NONE]")}},
		{Index: 3, Parent: -1, Name: "[SET TIMER]", Cells: []pvf.CTPCell{name("[A]"), name("[NONE]"), scalarNumber(0)}},
		{Index: 4, Parent: -1, Name: "[SET TIMER]", Cells: []pvf.CTPCell{name("[A]"), name("[NONE]"), scalarNumber(-1)}},
		{Index: 5, Parent: -1, Name: "[RESET TIMER]", Cells: []pvf.CTPCell{name("[A]"), name("[NONE]")}},
		{Index: 6, Parent: -1, Name: "[SET TIMER]", Cells: []pvf.CTPCell{name("[A]"), name("[NONE]"), scalarNumber(9)}},
	}
	mode := &catalog.DelezieModeRules{Difficulty: catalog.DelezieSingle, Phase: makeTree("single.ctp", phaseRows), Objects: makeTree("single.object.ctp", []pvf.CTPRecord{{Index: 0, Parent: -1, Name: "[SET TIMER]", Cells: []pvf.CTPCell{name("[B]"), name("[NONE]"), scalarNumber(99)}}})}
	rules := &catalog.DelezieRaidRules{Modes: map[catalog.DelezieDifficulty]*catalog.DelezieModeRules{mode.Difficulty: mode}, Timers: makeTree("timer.ctp", timerRows), Symbols: makeTree("symbol.ctp", []pvf.CTPRecord{{Index: 0, Parent: -1, Name: "[SYMBOL]", Cells: []pvf.CTPCell{scalarNumber(0), name("[X]")}}})}
	return rules, mode
}

func TestDelezieTimersAtomicRawCountdownAndScopedIdentity(t *testing.T) {
	rules, mode := rawTimerFixture(t)
	state, err := raid.NewDelezieTimers(rules, mode)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := state.CheckPhase(2); err == nil {
		t.Fatal("unarmed state silently initialized")
	}
	armed, err := state.ArmPhase([]int{0, 1})
	if err != nil || armed[0].Value != 5 || armed[1].Value != 3 || armed[0].Key == armed[1].Key {
		t.Fatal(armed, err)
	}
	before := state.Snapshot()
	for _, rows := range [][]int{{6, 3}, {6, 4}, {6, 5}, {6, 2}, {6, 99}} {
		if got, err := state.ArmPhase(rows); err == nil || got != nil || !reflect.DeepEqual(before, state.Snapshot()) {
			t.Fatal("partial/unsupported arm", rows, got, err)
		}
	}
	ended, err := state.AdvanceRaw(3)
	if err != nil || len(ended) != 1 || ended[0].Key.Main != 2 {
		t.Fatal(ended, err)
	}
	if ready, err := state.CheckPhase(2); err != nil || ready {
		t.Fatal("main/sub clocks merged", ready, err)
	}
	ended, err = state.AdvanceRaw(5)
	if err != nil || len(ended) != 1 || ended[0].Key.Main != 1 {
		t.Fatal(ended, err)
	}
	if ready, err := state.CheckPhase(2); err != nil || !ready {
		t.Fatal(ready, err)
	}
	if ended, err := state.AdvanceRaw(5); err != nil || len(ended) != 0 {
		t.Fatal("expiry replay", ended, err)
	}
	before = state.Snapshot()
	if _, err := state.AdvanceRaw(4); err == nil || state.NowRaw() != 5 || !reflect.DeepEqual(before, state.Snapshot()) {
		t.Fatal("backwards time changed state")
	}
	for _, keys := range [][]catalog.DelezieTimerKey{{armed[0].Key, {Main: 99}}, {armed[0].Key, armed[0].Key}} {
		if err := state.CancelResolved(keys); err == nil || !reflect.DeepEqual(before, state.Snapshot()) {
			t.Fatal("partial cancellation", err)
		}
	}
	if err := state.CancelResolved([]catalog.DelezieTimerKey{armed[0].Key}); err != nil {
		t.Fatal(err)
	}
	if _, err := state.CheckPhase(2); err == nil {
		t.Fatal("cancelled timer accepted")
	}
	armed, err = state.ArmPhase([]int{6})
	if err != nil || armed[0].Deadline != 14 || armed[0].Ended {
		t.Fatal("explicit rearm retained stale expiry", armed, err)
	}
	copy := state.Snapshot()
	delete(copy, armed[0].Key)
	if _, ok := state.Snapshot()[armed[0].Key]; !ok {
		t.Fatal("snapshot leaked")
	}
}

func TestDelezieTimersSnapshotSourceChangesOverflowAndForeignModes(t *testing.T) {
	rules, mode := rawTimerFixture(t)
	state, err := raid.NewDelezieTimers(rules, mode)
	if err != nil {
		t.Fatal(err)
	}
	mode.Phase.Table.Records[0].Cells[2] = scalarNumber(7)
	rules.Timers.Table.Records[1].Cells[0] = scalarNumber(8)
	// Cached projections are not the executable source.
	rules.TimerDefinitions = map[string]catalog.DelezieTimerRule{"[A]": {ID: 99}}
	got, err := state.ArmPhase([]int{0})
	if err != nil || got[0].Value != 5 || got[0].Key.Main != 1 {
		t.Fatal("established snapshot changed", got, err)
	}
	changed, err := raid.NewDelezieTimers(rules, mode)
	if err != nil {
		t.Fatal(err)
	}
	got, err = changed.ArmPhase([]int{0})
	if err != nil || got[0].Value != 7 || got[0].Key.Main != 8 {
		t.Fatal("source change ignored", got, err)
	}
	foreign := *mode
	if _, err := raid.NewDelezieTimers(rules, &foreign); err == nil {
		t.Fatal("foreign mode accepted")
	}
	if _, err := changed.AdvanceRaw(math.MaxUint64 - 3); err != nil {
		t.Fatal(err)
	}
	before := changed.Snapshot()
	if _, err := changed.ArmPhase([]int{1, 0}); err == nil || !reflect.DeepEqual(before, changed.Snapshot()) {
		t.Fatal("overflow partially armed")
	}
}
