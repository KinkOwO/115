package raid_test

import (
	"dfolan/internal/catalog"
	"dfolan/internal/catalog/pvf"
	"dfolan/internal/raid"
	"reflect"
	"testing"
)

func combinationFixture(t *testing.T) (*catalog.DelezieRaidRules, *catalog.DelezieModeRules, int) {
	t.Helper()
	table := &pvf.CTPTable{Path: "combine.object.ctp"}
	add := func(parent int, name string, cells ...pvf.CTPCell) int {
		i := len(table.Records)
		table.Records = append(table.Records, pvf.CTPRecord{Index: i, Parent: parent, Name: name, Cells: cells})
		if parent >= 0 {
			table.Records[parent].Refs = append(table.Records[parent].Refs, pvf.CTPRef{Name: name, Values: []uint64{uint64(i)}})
		}
		return i
	}
	objects := add(-1, "[OBJECTS]")
	for _, id := range []uint32{1, 2, 3} {
		o := add(objects, "[OBJECT]")
		add(o, "[INDEX]", scalarNumber(int32(id)))
		add(o, "[STATIC HP]", scalarNumber(int32(20+id)))
		add(o, "[CREATABLE LOCATION INDEX]", scalarNumber(7))
		if id != 2 {
			add(o, "[AI INDEX]", scalarNumber(7))
		}
	}
	combos := add(-1, "[OBJECT COMBOS]")
	recipe := add(combos, "[OBJECT COMBO]")
	add(recipe, "[MATERIAL]", scalarNumber(1), scalarNumber(2))
	add(recipe, "[RESULT]", scalarNumber(3))
	ais := add(-1, "[AIS]")
	ai := add(ais, "[AI]")
	add(ai, "[INDEX]", scalarNumber(7))
	add(ai, "[START RESULT]", pvf.CTPCell{Tag: 1, Kind: "name", Name: "MOVING ISLAND"}, scalarNumber(30))
	add(ai, "[RESULTS]", pvf.CTPCell{Tag: 1, Kind: "name", Name: "CONSUMING"}, scalarNumber(6), scalarNumber(-1))
	tree, err := pvf.NewCTPTree(table)
	if err != nil {
		t.Fatal(err)
	}
	mode := &catalog.DelezieModeRules{Difficulty: catalog.DelezieNormal, Objects: tree, Phases: []catalog.DeleziePhaseRules{{Locations: map[uint32]catalog.DelezieLocationRule{7: {ID: 7}, 9: {ID: 9}}}}}
	rules := &catalog.DelezieRaidRules{Modes: map[catalog.DelezieDifficulty]*catalog.DelezieModeRules{mode.Difficulty: mode}}
	return rules, mode, recipe
}

func TestDelezieResolvedCombinationAtomicSourceResultAndAIIdentity(t *testing.T) {
	rules, mode, row := combinationFixture(t)
	state, err := raid.NewDelezieObjects(rules, mode, 0)
	if err != nil {
		t.Fatal(err)
	}
	objects, err := state.Spawn([]raid.DelezieObjectSpawn{{Definition: 1, Location: 7}, {Definition: 2, Location: 7}})
	if err != nil {
		t.Fatal(err)
	}
	program, err := state.AIProgram(objects[0].Key)
	if err != nil || program.ID != 7 || program.StartResults[0].First != 30 || program.Results[0].Second != -1 {
		t.Fatal(program, err)
	}
	program.StartResults[0].First = 99
	if again, err := state.AIProgram(objects[0].Key); err != nil || again.StartResults[0].First != 30 {
		t.Fatal("AI snapshot mutated", again, err)
	}
	if _, err := state.AIProgram(objects[1].Key); err == nil {
		t.Fatal("unbound object borrowed AI")
	}
	valid := raid.DelezieResolvedCombination{RecipeRow: row, Materials: [2]uint32{objects[0].Key, objects[1].Key}, Location: 7}
	before := state.Snapshot()
	for _, bad := range []raid.DelezieResolvedCombination{
		{RecipeRow: row, Materials: [2]uint32{objects[1].Key, objects[0].Key}, Location: 7},
		{RecipeRow: row, Materials: [2]uint32{objects[0].Key, objects[0].Key}, Location: 7},
		{RecipeRow: row + 100, Materials: valid.Materials, Location: 7},
		{RecipeRow: row, Materials: valid.Materials, Location: 9},
		{RecipeRow: row, Materials: [2]uint32{objects[0].Key, 99}, Location: 7},
	} {
		if got, err := state.CombineResolved([]raid.DelezieResolvedCombination{bad}); err == nil || got != nil || !reflect.DeepEqual(before, state.Snapshot()) {
			t.Fatal("invalid combination mutated state", bad, got, err)
		}
	}
	if got, err := state.CombineResolved([]raid.DelezieResolvedCombination{valid, valid}); err == nil || got != nil || !reflect.DeepEqual(before, state.Snapshot()) {
		t.Fatal("batch failure retained result", got, err)
	}
	// Change caller-visible source after construction; the state owns its snapshot.
	for i := range mode.Objects.Table.Records {
		r := &mode.Objects.Table.Records[i]
		if r.Name == "[RESULT]" {
			r.Cells[0] = scalarNumber(1)
		}
	}
	changed, err := state.CombineResolved([]raid.DelezieResolvedCombination{valid})
	if err != nil || len(changed) != 1 || changed[0].Created.Key != 3 || changed[0].Created.Definition != 3 || changed[0].Created.HP != 23 || len(state.Snapshot()) != 1 {
		t.Fatal(changed, err)
	}
	if _, err := state.AIProgram(objects[0].Key); err == nil {
		t.Fatal("consumed identity retained AI")
	}
	if resultAI, err := state.AIProgram(changed[0].Created.Key); err != nil || resultAI.ID != 7 {
		t.Fatal("result lost source AI", resultAI, err)
	}
	if _, err := state.CombineResolved([]raid.DelezieResolvedCombination{valid}); err == nil {
		t.Fatal("combination replay accepted")
	}
	fresh, err := raid.NewDelezieObjects(rules, mode, 0)
	if err != nil {
		t.Fatal(err)
	}
	_, err = fresh.Spawn([]raid.DelezieObjectSpawn{{Definition: 1, Location: 7}, {Definition: 2, Location: 7}})
	if err != nil {
		t.Fatal(err)
	}
	result, err := fresh.CombineResolved([]raid.DelezieResolvedCombination{valid})
	if err != nil || result[0].Created.Definition != 1 {
		t.Fatal("new source ignored", result, err)
	}
}
