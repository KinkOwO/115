package raid_test

import (
	"dfolan/internal/catalog/pvf"
	"dfolan/internal/raid"
	"reflect"
	"testing"
)

func TestDelezieOpeningCoverageChecksWholeEventsWithoutExecuting(t *testing.T) {
	rules, mode, rows := executableScriptFixture(t)
	before := append([]byte(nil), []byte(mode.Phase.Table.Path)...)
	coverage, err := raid.InspectDelezieEventCoverage(rules, mode)
	if err != nil || coverage.TotalEvents != 5 || coverage.SupportedEvents != 3 || len(coverage.Unresolved) != 2 || coverage.Unresolved[0].TriggerRow != rows[2] || coverage.Unresolved[1].TriggerRow != rows[4] {
		t.Fatal("mixed events appear implemented or coverage depends on uninitialized predicates", coverage, err)
	}
	if coverage.SourcePath != string(before) {
		t.Fatal("source path lost")
	}
	// Coverage means supported instruction contracts, not runtime success:
	// the third supported event still has a division-by-zero action.
	script, err := raid.NewDelezieScript(rules, mode, map[string]int32{"[X]": 1, "[Y]": 1})
	if err != nil {
		t.Fatal(err)
	}
	values := script.Values()
	if matched, _, err := script.ExecuteEvent(rows[3]); err == nil || matched || !reflect.DeepEqual(values, script.Values()) {
		t.Fatal("coverage was confused with runtime initialization", matched, err)
	}
	behavior := mode.Phases[0].Events[0].BehaviorRow
	mode.Phase.Table.Records[behavior+1].Name = "[UNKNOWN ACTION]"
	mode.Phase.Table.Records[behavior].Refs[0].Name = "[UNKNOWN ACTION]"
	changed, err := raid.InspectDelezieEventCoverage(rules, mode)
	if err != nil || changed.SupportedEvents != 2 || changed.Unresolved[0].TriggerRow != rows[0] {
		t.Fatal("changed source action ignored", changed, err)
	}
	foreign := *mode
	if _, err := raid.InspectDelezieEventCoverage(rules, &foreign); err == nil {
		t.Fatal("foreign source accepted")
	}
	mode.Phases = nil
	if coverage, err := raid.InspectDelezieEventCoverage(rules, mode); err != nil || coverage.TotalEvents != 5 || coverage.SupportedEvents != 2 {
		t.Fatal("cached phase absence concealed source events", coverage, err)
	}
	mode.Phase.Table.Records = nil
	if _, err := raid.InspectDelezieEventCoverage(rules, mode); err == nil {
		t.Fatal("empty source counted as complete")
	}
}

func TestDelezieCoverageCannotHideSourceEventsWithCachedMetadata(t *testing.T) {
	rules, mode, rows := executableScriptFixture(t)
	// A stale or incomplete display index must not remove events from the
	// authoritative source checked before battle initialization.
	mode.Phases[0].Events = mode.Phases[0].Events[:1]
	coverage, err := raid.InspectDelezieEventCoverage(rules, mode)
	if err != nil || coverage.TotalEvents != 5 || coverage.SupportedEvents != 3 || len(coverage.Unresolved) != 2 {
		t.Fatal("cached metadata concealed source event gaps", coverage, err)
	}
	mode.Phases = nil
	script, err := raid.NewDelezieScript(rules, mode, map[string]int32{"[X]": 0, "[Y]": 10})
	if err != nil {
		t.Fatal(err)
	}
	if matched, changes, err := script.ExecuteEvent(rows[0]); err != nil || !matched || len(changes.Scalars) != 1 {
		t.Fatal("cached metadata replaced source execution", matched, changes, err)
	}
}

func TestDelezieEventRebindRejectsInvalidParentRefGraph(t *testing.T) {
	rules, mode, _ := executableScriptFixture(t)
	mode.Phase.Table.Records[mode.Phases[0].Events[0].BehaviorRow].Parent = -1
	if _, err := raid.NewDelezieScript(rules, mode, nil); err == nil {
		t.Fatal("stale tree index accepted an invalid source parent/ref graph")
	}
}

func TestDelezieCoverageReindexesAddedSourceEventAndKeepsExistingSnapshot(t *testing.T) {
	rules, mode, rows := executableScriptFixture(t)
	before, err := raid.NewDelezieScript(rules, mode, map[string]int32{"[X]": 0, "[Y]": 10})
	if err != nil {
		t.Fatal(err)
	}
	table := mode.Phase.Table
	first := len(table.Records)
	table.Records = append(table.Records,
		pvf.CTPRecord{Index: first, Parent: 0, Name: "[TRIGGER]", Refs: []pvf.CTPRef{{Name: "[IF]", Values: []uint64{uint64(first + 1)}}}},
		pvf.CTPRecord{Index: first + 1, Parent: first, Name: "[IF]", Cells: []pvf.CTPCell{scalarName("[X]"), scalarName("[==]"), scalarNumber(0)}},
		pvf.CTPRecord{Index: first + 2, Parent: 0, Name: "[BEHAVIOR]", Refs: []pvf.CTPRef{{Name: "[UNKNOWN ACTION]", Values: []uint64{uint64(first + 3)}}}},
		pvf.CTPRecord{Index: first + 3, Parent: first + 2, Name: "[UNKNOWN ACTION]"},
	)
	table.Records[0].Refs = append(table.Records[0].Refs,
		pvf.CTPRef{Name: "[TRIGGER]", Values: []uint64{uint64(first)}},
		pvf.CTPRef{Name: "[BEHAVIOR]", Values: []uint64{uint64(first + 2)}},
	)
	// Neither the old tree's child index nor the import-time phase index knows
	// the added event. A new executor must still see the current source gap.
	coverage, err := raid.InspectDelezieEventCoverage(rules, mode)
	if err != nil || coverage.TotalEvents != 6 || coverage.SupportedEvents != 3 || len(coverage.Unresolved) != 3 || coverage.Unresolved[2].TriggerRow != first {
		t.Fatal("added source event concealed by a stale index", coverage, err)
	}
	if matched, changes, err := before.ExecuteEvent(rows[0]); err != nil || !matched || len(changes.Scalars) != 1 {
		t.Fatal("source replacement corrupted an existing executor snapshot", matched, changes, err)
	}
	if err := before.ValidateEvent(first); err == nil {
		t.Fatal("new source event leaked into an existing executor snapshot")
	}
}
