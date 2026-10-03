package quest

import (
	"dfolan/internal/catalog"
	"dfolan/internal/catalog/pvf"
	"dfolan/internal/character"
	"dfolan/internal/testfixture"
	"encoding/json"
	"testing"
)

// Premium eligibility must stay after reward/base EXP validation. Moving it
// ahead of those gates adds database reads for rejected submissions.
func TestPrepareFinishPremiumLookupOrder(t *testing.T) {
	catalogXP, err := catalog.LoadProgression(testfixture.ProgressionPath(t))
	if err != nil {
		t.Fatal(err)
	}
	quests, err := catalog.LoadQuests(testfixture.CatalogPath(t, "quests"))
	if err != nil {
		t.Fatal(err)
	}
	definition := quests.Quests[3145]
	definition.RewardCells = nil
	plan := FinishPlan{Definition: definition, Model: "test"}
	s := &Service{Progression: &character.ProgressionService{Catalog: catalogXP}}
	rewards := FinishRewards{
		Items: func(cells []pvf.Token, p, g byte) ([]RewardItem, error) {
			awards, e := character.GrowthItemRewards(cells, p, g)
			var out []RewardItem
			for _, a := range awards {
				out = append(out, RewardItem{a.Template, a.Amount})
			}
			return out, e
		},
		Experience: func(d catalog.QuestDefinition, l byte) (uint32, error) {
			return character.GrowthQuestExperience(s.Progression.Catalog, d, l)
		},
		Gold: func(d catalog.QuestDefinition, l byte) (uint32, error) {
			return character.GrowthQuestGold(s.Progression.Catalog, d, l)
		},
	}

	calls := 0
	growth := func() bool { calls++; return false }
	if _, _, err := s.PrepareFinish(character.Character{State: json.RawMessage(`{`)}, plan, growth, rewards); err == nil || calls != 0 {
		t.Fatalf("invalid state queried premium: calls=%d err=%v", calls, err)
	}
	role := character.Character{State: json.RawMessage(`{"level":1}`)}
	goodCatalog := s.Progression.Catalog
	s.Progression.Catalog = catalog.Progression{}
	if _, _, err := s.PrepareFinish(role, plan, growth, rewards); err == nil || calls != 0 {
		t.Fatalf("invalid EXP source queried premium: calls=%d err=%v", calls, err)
	}
	s.Progression.Catalog = goodCatalog
	// Progression application may reject this incomplete fixture later; the
	// already-validated base EXP still performs exactly one premium lookup.
	_, _, _ = s.PrepareFinish(role, plan, growth, rewards)
	if calls != 1 {
		t.Fatalf("premium lookup count=%d, want 1", calls)
	}
}
