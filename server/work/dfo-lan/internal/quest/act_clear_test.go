package quest

import (
	"dfolan/internal/catalog"
	"dfolan/internal/storage"
	"slices"
	"testing"
)

func TestActClearPlanOnlyIncludesAlreadyAcceptedEpicQuests(t *testing.T) {
	quests, err := catalog.LoadQuests("../../configs/quests.generated.json")
	if err != nil {
		t.Fatal(err)
	}
	s := &Service{Catalog: quests}
	role := storage.Character{ConfigVersion: quests.Source.SaveIdentity()}
	states := []storage.QuestState{
		{ID: 3330, Status: "accepted", ConfigVersion: quests.Source.SaveIdentity()},
		{ID: 3331, Status: "completed", ConfigVersion: quests.Source.SaveIdentity()},
	}
	ids, err := s.ActClearPlan(role, states)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(ids, []uint16{3330}) {
		t.Fatalf("clear plan = %v, want only accepted epic quest 3330", ids)
	}

	ids, err = s.ActClearPlan(role, nil)
	if err != nil || len(ids) != 0 {
		t.Fatalf("unaccepted quests must not be synthesized: ids=%v err=%v", ids, err)
	}

	stale := []storage.QuestState{{ID: 3330, Status: "accepted", ConfigVersion: "stale"}}
	if _, err = s.ActClearPlan(role, stale); err == nil {
		t.Fatal("accepted quest with stale source version must be refused")
	}
}
