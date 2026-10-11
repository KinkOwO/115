package catalog

import (
	"dfolan/internal/catalog/pvf"
	"strings"
	"testing"
)

func mechanismBindingsFixture(t *testing.T) (*pvf.CTPTree, map[string]int) {
	t.Helper()
	table := &pvf.CTPTable{Path: "mechanisms.ctp"}
	indices := map[string]int{}
	add := func(parent int, name string, cells ...pvf.CTPCell) int {
		i := len(table.Records)
		table.Records = append(table.Records, pvf.CTPRecord{Index: i, Parent: parent, Name: name, Cells: cells})
		if parent >= 0 {
			table.Records[parent].Refs = append(table.Records[parent].Refs, pvf.CTPRef{Name: name, Values: []uint64{uint64(i)}})
		}
		return i
	}
	s := func(name string) pvf.CTPCell { return pvf.CTPCell{Tag: 1, Kind: "name", Name: name} }
	root := add(-1, "[DELEZIE ETC SYMBOL REFERENCE]")
	indices["phase"] = add(root, "[PHASE]", s("[X]"))
	root = add(-1, "[DELEZIE ETC TIMER REFERENCE]")
	indices["timer"] = add(root, "[CUTSCENE]", s("[A]"), s("[NONE]"))
	root = add(-1, "[DELEZIE SYNTHESIZE]")
	indices["synthesis"] = add(root, "[SYNTHESIZE COUNT SYMBOL]", s("[X]"))
	root = add(-1, "[DELEZIE NOBLE SACRIFICE]")
	rule := add(root, "[NOBLE SACRIFICE]")
	indices["sacrifice"] = add(rule, "[SYMBOL]", s("[X]"))
	tree, err := pvf.NewCTPTree(table)
	if err != nil {
		t.Fatal(err)
	}
	return tree, indices
}

func TestDelezieMechanismBindingsUseSourceAndRejectMissingReferences(t *testing.T) {
	timers := map[string]DelezieTimerRule{"[A]": {ID: 0, Name: "[A]", Subtypes: map[string]uint32{"[NONE]": 0}}}
	tree, rows := mechanismBindingsFixture(t)
	first, err := parseDelezieMechanismBindings(tree, map[string]uint32{"[X]": 0}, timers)
	if err != nil || len(first.Symbols) != 3 || len(first.Timers) != 1 || first.Symbols[rows["phase"]].ID != 0 || first.Timers[rows["timer"]].HasValue {
		t.Fatal(first, err)
	}
	timers["[A]"] = DelezieTimerRule{ID: 7, Name: "[A]", Subtypes: map[string]uint32{"[NONE]": 3}}
	changed, err := parseDelezieMechanismBindings(tree, map[string]uint32{"[X]": 9}, timers)
	if err != nil || changed.Symbols[rows["sacrifice"]].ID != 9 || changed.Timers[rows["timer"]].Key != (DelezieTimerKey{7, 3}) {
		t.Fatal(changed, err)
	}
	if first.Symbols[rows["phase"]].ID != 0 || first.Timers[rows["timer"]].Key.Main != 0 {
		t.Fatal("new source changed old bindings")
	}
	for _, tc := range []struct {
		name    string
		symbols map[string]uint32
		timers  map[string]DelezieTimerRule
	}{
		{"missing symbol", nil, timers},
		{"missing timer", map[string]uint32{"[X]": 9}, nil},
		{"wrong scoped subtype", map[string]uint32{"[X]": 9}, map[string]DelezieTimerRule{"[A]": {ID: 7, Subtypes: map[string]uint32{"[OTHER]": 0}}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := parseDelezieMechanismBindings(tree, tc.symbols, tc.timers)
			if err == nil || !strings.Contains(err.Error(), "mechanisms.ctp row") {
				t.Fatal(err)
			}
		})
	}
}

func TestDelezieMechanismBindingsRejectMalformedFieldsAndKeepAbsentModesEmpty(t *testing.T) {
	for _, target := range []string{"phase", "timer", "synthesis", "sacrifice"} {
		t.Run(target, func(t *testing.T) {
			tree, rows := mechanismBindingsFixture(t)
			r := &tree.Table.Records[rows[target]]
			r.Cells = append(r.Cells, pvf.CTPCell{Tag: 4, Kind: "float", Float: 1})
			if _, err := parseDelezieMechanismBindings(tree, map[string]uint32{"[X]": 0}, map[string]DelezieTimerRule{"[A]": {ID: 1, Subtypes: map[string]uint32{"[NONE]": 0}}}); err == nil {
				t.Fatal("malformed reference accepted")
			}
		})
	}
	tree, err := pvf.NewCTPTree(&pvf.CTPTable{Path: "hard.ctp"})
	if err != nil {
		t.Fatal(err)
	}
	bindings, err := parseDelezieMechanismBindings(tree, nil, nil)
	if err != nil || len(bindings.Symbols) != 0 || len(bindings.Timers) != 0 {
		t.Fatal("absent mode inherited special rules", bindings, err)
	}
}
