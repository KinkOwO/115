package catalog

import (
	"dfolan/internal/catalog/pvf"
	"reflect"
	"testing"
)

func delezieObjectGraphFixture() (*pvf.CTPTable, map[string]int) {
	table := &pvf.CTPTable{Path: "object-graph.ctp"}
	rows := map[string]int{}
	add := func(parent int, name string, cells ...pvf.CTPCell) int {
		i := len(table.Records)
		table.Records = append(table.Records, pvf.CTPRecord{Index: i, Parent: parent, Name: name, Cells: cells})
		if parent >= 0 {
			table.Records[parent].Refs = append(table.Records[parent].Refs, pvf.CTPRef{Name: name, Values: []uint64{uint64(i)}})
		}
		return i
	}
	n := func(v float64) pvf.CTPCell { return pvf.CTPCell{Tag: 4, Kind: "float", Float: v} }
	s := func(v string) pvf.CTPCell { return pvf.CTPCell{Tag: 1, Kind: "name", Name: v} }
	objects := add(-1, "[OBJECTS]")
	for _, id := range []uint32{1, 2, 3} {
		r := add(objects, "[OBJECT]")
		i := add(r, "[INDEX]", n(float64(id)))
		if id == 1 {
			rows["objectAI"] = add(r, "[AI INDEX]", n(7))
		}
		if id == 3 {
			rows["object3"] = i
		}
	}
	presets := add(-1, "[OBJECT PRESETS]")
	preset := add(presets, "[OBJECT PRESET]")
	rows["presetID"] = add(preset, "[INDEX]", n(20))
	rows["presetObjects"] = add(preset, "[OBJECT INDEX]", n(1), n(1), n(2))
	combos := add(-1, "[OBJECT COMBOS]")
	combo := add(combos, "[OBJECT COMBO]")
	rows["materials"] = add(combo, "[MATERIAL]", n(1), n(2))
	rows["result"] = add(combo, "[RESULT]", n(3))
	ais := add(-1, "[AIS]")
	ai := add(ais, "[AI]")
	rows["aiID"] = add(ai, "[INDEX]", n(7))
	rows["aiStart"] = add(ai, "[START RESULT]", s("MOVING ISLAND"), n(30))
	rows["aiResults"] = add(ai, "[RESULTS]", s("CONSUMING"), n(6), n(0), s("CLASH"), n(300), n(-1))
	return table, rows
}

func TestDelezieObjectGraphFollowsModeSourceAndPreservesRepeatedObjects(t *testing.T) {
	table, rows := delezieObjectGraphFixture()
	parse := func() *DelezieObjectGraph {
		tree, err := pvf.NewCTPTree(table)
		if err != nil {
			t.Fatal(err)
		}
		objects, err := parseDelezieObjects(tree, nil)
		if err != nil {
			t.Fatal(err)
		}
		g, err := parseDelezieObjectGraph(tree, objects)
		if err != nil {
			t.Fatal(err)
		}
		return g
	}
	g := parse()
	if !reflect.DeepEqual(g.Presets[20].Objects, []uint32{1, 1, 2}) || g.Combos[0].Materials != ([2]uint32{1, 2}) || g.Combos[0].Result != 3 || g.ObjectAIs[1] != 7 {
		t.Fatalf("source graph lost: %+v", g)
	}
	if len(g.AIs[7].Results) != 2 || g.AIs[7].StartResults[0].HasSecond || !g.AIs[7].Results[1].HasSecond || g.AIs[7].Results[1].Second != -1 {
		t.Fatalf("AI operand grouping changed: %+v", g.AIs[7])
	}
	table.Records[rows["presetID"]].Cells[0].Float = 25
	table.Records[rows["object3"]].Cells[0].Float = 9
	table.Records[rows["result"]].Cells[0].Float = 9
	table.Records[rows["aiID"]].Cells[0].Float = 11
	table.Records[rows["objectAI"]].Cells[0].Float = 11
	table.Records[rows["aiStart"]].Cells[1].Float = 41
	changed := parse()
	if _, ok := changed.Presets[20]; ok {
		t.Fatal("old preset ID survived source change")
	}
	if changed.Presets[25].ID != 25 || changed.Combos[0].Result != 9 || changed.ObjectAIs[1] != 11 || changed.AIs[11].StartResults[0].First != 41 {
		t.Fatalf("source change ignored: %+v", changed)
	}
}

func TestDelezieObjectGraphRejectsBrokenModeReferencesAndAIGrammar(t *testing.T) {
	for _, tc := range []struct {
		name, row string
		mutate    func(*pvf.CTPRecord)
	}{
		{"missing preset object", "presetObjects", func(r *pvf.CTPRecord) { r.Cells[0].Float = 99 }},
		{"missing combo material", "materials", func(r *pvf.CTPRecord) { r.Cells[1].Float = 99 }},
		{"missing combo result", "result", func(r *pvf.CTPRecord) { r.Cells[0].Float = 99 }},
		{"wrong material count", "materials", func(r *pvf.CTPRecord) { r.Cells = r.Cells[:1] }},
		{"missing AI", "objectAI", func(r *pvf.CTPRecord) { r.Cells[0].Float = 99 }},
		{"unknown AI kind", "aiStart", func(r *pvf.CTPRecord) { r.Cells[0].Name = "UNKNOWN" }},
		{"incomplete start pair", "aiStart", func(r *pvf.CTPRecord) { r.Cells = r.Cells[:1] }},
		{"incomplete result triple", "aiResults", func(r *pvf.CTPRecord) { r.Cells = r.Cells[:5] }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			table, rows := delezieObjectGraphFixture()
			tc.mutate(&table.Records[rows[tc.row]])
			tree, err := pvf.NewCTPTree(table)
			if err != nil {
				t.Fatal(err)
			}
			objects, err := parseDelezieObjects(tree, nil)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := parseDelezieObjectGraph(tree, objects); err == nil {
				t.Fatal("broken reference or AI silently accepted")
			}
		})
	}
}

func TestDelezieObjectGraphAllowsModeWithoutPresetsOrCombos(t *testing.T) {
	table := &pvf.CTPTable{Path: "minimal.ctp"}
	tree, err := pvf.NewCTPTree(table)
	if err != nil {
		t.Fatal(err)
	}
	g, err := parseDelezieObjectGraph(tree, map[uint32]DelezieObjectRule{})
	if err != nil {
		t.Fatal(err)
	}
	if len(g.Presets) != 0 || len(g.Combos) != 0 || len(g.AIs) != 0 {
		t.Fatal("mode inherited content that source omits")
	}
}
