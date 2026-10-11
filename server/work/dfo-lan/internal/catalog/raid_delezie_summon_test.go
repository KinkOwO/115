package catalog

import (
	"dfolan/internal/catalog/pvf"
	"math"
	"reflect"
	"strings"
	"testing"
)

func TestDelezieSummonNativePairReaderAndSourceOwnership(t *testing.T) {
	n := func(v float64) pvf.CTPCell { return pvf.CTPCell{Tag: 4, Kind: "float", Float: v} }
	name := pvf.CTPCell{Tag: 2, Kind: "name", Name: "[X]"}
	tail := pvf.CTPCell{Tag: 3, Kind: "byte", Byte: 17}
	table := &pvf.CTPTable{Path: "source.single.ctp", Records: []pvf.CTPRecord{
		{Index: 0, Parent: -1, Name: "[SUMMON OBJECTS]", Cells: []pvf.CTPCell{n(2.9), n(3), n(-1), name, n(7), tail}},
		{Index: 1, Parent: -1, Name: "[SUMMON OBJECTS BY ONE OBJECT PRESET INDEX]", Cells: []pvf.CTPCell{n(2), n(8), n(9)}},
	}}
	tree, err := pvf.NewCTPTree(table)
	if err != nil {
		t.Fatal(err)
	}
	mode := &DelezieModeRules{Difficulty: DelezieSingle, Phase: tree}
	rules := &DelezieRaidRules{Modes: map[DelezieDifficulty]*DelezieModeRules{DelezieSingle: mode}, SymbolIDs: map[string]uint32{"[X]": 0}}
	program, err := rules.SummonProgram(mode)
	if err != nil || len(program) != 1 {
		t.Fatal(program, err)
	}
	action := program[0]
	if len(action.Pairs) != 2 || action.Pairs[0].First.Value != 3 || action.Pairs[0].Second.Value != -1 || !action.Pairs[1].First.IsSymbol || action.Pairs[1].First.SymbolID != 0 || action.Pairs[1].Second.Value != 7 || !reflect.DeepEqual(action.IgnoredTail, []pvf.CTPCell{tail}) {
		t.Fatal("paired order, symbol binding or unconsumed tail lost", action)
	}
	table.Records[0].Cells[1] = n(4)
	changed, err := rules.SummonProgram(mode)
	if err != nil || changed[0].Pairs[0].First.Value != 4 || program[0].Pairs[0].First.Value != 3 {
		t.Fatal("source change ignored or previous program borrowed source", changed, err)
	}
	action.IgnoredTail[0].Byte = 99
	if table.Records[0].Cells[5].Byte != 17 {
		t.Fatal("tail exposes source storage")
	}
	foreign := *mode
	if _, err := rules.SummonProgram(&foreign); err == nil {
		t.Fatal("foreign mode accepted")
	}
	for _, cells := range [][]pvf.CTPCell{
		nil, {n(2), n(3), n(-1)}, {n(-1)}, {n(math.MaxInt32)},
		{{Tag: 5, Kind: "float", Float: 1}, n(3), n(7)},
		{n(1), {Tag: 1, Kind: "name", Name: "[MISSING]"}, n(7)},
		{n(1), n(math.Inf(1)), n(7)},
	} {
		table.Records[0].Cells = cells
		if _, err := rules.SummonProgram(mode); err == nil || !strings.Contains(err.Error(), "source.single.ctp row 0") {
			t.Fatal("invalid count/operand lacks source error", cells, err)
		}
	}
}
