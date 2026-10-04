package workflow

import (
	"context"
	"dfolan/internal/database"
	"dfolan/internal/dungeon"
	"dfolan/internal/quest"
	"encoding/json"
	"fmt"
)

// GrantSeekingMonsterItems writes source [monster reward item] awards directly
// into the bag. The event key binds the grant to one run and monster entity,
// so a replayed death can never duplicate the invisible quest item.
func (s *QuestService) GrantSeekingMonsterItems(ctx context.Context, role database.Character, run *dungeon.Session, entity uint16) (quest.SeekingGrantResult, error) {
	out := quest.SeekingGrantResult{Role: role}
	var awards []quest.SeekingItemGrant
	var err error
	if s.Quest.SeekingMonsterItemEligible(run, entity) {
		states, e := s.Store.Quests(ctx, role.AccountID, role.ID)
		if e != nil {
			return out, e
		}
		awards, err = s.Quest.SeekingMonsterItemGrants(role, run, entity, states)
	}
	if err != nil {
		return out, err
	}
	if len(awards) == 0 {
		advanced, e := s.InventoryProgress(ctx, role)
		out.Advanced = len(advanced) > 0
		return out, e
	}
	if s.Quest.Inventory == nil {
		return out, quest.ErrRewardPending
	}
	key := fmt.Sprintf("quest-monster-item:%s:%d", run.RunID, entity)
	saved, applied, err := s.Store.CommitCharacterEvent(ctx, role.AccountID, role.ID, s.Quest.Catalog.Source.SaveIdentity(), key, quest.SeekingItems, func(current database.Character) (json.RawMessage, json.RawMessage, error) {
		return s.Quest.PrepareSeekingGrant(current, run.RunID, entity, awards)
	})
	if err != nil {
		return out, err
	}
	receiptJSON, err := s.Store.CharacterEventReceipt(ctx, role.AccountID, role.ID, key)
	if err != nil {
		return out, err
	}
	var receipt quest.SeekingGrantReceipt
	if err = json.Unmarshal(receiptJSON, &receipt); err != nil {
		return out, err
	}
	if receipt.Run != run.RunID || receipt.Entity != entity || receipt.Source != s.Quest.Catalog.Source.SaveIdentity() {
		return out, fmt.Errorf("quest monster item receipt mismatch")
	}
	saved.WireID = role.WireID
	out.Role, out.Items, out.Applied = saved, receipt.Items, applied
	advanced, err := s.InventoryProgress(ctx, saved)
	out.Advanced = len(advanced) > 0
	return out, err
}

// InventoryProgress completes qualifying objectives after the inventory commit.
func (s *QuestService) InventoryProgress(ctx context.Context, role database.Character) ([]uint16, error) {
	states, err := s.Store.Quests(ctx, role.AccountID, role.ID)
	if err != nil {
		return nil, err
	}
	ids, err := s.Quest.InventoryProgressPlan(role, states)
	if err != nil {
		return nil, err
	}
	x := s.Quest.Index()
	var advanced []uint16
	for _, id := range ids {
		applied, e := s.Store.CompleteQuestObjective(ctx, role.AccountID, role.ID, id, x.Source, x.Entries[uint32(id)].Model)
		if e != nil {
			return advanced, e
		}
		if applied {
			advanced = append(advanced, id)
		}
	}
	return advanced, nil
}
