package catalog

import "testing"

func TestTownArrivalSceneWhitelistCurrentPVF(t *testing.T) {
	quests, err := LoadQuests("../../configs/quests.generated.json")
	if err != nil {
		t.Fatal(err)
	}
	world, err := LoadWorld("../../configs/world.generated.json")
	if err != nil {
		t.Fatal(err)
	}
	scenes, issues := TownArrivalSceneWhitelist(quests, world)
	if len(scenes) != 33 {
		t.Fatalf("town arrival scenes = %d, want 33; issues = %v", len(scenes), issues)
	}
	if len(issues) != 1 || issues[0] != "quest 22881: town area 195/67 absent from world catalog" {
		t.Fatalf("unexpected unresolved town arrival rules: %v", issues)
	}
	westCoast, ok := scenes[100004404]
	if !ok || westCoast.QuestID != 12152 || westCoast.Town != 40 || westCoast.Area != 0 {
		t.Fatalf("West Coast quest scene = %+v, present = %t", westCoast, ok)
	}
}
