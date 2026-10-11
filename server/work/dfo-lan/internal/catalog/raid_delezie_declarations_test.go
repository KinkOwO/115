package catalog

import (
	"dfolan/internal/catalog/pvf"
	"math"
	"testing"
)

func declarationTree(t *testing.T, rows []pvf.CTPRecord) *pvf.CTPTree {
	t.Helper()
	for i := range rows {
		rows[i].Index = i
		if rows[i].Parent >= 0 {
			p := &rows[rows[i].Parent]
			p.Refs = append(p.Refs, pvf.CTPRef{Name: rows[i].Name, Values: []uint64{uint64(i)}})
		}
	}
	tree, err := pvf.NewCTPTree(&pvf.CTPTable{Path: "declarations.ctp", Records: rows})
	if err != nil {
		t.Fatal(err)
	}
	return tree
}

func declarationCells(id float64, name string) []pvf.CTPCell {
	return []pvf.CTPCell{{Kind: "float", Float: id}, {Kind: "name", Name: name}}
}

func TestDelezieDeclarationSourceReplacement(t *testing.T) {
	tree := declarationTree(t, []pvf.CTPRecord{{Parent: -1, Name: "[SYMBOL]", Cells: declarationCells(7, "[STATE]")}})
	first, err := parseDelezieSymbols(tree)
	if err != nil || first["[STATE]"] != 7 {
		t.Fatalf("initial source: %v, %v", first, err)
	}
	tree.Table.Records[0].Cells[0].Float = 31
	second, err := parseDelezieSymbols(tree)
	if err != nil || second["[STATE]"] != 31 {
		t.Fatalf("changed source: %v, %v", second, err)
	}
}

func TestDelezieDeclarationRejectsAmbiguousNamesAndNumbers(t *testing.T) {
	valid := declarationCells(3, "[A]")
	cases := map[string][]pvf.CTPCell{
		"empty":          nil,
		"unpaired":       {{Kind: "float", Float: 3}},
		"negative":       declarationCells(-1, "[A]"),
		"fraction":       declarationCells(0.5, "[A]"),
		"overflow":       declarationCells(math.MaxUint32+1, "[A]"),
		"nonfinite":      declarationCells(math.NaN(), "[A]"),
		"localized":      {{Kind: "float", Float: 3}, {Kind: "name_indexed", Name: "[A]"}},
		"duplicate ID":   append(append([]pvf.CTPCell{}, valid...), declarationCells(3, "[B]")...),
		"duplicate name": append(append([]pvf.CTPCell{}, valid...), declarationCells(4, "[A]")...),
	}
	for name, cells := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := delezieNamedIDs(pvf.CTPRecord{Cells: cells}); err == nil {
				t.Fatal("invalid declaration accepted")
			}
		})
	}
}

func TestDelezieTimerSubtypesAreScopedAndZeroIsValid(t *testing.T) {
	tree := declarationTree(t, []pvf.CTPRecord{
		{Parent: -1, Name: "[TIMER]"},
		{Parent: 0, Name: "[MAIN TYPE]", Cells: declarationCells(10, "[A]")},
		{Parent: 0, Name: "[SUB TYPE]", Cells: declarationCells(0, "[NONE]")},
		{Parent: -1, Name: "[TIMER]"},
		{Parent: 3, Name: "[MAIN TYPE]", Cells: declarationCells(20, "[B]")},
		{Parent: 3, Name: "[SUB TYPE]", Cells: declarationCells(0, "[NONE]")},
	})
	timers, err := parseDelezieTimers(tree)
	if err != nil {
		t.Fatal(err)
	}
	if len(timers) != 2 || timers["[A]"].ID == timers["[B]"].ID {
		t.Fatal("timer identities merged")
	}
	for _, timer := range timers {
		id, exists := timer.Subtypes["[NONE]"]
		if !exists || id != 0 {
			t.Fatal("valid subtype zero treated as missing")
		}
	}
	tree.Table.Records[4].Cells[0].Float = 10
	if _, err := parseDelezieTimers(tree); err == nil {
		t.Fatal("conflicting main IDs accepted")
	}
}

func TestDelezieBuffTypeIndexKeepsPassiveLessEffects(t *testing.T) {
	tree := declarationTree(t, []pvf.CTPRecord{
		{Parent: -1, Name: "[RAID BUFF]"},
		{Parent: 0, Name: "[BUFF INFO]"},
		{Parent: 1, Name: "[TYPE]", Cells: []pvf.CTPCell{{Kind: "name", Name: "Debuff"}}},
		{Parent: 1, Name: "[BUFF PASSIVE INDEX]", Cells: []pvf.CTPCell{{Kind: "float", Float: -1}}},
		{Parent: 0, Name: "[BUFF INFO]"},
		{Parent: 4, Name: "[TYPE]", Cells: []pvf.CTPCell{{Kind: "name", Name: "Help"}}},
	})
	buffs, err := parseDelezieBuffs(tree)
	if err != nil || len(buffs) != 2 || buffs["Debuff"].Row != 1 {
		t.Fatalf("passive-less buff lost: %v, %v", buffs, err)
	}
	if tree.Children(buffs["Debuff"].Row, "[BUFF PASSIVE INDEX]")[0].Cells[0].Float != -1 {
		t.Fatal("passive index fabricated")
	}
	tree.Table.Records[5].Cells[0].Name = "Debuff"
	if _, err := parseDelezieBuffs(tree); err == nil {
		t.Fatal("duplicate buff type silently replaced")
	}
}
