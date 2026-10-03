package quest

import (
	"dfolan/internal/catalog"
	"dfolan/internal/catalog/pvf"
	"dfolan/internal/testfixture"
	"testing"
)

// numericCells builds a Type-0 (literal integer) objective run, which is the only
// cell type either model accepts.
func numericCells(values ...int32) []pvf.Token {
	out := make([]pvf.Token, 0, len(values))
	for _, v := range values {
		out = append(out, pvf.Token{Type: 0, Value: v})
	}
	return out
}

// The two objectives below are the "legion content" family. Both are settled as
// client-gated (initial progress 0), which is a deliberate downgrade recorded in
// analysis/tasks/next64-legion-apocalypse-plan.md §6.2 and explained at the
// implementation sites in progress.go. These tests pin two things that must not
// drift:
//
//  1. the shape validation accepts exactly the source shapes that exist today, so
//     a renumbered or restructured client falls back to "unimplemented" instead of
//     being silently accepted under a model whose semantics we cannot state;
//  2. the apocalypse guide quest (23128, content 6) is offered, because that is
//     what puts 末世录 into the adventure guide at all.

func loadQuestCatalog(t *testing.T) catalog.QuestCatalog {
	t.Helper()
	c, e := catalog.LoadQuests(testfixture.CatalogPath(t, "quests"))
	if e != nil {
		t.Fatal(e)
	}
	return c
}

func TestMonsterKillCheckpointShapeAcceptsSourceLayout(t *testing.T) {
	c := loadQuestCatalog(t)
	// Every quest in the catalog that declares the kind must be accepted: if one
	// is not, it drops out of the acceptable list and its content line is
	// unreachable again (that is exactly how the 2026 chain was blocked).
	found := 0
	for id, d := range c.Quests {
		if d.Kind != "[monster kill checkpoint]" {
			continue
		}
		found++
		if !MonsterKillCheckpointShape(d) {
			t.Errorf("quest %d (%s) declares [monster kill checkpoint] but the shape was rejected; "+
				"objective cells: %v", id, d.Script.Path, d.ObjectiveCells)
		}
		initial, model, e := InitialProgress(d)
		if e != nil {
			t.Errorf("quest %d: InitialProgress: %v", id, e)
			continue
		}
		if model != MonsterKillCheckpoint || initial != 0 {
			t.Errorf("quest %d: model=%q initial=%d, want %q/0", id, model, initial, MonsterKillCheckpoint)
		}
	}
	if found != 4 {
		t.Errorf("[monster kill checkpoint] is used by %d quests, want 4 (23053/23054/23055/23099); "+
			"the source moved - re-walk the chain before trusting this model", found)
	}
}

func TestMonsterKillCheckpointShapeRejectsOtherLayouts(t *testing.T) {
	base := catalog.QuestDefinition{Kind: "[monster kill checkpoint]"}
	cases := map[string]catalog.QuestDefinition{
		"wrong kind":        {Kind: "[clear map]"},
		"no terminator":     {Kind: base.Kind, ObjectiveCells: numericCells(1, 2, 3)},
		"not a triple":      {Kind: base.Kind, ObjectiveCells: numericCells(1, 2, 3, 4)},
		"negative key":      {Kind: base.Kind, ObjectiveCells: numericCells(-1, 2, -1)},
		"keys not ordered":  {Kind: base.Kind, ObjectiveCells: numericCells(9, 1, 1, 2, 1, -1, 3, -1, -1)},
		"pending condition": {Kind: base.Kind, ObjectiveCells: numericCells(1, 2, -1), Pending: []string{"x"}},
	}
	for name, d := range cases {
		if MonsterKillCheckpointShape(d) {
			t.Errorf("%s: shape accepted, want rejected", name)
		}
	}
}

func TestLegionContentClearCoverAndGuideQuest(t *testing.T) {
	c := loadQuestCatalog(t)
	x := BuildIndex(c)

	// The apocalypse guide quest is the one that makes the guide entry exist:
	// its [go guide] cell is 239 and its objective names content 6.
	guide, ok := c.Quests[23128]
	if !ok {
		t.Fatal("apocalypse guide quest 23128 is absent from the catalog")
	}
	if guide.Kind != "[legion content clear with difficulty]" {
		t.Fatalf("quest 23128 kind = %q, want [legion content clear with difficulty]", guide.Kind)
	}
	if len(guide.ObjectiveCells) == 0 || guide.ObjectiveCells[0].Value != 6 {
		t.Fatalf("quest 23128 objective content index = %v, want 6 (apocalypse)",
			guide.ObjectiveCells)
	}
	en := x.Entries[23128]
	if en == nil || !en.Implemented {
		t.Fatal("quest 23128 is not offered; the apocalypse entry stays missing from the adventure guide")
	}

	// Every quest of the sibling kind must be accepted too, or the same silent
	// drop hits other content guides.
	for id, d := range c.Quests {
		if d.Kind != "[legion content clear]" {
			continue
		}
		if !LegionContentClearShape(d) {
			t.Errorf("quest %d (%s) declares [legion content clear] but the shape was rejected; cells: %v",
				id, d.Script.Path, d.ObjectiveCells)
		}
	}
}

func TestLegionContentClearShapeRejectsOtherLayouts(t *testing.T) {
	cases := map[string]catalog.QuestDefinition{
		"wrong kind":       {Kind: "[legion operation clear]", ObjectiveCells: numericCells(5, 1)},
		"too short":        {Kind: "[legion content clear]", ObjectiveCells: numericCells(5)},
		"too long":         {Kind: "[legion content clear]", ObjectiveCells: numericCells(5, 1, 1, 1)},
		"negative content": {Kind: "[legion content clear]", ObjectiveCells: numericCells(-1, 1)},
		"difficulty len":   {Kind: "[legion content clear with difficulty]", ObjectiveCells: numericCells(6, 1, 2)},
		"pending":          {Kind: "[legion content clear]", ObjectiveCells: numericCells(5, 1), Pending: []string{"x"}},
	}
	for name, d := range cases {
		if LegionContentClearShape(d) {
			t.Errorf("%s: shape accepted, want rejected", name)
		}
	}
}
