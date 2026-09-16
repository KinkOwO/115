package loot

import (
	"context"
	"dfolan/internal/game/protocol"
	"dfolan/internal/inventory"
	"dfolan/internal/storage"
	"encoding/json"
	"fmt"
)

type ConsumeReceipt struct {
	Slot      uint16 `json:"slot"`
	Template  uint32 `json:"template"`
	Remaining uint32 `json:"remaining"`
	Source    string `json:"source"`
}

// Consume settles one stackable use durably. The decrement and its receipt
// commit in the same character transaction the pickup path uses, so a replayed
// or retried request returns the original receipt instead of spending a second
// unit — the native client sends this on a hotkey press, which is exactly the
// kind of input that repeats.
//
// The recovery effect is the client's own: the native success acknowledgement
// carries no restored amount, so nothing is invented here.
func (s *Service) Consume(ctx context.Context, role storage.Character, r protocol.UseStackableRequest) (storage.Character, ConsumeReceipt, bool, error) {
	var out ConsumeReceipt
	fail := func(e error) (storage.Character, ConsumeReceipt, bool, error) {
		return role, out, false, e
	}
	if role.ConfigVersion != s.Catalog.Source.Checksum {
		return fail(fmt.Errorf("consume source mismatch"))
	}
	if r.List != 0 {
		// list0 is the ordinary bag. Other containers (the shared temporary
		// inventories) have their own unverified semantics.
		return fail(fmt.Errorf("unsupported source container %d", r.List))
	}
	key := fmt.Sprintf("consume:%d:%d:%d", r.Slot, r.Template, r.Instance)
	saved, applied, e := s.Store.CommitCharacterEvent(ctx, role.AccountID, role.ID,
		s.Catalog.Source.Checksum, key, s.Rules.Model,
		func(current storage.Character) (json.RawMessage, json.RawMessage, error) {
			b, e := inventory.ReadBag(current.State)
			if e != nil {
				return nil, nil, e
			}
			b, remaining, e := b.Consume(s.Catalog, r.Slot, r.Template)
			if e != nil {
				return nil, nil, e
			}
			updated, e := inventory.SaveBag(current.State, b)
			if e != nil {
				return nil, nil, e
			}
			out = ConsumeReceipt{r.Slot, r.Template, remaining, s.Catalog.Source.Checksum}
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
	if out.Source != s.Catalog.Source.Checksum || out.Template != r.Template || out.Slot != r.Slot {
		return fail(fmt.Errorf("consume receipt conflict"))
	}
	saved.WireID = role.WireID
	return saved, out, applied, nil
}
