package workflow

import (
	"context"
	"dfolan/internal/catalog"
	"dfolan/internal/catalog/pvf"
	"dfolan/internal/game/protocol"
	"dfolan/internal/progression"
	"dfolan/internal/quest"
	"dfolan/internal/storage"
	"encoding/json"
	"fmt"
	"time"
)

// QuestService coordinates persistence around quest domain state transitions.
type QuestService struct {
	Store *storage.Store
	Quest *quest.Service
}

func (s *QuestService) Finish(ctx context.Context, role storage.Character, r protocol.QuestSubmitRequest) (quest.FinishResult, error) {
	var out quest.FinishResult
	plan, e := s.Quest.PlanFinish(r)
	if e != nil {
		return out, e
	}
	d, model := plan.Definition, plan.Model
	commit, e := s.Store.CommitQuestReward(ctx, role.AccountID, role.ID, r.ID, s.Quest.Catalog.Source.SaveIdentity(), model, s.Quest.Progression.Rules.Model, func(current storage.Character) (json.RawMessage, json.RawMessage, error) {
		return s.Quest.PrepareFinish(current, plan, func() bool {
			if s.Store == nil {
				return false
			}
			growth, _ := s.Store.HasActivePremium(ctx, role.AccountID, storage.PremiumGrowth, time.Now())
			return growth
		}, quest.FinishRewards{
			Items: questItemRewards,
			Experience: func(d catalog.QuestDefinition, level byte) (uint32, error) {
				return progression.QuestExperience(s.Quest.Progression.Catalog, d, level)
			},
			Gold: func(d catalog.QuestDefinition, level byte) (uint32, error) {
				return progression.QuestGold(s.Quest.Progression.Catalog, d, level)
			},
		})
	})
	if e != nil {
		return out, e
	}
	if e = json.Unmarshal(commit.Receipt, &out.Receipt); e != nil {
		return out, e
	}
	if out.Receipt.Quest != r.ID || out.Receipt.Source != s.Quest.Catalog.Source.SaveIdentity() {
		return out, fmt.Errorf("quest reward receipt mismatch")
	}
	out.Role, out.Applied = commit.Character, commit.Applied
	// Self-heal a pre-fix state: a character who once accepted several
	// [collision quest] branches still carries the unchosen factions' quests.
	// Once one branch completes, accepted siblings leave the journal (their
	// maps would otherwise be cleared again); completed siblings keep their
	// rewards. Best-effort: a sibling that vanished meanwhile is not an error.
	if out.Applied && len(d.Collisions) > 0 {
		if states, e := s.Store.Quests(ctx, role.AccountID, role.ID); e == nil {
			for _, q := range states {
				if q.Status != "accepted" {
					continue
				}
				for _, c := range d.Collisions {
					if uint32(q.ID) == c {
						_ = s.Store.AbandonQuest(ctx, role.AccountID, role.ID, q.ID)
						break
					}
				}
			}
		}
	}
	return out, nil
}

func questItemRewards(cells []pvf.Token, profession, advancement byte) ([]quest.RewardItem, error) {
	items, err := progression.ItemRewards(cells, profession, advancement)
	if err != nil {
		return nil, err
	}
	var out []quest.RewardItem
	for _, item := range items {
		out = append(out, quest.RewardItem{Template: item.Template, Amount: item.Amount})
	}
	return out, nil
}
