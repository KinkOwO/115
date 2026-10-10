package catalog

import (
	"dfolan/internal/catalog/pvf"
	"strings"
	"testing"
)

func TestDelezieTimerBindingsKeepScopedZeroAndSignedValues(t *testing.T) {
	s := func(v string) pvf.CTPCell { return pvf.CTPCell{Tag: 1, Kind: "name", Name: v} }
	n := func(v float64) pvf.CTPCell { return pvf.CTPCell{Tag: 4, Kind: "float", Float: v} }
	timers := map[string]DelezieTimerRule{
		"[A]": {ID: 0, Name: "[A]", Subtypes: map[string]uint32{"[NONE]": 0}},
		"[B]": {ID: 2, Name: "[B]", Subtypes: map[string]uint32{"[NONE]": 0, "[B ONLY]": 1}},
	}
	table := &pvf.CTPTable{Path: "timers.ctp", Records: []pvf.CTPRecord{
		{Index: 0, Parent: -1, Name: "[SET TIMER]", Cells: []pvf.CTPCell{s("[A]"), s("[NONE]"), n(-1)}},
		{Index: 1, Parent: -1, Name: "[SET TIMER]", Cells: []pvf.CTPCell{s("[B]"), s("[NONE]"), n(0)}},
		{Index: 2, Parent: -1, Name: "[CHECK TIMER END]", Cells: []pvf.CTPCell{n(0), n(0)}},
		{Index: 3, Parent: -1, Name: "[RESET TIMER]", Cells: []pvf.CTPCell{s("[A]"), s("[NONE]")}},
	}}
	tree, err := pvf.NewCTPTree(table)
	if err != nil {
		t.Fatal(err)
	}
	rules, err := parseDelezieTimerReferences(tree, timers)
	if err != nil {
		t.Fatal(err)
	}
	if rules[0].Key == rules[1].Key || rules[0].Value != -1 || !rules[1].HasValue || rules[1].Value != 0 || rules[2].HasValue || rules[0].Key != rules[2].Key || rules[3].HasValue {
		t.Fatalf("timer identity/value lost: %+v", rules)
	}
	timers["[A]"] = DelezieTimerRule{ID: 8, Name: "[A]", Subtypes: map[string]uint32{"[NONE]": 3}}
	table.Records[2].Cells = []pvf.CTPCell{n(8), n(3)}
	table.Records[0].Cells[2] = n(17.9)
	changed, err := parseDelezieTimerReferences(tree, timers)
	if err != nil || changed[0].Key != (DelezieTimerKey{Main: 8, Sub: 3}) || changed[0].Value != 17 || changed[2].Key != changed[0].Key {
		t.Fatalf("changed source ignored: %+v %v", changed, err)
	}
}

func TestDelezieTimerBindingsRejectUndeclaredAndMalformedOperands(t *testing.T) {
	s := func(v string) pvf.CTPCell { return pvf.CTPCell{Tag: 1, Kind: "name", Name: v} }
	n := func(v float64) pvf.CTPCell { return pvf.CTPCell{Tag: 4, Kind: "float", Float: v} }
	timers := map[string]DelezieTimerRule{
		"[A]": {ID: 0, Name: "[A]", Subtypes: map[string]uint32{"[NONE]": 0}},
		"[B]": {ID: 2, Name: "[B]", Subtypes: map[string]uint32{"[B ONLY]": 1}},
	}
	for _, tc := range []struct {
		name, kind string
		cells      []pvf.CTPCell
	}{
		{"missing main", "[SET TIMER]", []pvf.CTPCell{s("[X]"), s("[NONE]"), n(1)}},
		{"cross-main subtype", "[CHECK TIMER END]", []pvf.CTPCell{s("[A]"), s("[B ONLY]")}},
		{"unknown numeric main", "[RESET TIMER]", []pvf.CTPCell{n(7), n(0)}},
		{"unknown numeric subtype", "[RESET TIMER]", []pvf.CTPCell{n(2), n(0)}},
		{"negative identity", "[RESET TIMER]", []pvf.CTPCell{n(-1), n(0)}},
		{"mixed identity", "[RESET TIMER]", []pvf.CTPCell{s("[A]"), n(0)}},
		{"symbol value", "[SET TIMER]", []pvf.CTPCell{s("[A]"), s("[NONE]"), s("[VALUE]")}},
		{"wrong arity", "[SET TIMER]", []pvf.CTPCell{s("[A]"), s("[NONE]")}},
		{"alternate numeric tag", "[SET TIMER]", []pvf.CTPCell{s("[A]"), s("[NONE]"), {Tag: 5, Kind: "float", Float: 1}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tree, err := pvf.NewCTPTree(&pvf.CTPTable{Path: "invalid-timer.ctp", Records: []pvf.CTPRecord{{Index: 0, Parent: -1, Name: tc.kind, Cells: tc.cells}}})
			if err != nil {
				t.Fatal(err)
			}
			if _, err = parseDelezieTimerReferences(tree, timers); err == nil || !strings.Contains(err.Error(), "invalid-timer.ctp row 0") {
				t.Fatalf("missing source-aware failure: %v", err)
			}
		})
	}
}
