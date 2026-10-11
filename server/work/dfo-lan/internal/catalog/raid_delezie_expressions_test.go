package catalog

import (
	"dfolan/internal/catalog/pvf"
	"math"
	"strings"
	"testing"
)

func TestDelezieScalarOperandsFollowSourceAndDistinguishSymbolZero(t *testing.T) {
	n := func(v float64) pvf.CTPCell { return pvf.CTPCell{Tag: 4, Kind: "float", Float: v} }
	s := func(v string) pvf.CTPCell { return pvf.CTPCell{Tag: 1, Kind: "name", Name: v} }
	table := &pvf.CTPTable{Path: "scalar.ctp", Records: []pvf.CTPRecord{
		{Index: 0, Parent: -1, Name: "[SET]", Cells: []pvf.CTPCell{s("[GAUGE]"), n(-2.9)}},
		{Index: 1, Parent: -1, Name: "[IF]", Cells: []pvf.CTPCell{s("[GAUGE]"), s("[>=]"), s("[MAX]")}},
		{Index: 2, Parent: -1, Name: "[SET]", Cells: []pvf.CTPCell{s("[GAUGE]"), s("[+]"), s("[TICK]")}},
		{Index: 3, Parent: -1, Name: "[UNRESOLVED ACTION]", Cells: []pvf.CTPCell{n(17)}},
	}}
	tree, err := pvf.NewCTPTree(table)
	if err != nil {
		t.Fatal(err)
	}
	symbols := map[string]uint32{"[GAUGE]": 0, "[MAX]": 2, "[TICK]": 3}
	rules, err := parseDelezieScalarExpressions(tree, symbols)
	if err != nil {
		t.Fatal(err)
	}
	if len(rules) != 3 || rules[0].TargetID != 0 || rules[0].Operand.IsSymbol || rules[0].Operand.Value != -2 || !rules[1].Operand.IsSymbol || rules[1].Operand.SymbolID != 2 {
		t.Fatalf("numeric/symbol operands changed: %+v", rules)
	}
	// IDs, operation, literal and referenced symbol all follow the source.
	symbols["[GAUGE]"] = 11
	table.Records[0].Cells[1] = n(41.8)
	table.Records[2].Cells[1] = s("[-]")
	table.Records[2].Cells[2] = s("[GAUGE]")
	changed, err := parseDelezieScalarExpressions(tree, symbols)
	if err != nil || changed[0].TargetID != 11 || changed[0].Operand.Value != 41 || changed[2].Operator != "[-]" || changed[2].Operand.SymbolID != 11 {
		t.Fatalf("source change ignored: %+v, %v", changed, err)
	}
	if table.Records[3].Name != "[UNRESOLVED ACTION]" || table.Records[3].Cells[0].Float != 17 {
		t.Fatal("uncompiled action changed; scalar index is not an executable event")
	}
}

func TestDelezieScalarRejectsMissingSymbolsAndInvalidNativeOperands(t *testing.T) {
	s := func(v string) pvf.CTPCell { return pvf.CTPCell{Tag: 1, Kind: "name", Name: v} }
	n := func(v float64) pvf.CTPCell { return pvf.CTPCell{Tag: 4, Kind: "float", Float: v} }
	for _, tc := range []struct {
		name  string
		kind  string
		cells []pvf.CTPCell
	}{
		{"unknown target", "[SET]", []pvf.CTPCell{s("[UNKNOWN]"), n(0)}},
		{"unknown reference", "[IF]", []pvf.CTPCell{s("[X]"), s("[==]"), s("[UNKNOWN]")}},
		{"localized reference", "[IF]", []pvf.CTPCell{s("[X]"), s("[==]"), {Tag: 2, Kind: "name_indexed", Name: "[X]"}}},
		{"alt float", "[SET]", []pvf.CTPCell{s("[X]"), {Tag: 5, Kind: "float", Float: 1}}},
		{"unknown operation", "[SET]", []pvf.CTPCell{s("[X]"), s("[UNKNOWN]"), n(0)}},
		{"symbol shorthand", "[SET]", []pvf.CTPCell{s("[X]"), s("[X]")}},
		{"missing operand", "[IF]", []pvf.CTPCell{s("[X]"), s("[==]")}},
		{"extra operand", "[SET]", []pvf.CTPCell{s("[X]"), n(0), n(1), n(2)}},
		{"NaN", "[SET]", []pvf.CTPCell{s("[X]"), n(math.NaN())}},
		{"infinity", "[SET]", []pvf.CTPCell{s("[X]"), n(math.Inf(1))}},
		{"overflow", "[SET]", []pvf.CTPCell{s("[X]"), n(2147483648)}},
		{"underflow", "[SET]", []pvf.CTPCell{s("[X]"), n(-2147483649)}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tree, err := pvf.NewCTPTree(&pvf.CTPTable{Path: "invalid.ctp", Records: []pvf.CTPRecord{{Index: 0, Parent: -1, Name: tc.kind, Cells: tc.cells}}})
			if err != nil {
				t.Fatal(err)
			}
			_, err = parseDelezieScalarExpressions(tree, map[string]uint32{"[X]": 0})
			if err == nil || !strings.Contains(err.Error(), "invalid.ctp row 0") {
				t.Fatalf("missing source-aware error: %v", err)
			}
		})
	}
}

func TestDelezieScalarAcceptsNativeSignedBoundaries(t *testing.T) {
	for _, v := range []float64{0, math.MinInt32, math.MaxInt32, -2147483648.5, 2147483647.5} {
		got, err := delezieScalarOperand(pvf.CTPCell{Tag: 4, Kind: "float", Float: v}, nil)
		if err != nil || got.IsSymbol || got.Value != int32(math.Trunc(v)) {
			t.Fatalf("%v: %+v %v", v, got, err)
		}
	}
}
