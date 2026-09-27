package quest

import (
	"dfolan/internal/catalog"
	"testing"
)

func TestSkywarUseItemQuestRoute(t *testing.T) {
	quests, err := catalog.LoadQuests("../../configs/quests.generated.json")
	if err != nil {
		t.Fatal(err)
	}
	x := BuildIndex(quests)
	d := quests.Quests[12122]
	item, ok := UseItemObjective(d)
	if !ok || item != 10312154 {
		t.Fatalf("quest 12122 source use item = %d, %t", item, ok)
	}
	en := x.Entries[12122]
	if en == nil || !en.Implemented || !en.RewardUsable || en.Model != SingleUseItem ||
		en.Initial != 1 || en.UseItem != item || len(x.ByUseItem[item]) != 1 || x.ByUseItem[item][0] != 12122 {
		t.Fatalf("quest 12122 unavailable through the item-use route: %+v", en)
	}
	if !prerequisitesMet(en.PrerequisiteGroups, map[uint32]string{13595: "completed"}) ||
		prerequisitesMet(en.PrerequisiteGroups, nil) {
		t.Fatal("quest 12122 prerequisite does not follow 13595")
	}
	next := x.Entries[12123]
	if next == nil || next.MinimumLevel != 95 ||
		!prerequisitesMet(next.PrerequisiteGroups, map[uint32]string{12122: "completed"}) {
		t.Fatal("level-95 successor does not follow 12122")
	}
	// The similarly shaped title quest has a different subtype and stays out
	// of the one-use epic model.
	if _, ok := UseItemObjective(quests.Quests[6532]); ok {
		t.Fatal("title quest was admitted as the skywar item-use objective")
	}
}
