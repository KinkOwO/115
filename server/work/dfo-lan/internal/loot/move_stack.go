package loot

import (
	"context"
	"crypto/sha256"
	"dfolan/internal/game/protocol"
	"dfolan/internal/inventory"
	"dfolan/internal/storage"
	"encoding/json"
	"fmt"
)

type MoveStackReceipt struct {
	From     uint16 `json:"from"`
	To       uint16 `json:"to"`
	Template uint32 `json:"template"`
	Source   string `json:"source"`
}

// MoveStack settles one stack moving between bag slots - the client's way of
// filling the quick-use belt. Live capture 20260912T004320 shows CMD19 with
// both list fields 0, moving the 6003 stack out of slot 65 into slot 3; the
// equipment path refused it ("slot outside equipment bag") and the client
// then showed its generic "target inventory is full" notice, which is why the
// belt looked broken rather than unimplemented.
//
// Like every other bag write it goes through the character event log, so a
// drag the client retries moves the stack once.
func (s *Service) MoveStack(ctx context.Context, role storage.Character, rules inventory.BagRules,
	r protocol.ItemMoveRequest, key string) (storage.Character, MoveStackReceipt, bool, error) {
	var out MoveStackReceipt
	fail := func(e error) (storage.Character, MoveStackReceipt, bool, error) {
		return role, out, false, e
	}
	if role.ConfigVersion != s.Catalog.Source.Checksum {
		return fail(fmt.Errorf("stack move source mismatch"))
	}
	if r.SourceList != 0 || r.DestinationList != 0 {
		return fail(fmt.Errorf("stack moves stay inside the ordinary bag"))
	}
	request, e := json.Marshal(r)
	if e != nil {
		return fail(e)
	}
	model := fmt.Sprintf("bag-move-v2:%x", sha256.Sum256(request))
	saved, applied, e := s.Store.CommitCharacterEvent(ctx, role.AccountID, role.ID,
		s.Catalog.Source.Checksum, key, model,
		func(current storage.Character) (json.RawMessage, json.RawMessage, error) {
			b, e := inventory.ReadBag(current.State)
			if e != nil {
				return nil, nil, e
			}
			// The client names the item it is dragging in the destination
			// fields and leaves the source ones clear, so the destination
			// slot is where the stack currently is.
			from, to, template := r.DestinationSlot, r.SourceSlot, r.DestinationItem
			if r.Count != 0 {
				from, to, template = r.SourceSlot, r.DestinationSlot, r.SourceItem
			}
			b, e = b.MoveStackRequest(s.Catalog, rules, r)
			if e != nil {
				return nil, nil, e
			}
			updated, e := inventory.SaveBag(current.State, b)
			if e != nil {
				return nil, nil, e
			}
			if _, e = protocol.InventoryRestore(b.Rows(), b.Expansion); e != nil {
				return nil, nil, e
			}
			out = MoveStackReceipt{from, to, template, s.Catalog.Source.Checksum}
			receipt, e := json.Marshal(out)
			return updated, receipt, e
		})
	if e != nil {
		return fail(e)
	}
	saved.WireID = role.WireID
	return saved, out, applied, nil
}
