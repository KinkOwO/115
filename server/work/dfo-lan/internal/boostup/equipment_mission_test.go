package boostup

import (
	"dfolan/internal/catalog/pvf"
	"testing"
)

// 起步装备的源分组身份由 character.FameRules.Groups 独占解析，
// 其对内层 PVF 的钉桩见 internal/character/boostup_groups_test.go。

type testGroups map[uint32][]int32

func (g testGroups) ForTemplate(id uint32) []int32 { return g[id] }

func TestBoostWearMissionRequiresAllKindsAndDistinctSlots(t *testing.T) {
	r := &WearRequirement{Kinds: []string{"[title name]", "[creature]", "[aurora avatar]"}}
	items := []WornFact{{13, 1, "[title name]"}, {26, 2, "[creature]"}, {9, 3, "[aurora avatar]"}}
	if ok, e := r.Satisfied(items[:2], nil); e != nil || ok {
		t.Fatal("missing aura passed", e)
	}
	if ok, e := r.Satisfied(items, nil); e != nil || !ok {
		t.Fatal(ok, e)
	}
	items[2].Slot = 13
	if _, e := r.Satisfied(items, nil); e == nil {
		t.Fatal("same slot counted twice")
	}
}

func TestBoostWearGroupingIsUnionNotSumOfMemberships(t *testing.T) {
	r := &WearRequirement{Count: 11, Groups: []int32{77, 161, 241}}
	g := testGroups{}
	var items []WornFact
	for i := 0; i < 11; i++ {
		id := uint32(i + 1)
		g[id] = []int32{77, 161, 241}
		items = append(items, WornFact{uint16(i), id, "[coat avatar]"})
	}
	if ok, e := r.Satisfied(items[:10], g); e != nil || ok {
		t.Fatal("overlapping group tripled count", e)
	}
	if ok, e := r.Satisfied(items, g); e != nil || !ok {
		t.Fatal(ok, e)
	}
	delete(g, 11)
	if ok, e := r.Satisfied(items, g); e != nil || ok {
		t.Fatal("foreign outfit passed", e)
	}
	if _, e := r.Satisfied(items, nil); e == nil {
		t.Fatal("missing group source treated as satisfied")
	}
}

func TestBoostWearSourceUnknownClauseDoesNotAutoComplete(t *testing.T) {
	row := Step{Mission: "equip item", MissionCells: []pvf.Token{
		{Type: 3, Text: "[equip condition]"}, {Type: 3, Text: "[type]"}, {Type: 6, Text: "title"},
		{Type: 3, Text: "[condition]"}, {Type: 0, Value: 99}, {Type: 3, Text: "[/condition]"}, {Type: 3, Text: "[/equip condition]"},
	}}
	if _, e := row.WearRequirement(); e == nil {
		t.Fatal("unmodelled item restriction ignored")
	}
}
