package loot

import (
	"context"
	"dfolan/internal/game/protocol"
	"dfolan/internal/inventory"
	"dfolan/internal/storage"
	"encoding/json"
	"fmt"
	"sync/atomic"
)

var shopEventSeq uint64

const shopUnitPrice = 1

type BuyReceipt struct {
	NpcID    uint32 `json:"npc_id"`
	Template uint32 `json:"template"`
	Count    uint32 `json:"count"`
	Slot     uint16 `json:"slot"`
	Cost     uint32 `json:"cost"`
	NewGold  uint32 `json:"new_gold"`
	Source   string `json:"source"`
	Seq      uint64 `json:"seq"`
}

type SellReceipt struct {
	NpcID      uint32 `json:"npc_id"`
	Slot       uint16 `json:"slot"`
	Template   uint32 `json:"template"`
	GoldGained uint32 `json:"gold_gained"`
	NewGold    uint32 `json:"new_gold"`
	Source     string `json:"source"`
	Seq        uint64 `json:"seq"`
}

// Buy processes an NPC shop purchase transaction durably.
// Uses a process-level monotonic sequence to prevent idempotency key collision
// during rapid burst purchases.
func (s *Service) Buy(ctx context.Context, role storage.Character, r protocol.BuyItemRequest) (storage.Character, BuyReceipt, bool, error) {
	var out BuyReceipt
	fail := func(e error) (storage.Character, BuyReceipt, bool, error) {
		return role, out, false, e
	}
	if role.ConfigVersion != s.Catalog.Source.Checksum {
		return fail(fmt.Errorf("buy source mismatch"))
	}
	seq := atomic.AddUint64(&shopEventSeq, 1)
	key := fmt.Sprintf("buy:%d:%d:%d", r.Template, r.Count, seq)
	cost := r.Count * shopUnitPrice

	var stackableType string
	if item, ok := s.Catalog.Items[r.Template]; ok {
		stackableType = item.StackableType
	}

	saved, applied, e := s.Store.CommitCharacterEvent(ctx, role.AccountID, role.ID,
		s.Catalog.Source.Checksum, key, s.Rules.Model,
		func(current storage.Character) (json.RawMessage, json.RawMessage, error) {
			b, e := inventory.ReadBag(current.State)
			if e != nil {
				return nil, nil, e
			}
			b, slot, e := b.Buy(s.BagRules, r.Template, r.Count, cost, stackableType)
			if e != nil {
				return nil, nil, e
			}
			updated, e := inventory.SaveBag(current.State, b)
			if e != nil {
				return nil, nil, e
			}
			out = BuyReceipt{
				NpcID:    r.NpcID,
				Template: r.Template,
				Count:    r.Count,
				Slot:     slot,
				Cost:     cost,
				NewGold:  b.Gold,
				Source:   s.Catalog.Source.Checksum,
				Seq:      seq,
			}
			receipt, e := json.Marshal(out)
			return updated, receipt, e
		})
	if e != nil {
		return fail(e)
	}
	receipt, e := s.Store.CharacterEventReceipt(ctx, role.AccountID, role.ID, key)
	if e != nil {
		return fail(e)
	}
	if e = json.Unmarshal(receipt, &out); e != nil {
		return fail(e)
	}
	if out.Source != s.Catalog.Source.Checksum || out.Template != r.Template || out.Count != r.Count {
		return fail(fmt.Errorf("buy receipt conflict"))
	}
	saved.WireID = role.WireID
	return saved, out, applied, nil
}

// Sell processes an NPC shop item sale transaction durably.
func (s *Service) Sell(ctx context.Context, role storage.Character, r protocol.SellItemRequest) (storage.Character, SellReceipt, bool, error) {
	var out SellReceipt
	fail := func(e error) (storage.Character, SellReceipt, bool, error) {
		return role, out, false, e
	}
	if role.ConfigVersion != s.Catalog.Source.Checksum {
		return fail(fmt.Errorf("sell source mismatch"))
	}
	seq := atomic.AddUint64(&shopEventSeq, 1)
	key := fmt.Sprintf("sell:%d:%d:%d", r.List, r.Slot, seq)

	saved, applied, e := s.Store.CommitCharacterEvent(ctx, role.AccountID, role.ID,
		s.Catalog.Source.Checksum, key, s.Rules.Model,
		func(current storage.Character) (json.RawMessage, json.RawMessage, error) {
			b, e := inventory.ReadBag(current.State)
			if e != nil {
				return nil, nil, e
			}
			b, template, goldGained, e := b.Sell(s.BagRules, r.List, r.Slot, shopUnitPrice)
			if e != nil {
				return nil, nil, e
			}
			updated, e := inventory.SaveBag(current.State, b)
			if e != nil {
				return nil, nil, e
			}
			out = SellReceipt{
				NpcID:      r.NpcID,
				Slot:       r.Slot,
				Template:   template,
				GoldGained: goldGained,
				NewGold:    b.Gold,
				Source:     s.Catalog.Source.Checksum,
				Seq:        seq,
			}
			receipt, e := json.Marshal(out)
			return updated, receipt, e
		})
	if e != nil {
		return fail(e)
	}
	receipt, e := s.Store.CharacterEventReceipt(ctx, role.AccountID, role.ID, key)
	if e != nil {
		return fail(e)
	}
	if e = json.Unmarshal(receipt, &out); e != nil {
		return fail(e)
	}
	if out.Source != s.Catalog.Source.Checksum || out.Slot != r.Slot {
		return fail(fmt.Errorf("sell receipt conflict"))
	}
	saved.WireID = role.WireID
	return saved, out, applied, nil
}
