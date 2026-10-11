package raid_test

import (
	"dfolan/internal/catalog"
	"dfolan/internal/catalog/pvf"
	"dfolan/internal/gamedata"
	"dfolan/internal/raid"
	"fmt"
	"math"
	"os"
	"reflect"
	"strings"
	"testing"
)

func scalarName(s string) pvf.CTPCell  { return pvf.CTPCell{Tag: 1, Kind: "name", Name: s} }
func scalarNumber(n int32) pvf.CTPCell { return pvf.CTPCell{Tag: 4, Kind: "float", Float: float64(n)} }
func scalarRow(kind, target, op string, rhs pvf.CTPCell) pvf.CTPRecord {
	return pvf.CTPRecord{Parent: -1, Name: kind, Cells: []pvf.CTPCell{scalarName(target), scalarName(op), rhs}}
}

func scalarRules(t *testing.T, rows []pvf.CTPRecord) (*catalog.DelezieRaidRules, *catalog.DelezieModeRules) {
	t.Helper()
	for i := range rows {
		rows[i].Index = i
	}
	tree, err := pvf.NewCTPTree(&pvf.CTPTable{Path: "fixture.single.ctp", Records: rows})
	if err != nil {
		t.Fatal(err)
	}
	mode := &catalog.DelezieModeRules{Difficulty: catalog.DelezieSingle, Phase: tree}
	rules := &catalog.DelezieRaidRules{SymbolIDs: map[string]uint32{"[X]": 0, "[Y]": 4}, Modes: map[catalog.DelezieDifficulty]*catalog.DelezieModeRules{catalog.DelezieSingle: mode}}
	return rules, mode
}

func scalarState(t *testing.T, rows []pvf.CTPRecord, initial map[string]int32) *raid.DelezieScalars {
	t.Helper()
	rules, mode := scalarRules(t, rows)
	state, err := raid.NewDelezieScalars(rules, mode, initial)
	if err != nil {
		t.Fatal(err)
	}
	return state
}

func TestDelezieScalarsSourceIsolationAndSnapshots(t *testing.T) {
	rules, mode := scalarRules(t, []pvf.CTPRecord{scalarRow("[SET]", "[X]", "[=]", scalarNumber(7))})
	initial := map[string]int32{"[Y]": 3}
	state, err := raid.NewDelezieScalars(rules, mode, initial)
	if err != nil {
		t.Fatal(err)
	}
	initial["[Y]"] = 99
	state.Values()["[Y]"] = 99
	mode.Phase.Table.Records[0].Cells[2] = scalarNumber(17)
	rules.SymbolIDs["[X]"] = 9
	changes, err := state.ApplyAssignments([]int{0})
	if err != nil || len(changes) != 1 || changes[0].SymbolID != 0 || changes[0].After != 7 || changes[0].PreviouslySet || state.Values()["[Y]"] != 3 {
		t.Fatalf("source or input leaked into snapshot: %v %+v %v", err, changes, state.Values())
	}
	changed, err := raid.NewDelezieScalars(rules, mode, nil)
	if err != nil {
		t.Fatal(err)
	}
	changes, err = changed.ApplyAssignments([]int{0})
	if err != nil || changes[0].After != 17 || changes[0].SymbolID != 9 {
		t.Fatalf("new source values/identity ignored: %+v %v", changes, err)
	}
	foreign := *mode
	if _, err := raid.NewDelezieScalars(rules, &foreign, nil); err == nil {
		t.Fatal("mode from another source accepted")
	}
	if _, err := raid.NewDelezieScalars(nil, mode, nil); err == nil {
		t.Fatal("missing rules accepted")
	}
	if _, err := raid.NewDelezieScalars(rules, mode, map[string]int32{"[UNKNOWN]": 0}); err == nil {
		t.Fatal("unknown initial symbol accepted")
	}
	mode.Phase.Table.Records[0].Cells[0] = scalarName("[UNKNOWN]")
	if _, err := raid.NewDelezieScalars(rules, mode, nil); err == nil {
		t.Fatal("unbound source operand accepted")
	}
}

func TestDelezieScalarsSignedComparisonAndAllOperators(t *testing.T) {
	for _, op := range []string{"[==]", "[!=]", "[>]", "[<]", "[>=]", "[<=]"} {
		for _, pair := range [][2]int32{{-3, -3}, {-4, 1}, {math.MaxInt32, math.MinInt32}} {
			t.Run(fmt.Sprintf("%s/%d/%d", op, pair[0], pair[1]), func(t *testing.T) {
				state := scalarState(t, []pvf.CTPRecord{scalarRow("[IF]", "[X]", op, scalarName("[Y]")), scalarRow("[IF]", "[X]", op, scalarNumber(pair[1]))}, map[string]int32{"[X]": pair[0], "[Y]": pair[1]})
				want := map[string]bool{"[==]": pair[0] == pair[1], "[!=]": pair[0] != pair[1], "[>]": pair[0] > pair[1], "[<]": pair[0] < pair[1], "[>=]": pair[0] >= pair[1], "[<=]": pair[0] <= pair[1]}[op]
				for _, row := range []int{0, 1} {
					got, err := state.Check(row)
					if err != nil || got != want {
						t.Fatalf("signed comparison: %v %v want %v", got, err, want)
					}
				}
			})
		}
	}
	for _, tc := range []struct {
		op                string
		left, right, want int32
	}{
		{"[=]", 4, -7, -7}, {"[+]", -7, 3, -4}, {"[-]", -7, 3, -10}, {"[*]", -7, 3, -21},
		{"[/]", -7, 3, -2}, {"[MOD]", -7, 3, -1}, {"[/]", 7, -3, -2}, {"[MOD]", 7, -3, 1},
	} {
		state := scalarState(t, []pvf.CTPRecord{scalarRow("[SET]", "[X]", tc.op, scalarName("[Y]"))}, map[string]int32{"[X]": tc.left, "[Y]": tc.right})
		got, err := state.ApplyAssignments([]int{0})
		if err != nil || len(got) != 1 || got[0].Before != tc.left || got[0].After != tc.want || !got[0].PreviouslySet {
			t.Fatalf("%s: %+v %v", tc.op, got, err)
		}
	}
}

func TestDelezieScalarsStagedReferencesAndRollback(t *testing.T) {
	state := scalarState(t, []pvf.CTPRecord{
		scalarRow("[SET]", "[X]", "[=]", scalarNumber(5)),
		scalarRow("[SET]", "[Y]", "[=]", scalarName("[X]")),
		scalarRow("[SET]", "[X]", "[+]", scalarName("[Y]")),
		scalarRow("[SET]", "[Y]", "[/]", scalarNumber(0)),
	}, nil)
	changes, err := state.ApplyAssignments([]int{0, 1, 2, 3})
	if err == nil || !strings.Contains(err.Error(), "fixture.single.ctp row 3") || changes != nil || len(state.Values()) != 0 {
		t.Fatalf("failed transaction leaked: %+v %v %v", changes, err, state.Values())
	}
	changes, err = state.ApplyAssignments([]int{0, 1, 2})
	if err != nil || len(changes) != 3 || !reflect.DeepEqual(state.Values(), map[string]int32{"[X]": 10, "[Y]": 5}) {
		t.Fatalf("staged references/order: %+v %v %v", changes, err, state.Values())
	}
	before := state.Values()
	if changes, err = state.ApplyAssignments([]int{0, 999}); err == nil || changes != nil || !reflect.DeepEqual(before, state.Values()) {
		t.Fatal("unknown row did not rollback")
	}
}

func TestDelezieScalarsNeverTreatMissingValuesAsZero(t *testing.T) {
	state := scalarState(t, []pvf.CTPRecord{
		scalarRow("[IF]", "[X]", "[==]", scalarNumber(0)), scalarRow("[SET]", "[X]", "[+]", scalarNumber(1)),
		scalarRow("[SET]", "[X]", "[=]", scalarName("[Y]")), scalarRow("[SET]", "[X]", "[=]", scalarNumber(0)),
		scalarRow("[IF]", "[X]", "[==]", scalarName("[Y]")),
	}, nil)
	if _, err := state.Check(0); err == nil {
		t.Fatal("unset comparison became zero")
	}
	for _, row := range []int{1, 2} {
		if _, err := state.ApplyAssignments([]int{row}); err == nil {
			t.Fatalf("row %d consumed an unset variable", row)
		}
	}
	if _, err := state.ApplyAssignments([]int{3}); err != nil {
		t.Fatal(err)
	}
	if matched, err := state.Check(0); err != nil || !matched {
		t.Fatal("explicit zero lost presence", matched, err)
	}
	if _, err := state.Check(4); err == nil {
		t.Fatal("unset RHS became zero")
	}
	if _, err := state.ApplyAssignments([]int{0}); err == nil {
		t.Fatal("IF applied as SET")
	}
	if _, err := state.Check(3); err == nil {
		t.Fatal("SET checked as IF")
	}
}

func TestDelezieScalarsArithmeticFailuresRollback(t *testing.T) {
	for _, tc := range []struct {
		op          string
		left, right int32
	}{
		{"[+]", math.MaxInt32, 1}, {"[-]", math.MinInt32, 1}, {"[*]", math.MaxInt32, 2},
		{"[/]", 10, 0}, {"[MOD]", 10, 0}, {"[/]", math.MinInt32, -1}, {"[MOD]", math.MinInt32, -1},
	} {
		state := scalarState(t, []pvf.CTPRecord{scalarRow("[SET]", "[Y]", "[=]", scalarNumber(123)), scalarRow("[SET]", "[X]", tc.op, scalarNumber(tc.right))}, map[string]int32{"[X]": tc.left})
		before := state.Values()
		if changes, err := state.ApplyAssignments([]int{0, 1}); err == nil || changes != nil || !reflect.DeepEqual(state.Values(), before) {
			t.Fatalf("%s invalid math leaked: %+v %v", tc.op, changes, err)
		}
	}
}

func scalarEventRules(t *testing.T, unsupported bool) (*catalog.DelezieRaidRules, *catalog.DelezieModeRules) {
	t.Helper()
	rows := []pvf.CTPRecord{{Parent: -1, Name: "[TRIGGER]", Refs: []pvf.CTPRef{{Name: "[IF]", Values: []uint64{1}}}}, scalarRow("[IF]", "[X]", "[==]", scalarNumber(0)), {Parent: -1, Name: "[BEHAVIOR]", Refs: []pvf.CTPRef{{Name: "[SET]", Values: []uint64{3}}}}, scalarRow("[SET]", "[X]", "[=]", scalarNumber(1))}
	rows[1].Parent, rows[3].Parent = 0, 2
	if unsupported {
		rows[2].Refs = append(rows[2].Refs, pvf.CTPRef{Name: "[SUMMON OBJECTS]", Values: []uint64{4}})
		rows = append(rows, pvf.CTPRecord{Parent: 2, Name: "[SUMMON OBJECTS]"})
	}
	// Executable events belong to an actual source phase, rather than an
	// independently supplied cached event list.
	for i := range rows {
		rows[i].Parent++
		for j := range rows[i].Refs {
			for k := range rows[i].Refs[j].Values {
				rows[i].Refs[j].Values[k]++
			}
		}
	}
	rows = append([]pvf.CTPRecord{{Parent: -1, Name: "[PHASE]", Refs: []pvf.CTPRef{{Name: "[TRIGGER]", Values: []uint64{1}}, {Name: "[BEHAVIOR]", Values: []uint64{3}}}}}, rows...)
	rules, mode := scalarRules(t, rows)
	mode.Phases = []catalog.DeleziePhaseRules{{Events: []catalog.DelezieScriptEvent{{TriggerRow: 1, BehaviorRow: 3}}}}
	return rules, mode
}

func TestDelezieScalarEventRunsOnceAndRejectsMixedActionsBeforeMutation(t *testing.T) {
	for _, mixed := range []bool{false, true} {
		rules, mode := scalarEventRules(t, mixed)
		state, err := raid.NewDelezieScalars(rules, mode, map[string]int32{"[X]": 0})
		if err != nil {
			t.Fatal(err)
		}
		matched, changes, err := state.ExecuteScalarEvent(1)
		if mixed {
			if err == nil || !strings.Contains(err.Error(), "row 5") || matched || changes != nil || state.Values()["[X]"] != 0 {
				t.Fatalf("mixed event partly ran: %v %+v %v", matched, changes, err)
			}
		} else {
			if err != nil || !matched || len(changes) != 1 || state.Values()["[X]"] != 1 {
				t.Fatalf("scalar event failed: %v %+v %v", matched, changes, err)
			}
			if matched, changes, err = state.ExecuteScalarEvent(1); err != nil || matched || len(changes) != 0 {
				t.Fatalf("false predicate changed state: %v %+v %v", matched, changes, err)
			}
		}
	}
}

func TestDelezieScalarEventRejectsNestedAndGroupedPredicates(t *testing.T) {
	for _, nested := range []bool{false, true} {
		rules, mode := scalarEventRules(t, false)
		rows := mode.Phase.Table.Records
		rows = append(rows, scalarRow("[IF]", "[Y]", "[==]", scalarNumber(0)))
		rows[5].Index = 5
		parent := 1
		if nested {
			parent = 2
		}
		rows[5].Parent = parent
		rows[parent].Refs = append(rows[parent].Refs, pvf.CTPRef{Name: "[IF]", Values: []uint64{5}})
		var err error
		mode.Phase, err = pvf.NewCTPTree(&pvf.CTPTable{Path: "fixture.single.ctp", Records: rows})
		if err != nil {
			t.Fatal(err)
		}
		state, err := raid.NewDelezieScalars(rules, mode, map[string]int32{"[X]": 0, "[Y]": 0})
		if err != nil {
			t.Fatal(err)
		}
		before := state.Values()
		if matched, changes, err := state.ExecuteScalarEvent(1); err == nil || matched || changes != nil || !reflect.DeepEqual(before, state.Values()) {
			t.Fatalf("unsupported predicate executed: %v %+v %v", matched, changes, err)
		}
	}
}

func TestDelezieScalarEventArithmeticFailureRollsBack(t *testing.T) {
	rules, mode := scalarEventRules(t, false)
	rows := mode.Phase.Table.Records
	rows[3].Refs[0].Values = append(rows[3].Refs[0].Values, 5)
	rows = append(rows, scalarRow("[SET]", "[Y]", "[/]", scalarNumber(0)))
	rows[5].Index, rows[5].Parent = 5, 3
	var err error
	mode.Phase, err = pvf.NewCTPTree(&pvf.CTPTable{Path: "fixture.single.ctp", Records: rows})
	if err != nil {
		t.Fatal(err)
	}
	state, err := raid.NewDelezieScalars(rules, mode, map[string]int32{"[X]": 0, "[Y]": 8})
	if err != nil {
		t.Fatal(err)
	}
	before := state.Values()
	if applied, changes, err := state.ExecuteScalarEvent(1); err == nil || applied || changes != nil || !reflect.DeepEqual(before, state.Values()) {
		t.Fatalf("event left partial writes: %v %+v %v", applied, changes, err)
	}
}

func TestDelezieScalarsNativePVFProjection(t *testing.T) {
	path := os.Getenv("DFO_PVF_CORE_TEST_ARCHIVE")
	if path == "" {
		t.Skip("read-only current PVF required")
	}
	source, err := gamedata.Open(gamedata.Options{Mode: gamedata.PVF, ArchivePath: path, ExpectedChecksum: os.Getenv("DFO_PVF_CORE_TEST_SHA256"), DerivedCacheDir: "-"})
	if err != nil {
		t.Fatal(err)
	}
	defer source.Close()
	directory, err := source.ChannelDirectory()
	if err != nil {
		t.Fatal(err)
	}
	entries, err := source.RaidEntrances(directory)
	if err != nil {
		t.Fatal(err)
	}
	rules := entries[112].Delezie
	if rules == nil {
		t.Fatal("current Source has no Delezie rules")
	}
	for difficulty := catalog.DelezieNormal; difficulty <= catalog.DelezieMatch; difficulty++ {
		t.Run(difficulty.Name(), func(t *testing.T) {
			mode := rules.Modes[difficulty]
			// Zero is an explicit test fixture, not a runtime initialization rule.
			initial := map[string]int32{}
			for symbol := range rules.SymbolIDs {
				initial[symbol] = 0
			}
			state, err := raid.NewDelezieScalars(rules, mode, initial)
			if err != nil {
				t.Fatal(err)
			}
			event := mode.Phases[0].Events[0]
			for _, row := range event.Trigger {
				if row.Name != "[IF]" {
					t.Fatalf("initial scalar projection has non-IF trigger %s", row.Name)
				}
				if matched, err := state.Check(row.Index); err != nil || !matched {
					t.Fatalf("source initial condition: %v %v", matched, err)
				}
			}
			// The complete initialization includes unimplemented actions, so the
			// event API must reject it without any variable writes.
			if matched, changes, err := state.ExecuteScalarEvent(event.TriggerRow); err == nil || matched || changes != nil || !reflect.DeepEqual(state.Values(), initial) {
				t.Fatalf("initialization falsely completed: %v %+v %v", matched, changes, err)
			}
			var assignments []int
			want := state.Values()
			for _, row := range event.Behavior {
				if row.Name != "[SET]" {
					continue
				}
				x := mode.ScalarExpressions[row.Index]
				if x.Operator != "[=]" || x.Operand.IsSymbol {
					t.Fatalf("source initialization projection changed: %+v", x)
				}
				assignments = append(assignments, row.Index)
				want[x.Target] = x.Operand.Value
			}
			changes, err := state.ApplyAssignments(assignments)
			if err != nil || len(assignments) == 0 || len(changes) != len(assignments) || !reflect.DeepEqual(state.Values(), want) {
				t.Fatalf("source scalar initialization projection: %+v %v", changes, err)
			}
			t.Logf("source=%s expressions=%d initial-scalar-assignments=%d complete-initialization=false", mode.Phase.Table.Path, len(mode.ScalarExpressions), len(changes))
		})
	}
}
