package quest

import (
	"dfolan/internal/catalog"
	"dfolan/internal/storage"
	"encoding/json"
	"slices"
	"testing"
)

func TestActClearPlanUsesSourceGradeLevelAndPrerequisites(t *testing.T) {
	quests, err := catalog.LoadQuests("../../configs/quests.generated.json")
	if err != nil {
		t.Fatal(err)
	}
	professions, err := catalog.LoadCharacters("../../configs/characters.alljobs-pilot.json")
	if err != nil {
		t.Fatal(err)
	}
	s := &Service{Catalog: quests, Professions: professions}
	role := storage.Character{Profession: 0, ConfigVersion: quests.Source.Checksum, State: json.RawMessage(`{"level":50,"advancement":0}`)}
	ids, err := s.ActClearPlan(role, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(ids) == 0 {
		t.Fatal("level-50 character has no source epic path")
	}
	t.Logf("level-50 source epic clear plan: %d quests", len(ids))
	for _, id := range ids {
		d := quests.Quests[uint32(id)]
		grade := cells(d.Script.Cells, "[grade]")
		if len(grade) != 1 || grade[0].Text != "[epic]" || d.MinimumLevel > 50 {
			t.Fatalf("out-of-scope quest %d", id)
		}
	}
	completed := []storage.QuestState{{ID: ids[0], Status: "completed", ConfigVersion: quests.Source.Checksum}}
	again, err := s.ActClearPlan(role, completed)
	if err != nil || slices.Contains(again, ids[0]) {
		t.Fatalf("completed quest scheduled again: %v, %v", ids[0], err)
	}
}
