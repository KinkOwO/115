package catalog

import (
	"dfolan/internal/catalog/pvf"
	"math"
	"testing"
)

func TestDelezieObjectHPUsesNative64BitReader(t *testing.T) {
	for _, n := range []float64{0, 10000.9, 4294967297, 0x1p53, -0x1p63} {
		v, err := delezieInt64(pvf.CTPCell{Tag: 4, Kind: "float", Float: n})
		if err != nil || v != int64(math.Trunc(n)) {
			t.Fatalf("native int64 %v: %v %v", n, v, err)
		}
	}
	for _, cell := range []pvf.CTPCell{{Tag: 4, Kind: "float", Float: 0x1p63}, {Tag: 4, Kind: "float", Float: math.Inf(1)}, {Tag: 4, Kind: "float", Float: math.NaN()}, {Tag: 5, Kind: "float", Float: 10000}, {Tag: 1, Kind: "name", Name: "HP"}} {
		if _, err := delezieInt64(cell); err == nil {
			t.Fatalf("invalid HP conversion accepted: %+v", cell)
		}
	}
}

func TestDelezieObjectBindingsFollowSourceAndPreserveAbsence(t *testing.T) {
	n := func(v float64) pvf.CTPCell { return pvf.CTPCell{Tag: 4, Kind: "float", Float: v} }
	rows := []pvf.CTPRecord{
		{Index: 0, Parent: -1, Name: "[OBJECTS]", Refs: []pvf.CTPRef{{Name: "[OBJECT]", Values: []uint64{1, 6}}}},
		{Index: 1, Parent: 0, Name: "[OBJECT]", Refs: []pvf.CTPRef{{Name: "[INDEX]", Values: []uint64{2}}, {Name: "[STATIC HP]", Values: []uint64{3}}, {Name: "[CREATABLE LOCATION INDEX]", Values: []uint64{4}}, {Name: "[LOCK SYMBOL]", Values: []uint64{5}}}},
		{Index: 2, Parent: 1, Name: "[INDEX]", Cells: []pvf.CTPCell{n(1)}},
		{Index: 3, Parent: 1, Name: "[STATIC HP]", Cells: []pvf.CTPCell{n(4294967297)}},
		{Index: 4, Parent: 1, Name: "[CREATABLE LOCATION INDEX]", Cells: []pvf.CTPCell{n(3), n(4)}},
		{Index: 5, Parent: 1, Name: "[LOCK SYMBOL]", Cells: []pvf.CTPCell{{Tag: 1, Kind: "name", Name: "[LOCK]"}}},
		{Index: 6, Parent: 0, Name: "[OBJECT]", Refs: []pvf.CTPRef{{Name: "[INDEX]", Values: []uint64{7}}}},
		{Index: 7, Parent: 6, Name: "[INDEX]", Cells: []pvf.CTPCell{n(2)}},
	}
	tree, err := pvf.NewCTPTree(&pvf.CTPTable{Path: "objects.ctp", Records: rows})
	if err != nil {
		t.Fatal(err)
	}
	symbols := map[string]uint32{"[LOCK]": 0}
	objects, err := parseDelezieObjects(tree, symbols)
	if err != nil {
		t.Fatal(err)
	}
	if !objects[1].HasStaticHP || objects[1].StaticHP != 4294967297 || objects[1].LockSymbolID != 0 || objects[2].HasStaticHP || len(objects[2].CreatableLocations) != 0 {
		t.Fatalf("object bindings: %+v", objects)
	}
	tree.Table.Records[3].Cells[0] = n(13)
	tree.Table.Records[4].Cells[0] = n(9)
	symbols["[LOCK]"] = 7
	changed, err := parseDelezieObjects(tree, symbols)
	if err != nil || changed[1].StaticHP != 13 || changed[1].CreatableLocations[0] != 9 || changed[1].LockSymbolID != 7 {
		t.Fatalf("source ignored: %+v %v", changed, err)
	}
	delete(symbols, "[LOCK]")
	if _, err := parseDelezieObjects(tree, symbols); err == nil {
		t.Fatal("undeclared lock accepted as zero")
	}
}
