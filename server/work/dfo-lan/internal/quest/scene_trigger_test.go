package quest

import (
	"dfolan/internal/catalog"
	"dfolan/internal/catalog/pvf"
	"dfolan/internal/dungeon"
	"testing"
)

func sceneQuest() catalog.QuestDefinition {
	return catalog.QuestDefinition{ID: 3191, Kind: "[clear map]", ObjectiveCells: []pvf.Token{{Type: 0, Value: 100008683}}}
}

func TestSceneClearObjectiveAppliesOnFinalLayer(t *testing.T) {
	run := &dungeon.Session{Loaded: true, Room: catalog.DungeonRoom{Map: 100008683, Boss: true}}
	objective, applies, err := SceneClearObjective(sceneQuest(), run)
	if err != nil || !applies || objective != 100008683 {
		t.Fatalf("final layer trigger rejected: %d %t %v", objective, applies, err)
	}
}

func TestSceneClearObjectiveSilentForOtherKinds(t *testing.T) {
	run := &dungeon.Session{Loaded: true, Room: catalog.DungeonRoom{Map: 100008683}}
	for _, d := range []catalog.QuestDefinition{
		{ID: 1, Kind: "[meet npc]", ObjectiveCells: []pvf.Token{{Type: 0, Value: 12}}},
		{ID: 2, Kind: "[clear map]", ObjectiveCells: []pvf.Token{{Type: 0, Value: 100008683}, {Type: 0, Value: 100008684}}},
		{ID: 3, Kind: "[clear map]", Pending: []string{"unresolved"}, ObjectiveCells: []pvf.Token{{Type: 0, Value: 100008683}}},
		{ID: 4, Kind: "[look cinematic]", ObjectiveCells: []pvf.Token{{Type: 0, Value: 14898}}},
	} {
		if _, applies, err := SceneClearObjective(d, run); applies || err != nil {
			t.Fatalf("quest %d must stay on the silent path: %t %v", d.ID, applies, err)
		}
	}
}

func TestSceneClearObjectiveRequiresOwnedLoadedRunOnObjectiveMap(t *testing.T) {
	d := sceneQuest()
	if _, _, err := SceneClearObjective(d, nil); err == nil {
		t.Fatal("trigger without a run accepted")
	}
	if _, _, err := SceneClearObjective(d, &dungeon.Session{Room: catalog.DungeonRoom{Map: 100008683}}); err == nil {
		t.Fatal("trigger on an unloaded run accepted")
	}
	run := &dungeon.Session{Loaded: true, Room: catalog.DungeonRoom{Map: 100008684}}
	if _, _, err := SceneClearObjective(d, run); err == nil {
		t.Fatal("trigger from an earlier layer map accepted")
	}
}
