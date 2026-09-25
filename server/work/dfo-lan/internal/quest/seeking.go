package quest

import (
	"context"
	"dfolan/internal/dungeon"
	"dfolan/internal/inventory"
	"dfolan/internal/storage"
	"encoding/json"
	"fmt"
)

type seekingItemGrant struct{ Template, Amount uint32 }

type SeekingGrantResult struct {
	Role     storage.Character
	Items    []inventory.AwardReceipt
	Applied  bool
	Advanced bool
}

type seekingGrantReceipt struct {
	Run    string                   `json:"run"`
	Entity uint16                   `json:"entity"`
	Source string                   `json:"source"`
	Items  []inventory.AwardReceipt `json:"items"`
}

// seekingMonsterItemGrants resolves quest items for a confirmed, owned death.
// Only the quest selected for this source maze can pay.
func (s *Service) seekingMonsterItemGrants(ctx context.Context, role storage.Character, run *dungeon.Session, entity uint16) ([]seekingItemGrant, error) {
	if run == nil || !run.Loaded || run.Maze.Quest == 0 || !run.Dead[entity] || run.Unowned[entity] {
		return nil, nil
	}
	en := s.Index().Entries[uint32(run.Maze.Quest)]
	if en == nil || !en.Implemented || en.Model != SeekingItems || en.Seeking.Dungeon != run.Definition.ID {
		return nil, nil
	}
	states, err := s.Store.Quests(ctx, role.AccountID, role.ID)
	if err != nil {
		return nil, err
	}
	accepted := false
	for _, q := range states {
		if q.ID == run.Maze.Quest && q.Status == "accepted" && q.Progress > 0 && q.ConfigVersion == s.Catalog.Source.Checksum && q.ProgressModel == en.Model {
			accepted = true
			break
		}
	}
	if !accepted {
		return nil, nil
	}
	var monster uint32
	for _, row := range run.Monsters {
		if row.Entity == entity {
			monster = row.Template
			break
		}
	}
	if monster == 0 {
		return nil, fmt.Errorf("quest drop target outside current room")
	}
	bag, err := inventory.ReadBag(role.State)
	if err != nil {
		return nil, err
	}
	have := itemCounts(bag)
	need := map[uint32]uint32{}
	for _, row := range en.Seeking.Items {
		need[row.Template] = row.Amount
	}
	var out []seekingItemGrant
	for _, row := range en.Seeking.Rewards {
		if row.Monster != monster || have[row.Item] >= need[row.Item] {
			continue
		}
		amount := row.Amount
		if missing := need[row.Item] - have[row.Item]; amount > missing {
			amount = missing
		}
		if amount > 0 {
			out = append(out, seekingItemGrant{row.Item, amount})
			have[row.Item] += amount
		}
	}
	return out, nil
}

// GrantSeekingMonsterItems writes source [monster reward item] awards directly
// into the bag. The event key binds the grant to one run and monster entity,
// so a replayed death can never duplicate the invisible quest item.
func (s *Service) GrantSeekingMonsterItems(ctx context.Context, role storage.Character, run *dungeon.Session, entity uint16) (SeekingGrantResult, error) {
	out := SeekingGrantResult{Role: role}
	awards, err := s.seekingMonsterItemGrants(ctx, role, run, entity)
	if err != nil {
		return out, err
	}
	if len(awards) == 0 {
		advanced, e := s.InventoryProgress(ctx, role)
		out.Advanced = len(advanced) > 0
		return out, e
	}
	if s.Inventory == nil {
		return out, ErrRewardPending
	}
	key := fmt.Sprintf("quest-monster-item:%s:%d", run.RunID, entity)
	saved, applied, err := s.Store.CommitCharacterEvent(ctx, role.AccountID, role.ID, s.Catalog.Source.Checksum, key, SeekingItems, func(current storage.Character) (json.RawMessage, json.RawMessage, error) {
		var items []inventory.AwardReceipt
		for _, award := range awards {
			var receipt inventory.AwardReceipt
			current.State, receipt, err = s.Inventory.Grant(current.State, award.Template, award.Amount)
			if err != nil {
				return nil, nil, err
			}
			items = append(items, receipt)
		}
		receipt, e := json.Marshal(seekingGrantReceipt{Run: run.RunID, Entity: entity, Source: s.Catalog.Source.Checksum, Items: items})
		return current.State, receipt, e
	})
	if err != nil {
		return out, err
	}
	receiptJSON, err := s.Store.CharacterEventReceipt(ctx, role.AccountID, role.ID, key)
	if err != nil {
		return out, err
	}
	var receipt seekingGrantReceipt
	if err = json.Unmarshal(receiptJSON, &receipt); err != nil {
		return out, err
	}
	if receipt.Run != run.RunID || receipt.Entity != entity || receipt.Source != s.Catalog.Source.Checksum {
		return out, fmt.Errorf("quest monster item receipt mismatch")
	}
	saved.WireID = role.WireID
	out.Role, out.Items, out.Applied = saved, receipt.Items, applied
	advanced, err := s.InventoryProgress(ctx, saved)
	out.Advanced = len(advanced) > 0
	return out, err
}

// InventoryProgress completes accepted seeking objectives after a committed
// pickup makes every required item present in the bag.
func (s *Service) InventoryProgress(ctx context.Context, role storage.Character) ([]uint16, error) {
	states, err := s.Store.Quests(ctx, role.AccountID, role.ID)
	if err != nil {
		return nil, err
	}
	bag, err := inventory.ReadBag(role.State)
	if err != nil {
		return nil, err
	}
	x := s.Index()
	var advanced []uint16
	for _, q := range states {
		en := x.Entries[uint32(q.ID)]
		if q.Status != "accepted" || q.Progress == 0 || q.ConfigVersion != x.Source ||
			en == nil || en.Model != SeekingItems || q.ProgressModel != en.Model || !holdsSeekingItems(bag, en.Seeking.Items) {
			continue
		}
		applied, e := s.Store.CompleteQuestObjective(ctx, role.AccountID, role.ID, q.ID, x.Source, en.Model)
		if e != nil {
			return advanced, e
		}
		if applied {
			advanced = append(advanced, q.ID)
		}
	}
	return advanced, nil
}

func itemCounts(b inventory.Bag) map[uint32]uint32 {
	out := map[uint32]uint32{}
	for _, row := range b.Items {
		out[row.Template] += row.Amount
	}
	return out
}

func holdsSeekingItems(b inventory.Bag, need []ItemNeed) bool {
	if len(need) == 0 {
		return false
	}
	have := itemCounts(b)
	for _, row := range need {
		if have[row.Template] < row.Amount {
			return false
		}
	}
	return true
}

// consumeSeekingItems removes the objective items from ordinary stackable
// rows. It runs inside CommitQuestReward, so removal, rewards and quest
// completion either all commit or all roll back.
func consumeSeekingItems(raw json.RawMessage, need []ItemNeed) (json.RawMessage, []inventory.AwardReceipt, error) {
	bag, err := inventory.ReadBag(raw)
	if err != nil {
		return nil, nil, err
	}
	if !holdsSeekingItems(bag, need) {
		return nil, nil, ErrObjectiveIncomplete
	}
	var receipts []inventory.AwardReceipt
	for _, n := range need {
		left := n.Amount
		var slots []uint16
		for i := 0; i < len(bag.Items) && left > 0; {
			row := &bag.Items[i]
			if row.Template != n.Template {
				i++
				continue
			}
			take := row.Amount
			if take > left {
				take = left
			}
			row.Amount -= take
			left -= take
			slots = append(slots, row.Slot)
			if row.Amount == 0 {
				bag.Items = append(bag.Items[:i], bag.Items[i+1:]...)
			} else {
				i++
			}
		}
		if left != 0 {
			return nil, nil, ErrObjectiveIncomplete
		}
		receipts = append(receipts, inventory.AwardReceipt{Template: n.Template, Amount: n.Amount, Slots: slots})
	}
	out, err := inventory.SaveBag(raw, bag)
	return out, receipts, err
}
