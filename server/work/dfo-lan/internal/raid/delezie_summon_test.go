package raid_test

import (
	"dfolan/internal/catalog/pvf"
	"dfolan/internal/raid"
	"reflect"
	"testing"
)

func TestDelezieObjectSummonOperandsPreserveUnresolvedValuesAndSnapshots(t *testing.T) {
	rules, mode := objectStateFixture(t)
	rules.SymbolIDs = map[string]uint32{"[X]": 0}
	table := &pvf.CTPTable{Path: "summon.single.ctp", Records: []pvf.CTPRecord{{Index: 0, Parent: -1, Name: "[SUMMON OBJECTS]", Cells: []pvf.CTPCell{scalarNumber(2), scalarNumber(3), scalarNumber(-1), scalarName("[X]"), scalarNumber(7), scalarNumber(0)}}}}
	var err error
	mode.Phase, err = pvf.NewCTPTree(table)
	if err != nil {
		t.Fatal(err)
	}
	state, err := raid.NewDelezieObjects(rules, mode, 0)
	if err != nil {
		t.Fatal(err)
	}
	if operands, err := state.SummonOperands(0, nil); err == nil || operands != nil || len(state.Snapshot()) != 0 {
		t.Fatal("missing symbol defaulted or partially created objects", operands, err)
	}
	got, err := state.SummonOperands(0, map[string]int32{"[X]": 3})
	if err != nil || !reflect.DeepEqual(got, [][2]int32{{3, -1}, {3, 7}}) || len(state.Snapshot()) != 0 {
		t.Fatal("unresolved destination changed or operands became spawned objects", got, err)
	}
	got[0][0] = 99
	table.Records[0].Cells[1] = scalarNumber(4)
	again, err := state.SummonOperands(0, map[string]int32{"[X]": 3})
	if err != nil || again[0][0] != 3 {
		t.Fatal("source/output mutation leaked into owned program", again, err)
	}
	changed, err := raid.NewDelezieObjects(rules, mode, 0)
	if err != nil {
		t.Fatal(err)
	}
	again, err = changed.SummonOperands(0, map[string]int32{"[X]": 4})
	if err != nil || again[0][0] != 4 || again[1][0] != 4 {
		t.Fatal("fresh source or supplied symbol ignored", again, err)
	}
	if _, err := state.SummonOperands(99, nil); err == nil {
		t.Fatal("unknown summon row accepted")
	}
}
