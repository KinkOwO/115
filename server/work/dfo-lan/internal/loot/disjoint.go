package loot

import (
	"context"
	"dfolan/internal/game/protocol"
	"dfolan/internal/inventory"
	"dfolan/internal/storage"
	"encoding/json"
	"fmt"
)

type DisjointReceipt struct {
	DeletedSlots []uint16                       `json:"deleted_slots"`
	ToolSlot     uint16                         `json:"tool_slot"`
	Rewards      []protocol.DisjointRewardEntry `json:"rewards"`
	Source       string                         `json:"source"`
}

func (s *Service) Disjoint(
	ctx context.Context,
	role storage.Character,
	r protocol.DisjointItemRequest,
) (storage.Character, DisjointReceipt, bool, error) {
	var result DisjointReceipt
	fail := func(e error) (storage.Character, DisjointReceipt, bool, error) {
		return role, result, false, e
	}
	if role.ConfigVersion != s.Catalog.Source.Checksum {
		return fail(fmt.Errorf("inventory source mismatch"))
	}
	if len(r.Items) == 0 {
		return fail(fmt.Errorf("empty disjoint request"))
	}

	slots := make([]uint16, len(r.Items))
	for i, item := range r.Items {
		slots[i] = item.Slot
	}

	key := fmt.Sprintf("disjoint:%d:%d:%d", r.ToolSlot, r.Items[0].Slot, r.Items[0].Template)
	saved, applied, e := s.Store.CommitCharacterEvent(ctx, role.AccountID, role.ID,
		s.Catalog.Source.Checksum, key, s.Rules.Model,
		func(current storage.Character) (json.RawMessage, json.RawMessage, error) {
			b, e := inventory.ReadBag(current.State)
			if e != nil {
				return nil, nil, e
			}
			b, res, e := b.Disjoint(s.Catalog, s.BagRules, s.Equipment, slots, r.ToolSlot)
			if e != nil {
				return nil, nil, e
			}
			updated, e := inventory.SaveBag(current.State, b)
			if e != nil {
				return nil, nil, e
			}
			var rewards []protocol.DisjointRewardEntry
			for _, rw := range res.Rewards {
				rewards = append(rewards, protocol.DisjointRewardEntry{
					Slot:     rw.Slot,
					Template: rw.Template,
					Count:    rw.Count,
				})
			}
			result = DisjointReceipt{
				DeletedSlots: res.DeletedSlots,
				ToolSlot:     res.ToolSlot,
				Rewards:      rewards,
				Source:       s.Catalog.Source.Checksum,
			}
			receipt, e := json.Marshal(result)
			return updated, receipt, e
		})
	if e != nil {
		return fail(e)
	}
	receipt, e := s.Store.CharacterEventReceipt(ctx, role.AccountID, role.ID, key)
	if e != nil {
		return fail(e)
	}
	if e = json.Unmarshal(receipt, &result); e != nil {
		return fail(e)
	}
	if result.Source != s.Catalog.Source.Checksum || len(result.DeletedSlots) != len(slots) {
		return fail(fmt.Errorf("disjoint receipt conflict"))
	}
	saved.WireID = role.WireID
	return saved, result, applied, nil
}
