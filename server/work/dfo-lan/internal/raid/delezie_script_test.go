package raid_test

import (
	"dfolan/internal/catalog"
	"dfolan/internal/catalog/pvf"
	"dfolan/internal/raid"
	"reflect"
	"testing"
)

func executableScriptFixture(t *testing.T) (*catalog.DelezieRaidRules, *catalog.DelezieModeRules, []int) {
	t.Helper()
	rules, mode := rawTimerFixture(t)
	rules.SymbolIDs = map[string]uint32{"[X]": 0, "[Y]": 1}
	rules.Symbols.Table.Records[0].Cells = append(rules.Symbols.Table.Records[0].Cells, scalarNumber(1), scalarName("[Y]"))
	table := &pvf.CTPTable{Path: "executable.single.ctp"}
	add := func(parent int, name string, cells ...pvf.CTPCell) int {
		i := len(table.Records)
		table.Records = append(table.Records, pvf.CTPRecord{Index: i, Parent: parent, Name: name, Cells: cells})
		if parent >= 0 {
			table.Records[parent].Refs = append(table.Records[parent].Refs, pvf.CTPRef{Name: name, Values: []uint64{uint64(i)}})
		}
		return i
	}
	phase := add(-1, "[PHASE]")
	var events []catalog.DelezieScriptEvent
	var rows []int
	for event := 0; event < 5; event++ {
		trigger := add(phase, "[TRIGGER]")
		if event == 1 {
			add(trigger, "[CHECK TIMER END]", scalarName("[A]"), scalarName("[NONE]"))
		} else {
			op, value := "[==]", int32(0)
			if event == 2 || event == 3 {
				op, value = "[>]", 0
			}
			add(trigger, "[IF]", scalarName("[X]"), scalarName(op), scalarNumber(value))
		}
		behavior := add(phase, "[BEHAVIOR]")
		increment := int32(1)
		if event == 1 {
			increment = 2
		}
		add(behavior, "[SET]", scalarName("[X]"), scalarName("[+]"), scalarNumber(increment))
		if event == 2 {
			add(behavior, "[SUMMON OBJECTS]", scalarNumber(1), scalarNumber(3))
		} else if event == 4 {
			add(behavior, "[RESET TIMER]", scalarName("[A]"), scalarName("[NONE]"))
		} else {
			main, value := "[A]", int32(5)
			if event == 3 {
				main, value = "[B]", 3
			}
			add(behavior, "[SET TIMER]", scalarName(main), scalarName("[NONE]"), scalarNumber(value))
			if event == 3 {
				add(behavior, "[SET]", scalarName("[Y]"), scalarName("[/]"), scalarNumber(0))
			}
		}
		events = append(events, catalog.DelezieScriptEvent{TriggerRow: trigger, BehaviorRow: behavior})
		rows = append(rows, trigger)
	}
	tree, err := pvf.NewCTPTree(table)
	if err != nil {
		t.Fatal(err)
	}
	mode.Phase = tree
	mode.Phases = []catalog.DeleziePhaseRules{{Events: events}}
	return rules, mode, rows
}

func TestDelezieCompleteScalarTimerEventsAtomicityAndRearm(t *testing.T) {
	rules, mode, rows := executableScriptFixture(t)
	script, err := raid.NewDelezieScript(rules, mode, map[string]int32{"[X]": 0, "[Y]": 10})
	if err != nil {
		t.Fatal(err)
	}
	if matched, changes, err := script.ExecuteEvent(rows[1]); err == nil || matched || changes != nil {
		t.Fatal("unarmed timer predicate accepted", matched, changes, err)
	}
	if matched, changes, err := script.ExecuteEvent(rows[0]); err != nil || !matched || len(changes.Scalars) != 1 || len(changes.Timers) != 1 || script.Values()["[X]"] != 1 {
		t.Fatal(matched, changes, err)
	}
	if matched, changes, err := script.ExecuteEvent(rows[0]); err != nil || matched || changes != nil {
		t.Fatal("false predicate replayed effects", matched, changes, err)
	}
	if _, err := script.AdvanceRaw(4); err != nil {
		t.Fatal(err)
	}
	if matched, _, err := script.ExecuteEvent(rows[1]); err != nil || matched {
		t.Fatal("early timer event", matched, err)
	}
	if _, err := script.AdvanceRaw(5); err != nil {
		t.Fatal(err)
	}
	if matched, changes, err := script.ExecuteEvent(rows[1]); err != nil || !matched || script.Values()["[X]"] != 3 || changes.Timers[0].Deadline != 10 {
		t.Fatal(matched, changes, err)
	}
	if matched, _, err := script.ExecuteEvent(rows[1]); err != nil || matched || script.Values()["[X]"] != 3 {
		t.Fatal("rearmed expiry replayed", matched, err)
	}
	beforeValues, beforeTimers := script.Values(), script.Timers()
	for _, row := range rows[2:] {
		if matched, changes, err := script.ExecuteEvent(row); err == nil || matched || changes != nil || !reflect.DeepEqual(beforeValues, script.Values()) || !reflect.DeepEqual(beforeTimers, script.Timers()) {
			t.Fatal("unsupported/failing event projected partial effects", row, matched, changes, err)
		}
	}
	if err := script.ValidateEvent(rows[0]); err != nil {
		t.Fatal(err)
	}
	if err := script.ValidateEvent(rows[2]); err == nil {
		t.Fatal("mixed event marked supported")
	}
	if _, _, err := script.ExecuteEvent(999); err == nil {
		t.Fatal("unknown event accepted")
	}
	copy := script.Values()
	copy["[X]"] = 99
	if script.Values()["[X]"] != 3 {
		t.Fatal("script snapshot leaked")
	}
}

func TestDelezieCompleteEventFollowsSourceAndRejectsUnknownActionBeforeFalsePredicate(t *testing.T) {
	rules, mode, rows := executableScriptFixture(t)
	script, err := raid.NewDelezieScript(rules, mode, map[string]int32{"[X]": 5, "[Y]": 10})
	if err != nil {
		t.Fatal(err)
	}
	if matched, _, err := script.ExecuteEvent(rows[4]); err == nil || matched {
		t.Fatal("false predicate hid unsupported reset")
	}
	for i := range mode.Phase.Table.Records {
		r := &mode.Phase.Table.Records[i]
		if r.Name == "[SET TIMER]" {
			r.Cells[2] = scalarNumber(7)
		}
		if r.Name == "[SET]" && r.Cells[0].Name == "[X]" {
			r.Cells[2] = scalarNumber(4)
		}
	}
	changed, err := raid.NewDelezieScript(rules, mode, map[string]int32{"[X]": 0, "[Y]": 10})
	if err != nil {
		t.Fatal(err)
	}
	matched, result, err := changed.ExecuteEvent(rows[0])
	if err != nil || !matched || changed.Values()["[X]"] != 4 || result.Timers[0].Value != 7 {
		t.Fatal("new source ignored", matched, result, err)
	}
	if script.Values()["[X]"] != 5 {
		t.Fatal("source edit changed established script")
	}
}
