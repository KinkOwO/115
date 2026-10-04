package workflow

import (
	"context"
	"crypto/sha256"
	"dfolan/internal/catalog"
	"dfolan/internal/database"
	"dfolan/internal/game/protocol"
	"dfolan/internal/inventory"
	"encoding/json"
	"fmt"
)

// Package workflow owns cross-domain orchestration that spans more than one
// domain and the persistence transaction (contract §5). Domains expose pure
// state and rules; this layer holds the store handle and sequences the calls so
// that domains do not import each other or internal/database.
// MoveStackReceipt records one completed quick-use-belt stack move.
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
// drag the client retries moves the stack once. The bag transform itself stays
// in inventory; this workflow owns the transaction and receipt.
func MoveStack(ctx context.Context, store *database.Store, role database.Character, c catalog.LootCatalog, rules inventory.BagRules,
	r protocol.ItemMoveRequest, key string) (database.Character, MoveStackReceipt, bool, error) {
	var out MoveStackReceipt
	fail := func(e error) (database.Character, MoveStackReceipt, bool, error) {
		return role, out, false, e
	}
	if store == nil {
		return fail(fmt.Errorf("stack move store missing"))
	}
	if role.ConfigVersion != c.Source.SaveIdentity() {
		return fail(fmt.Errorf("stack move source mismatch"))
	}
	if r.SourceList != 0 || r.DestinationList != 0 {
		return fail(fmt.Errorf("stack moves stay inside the ordinary bag"))
	}
	request, e := json.Marshal(r)
	if e != nil {
		return fail(e)
	}
	eventModel := fmt.Sprintf("bag-move-v2:%x", sha256.Sum256(request))
	saved, applied, e := store.CommitCharacterEvent(ctx, role.AccountID, role.ID,
		c.Source.SaveIdentity(), key, eventModel,
		func(current database.Character) (json.RawMessage, json.RawMessage, error) {
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
			b, e = b.MoveStackRequest(c, rules, r)
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
			out = MoveStackReceipt{from, to, template, c.Source.SaveIdentity()}
			receipt, e := json.Marshal(out)
			return updated, receipt, e
		})
	if e != nil {
		return fail(e)
	}
	saved.WireID = role.WireID
	return saved, out, applied, nil
}

// SortBag adopts one CMD20 arrangement into the stored bag under the owning
// character's lock. The receipt key makes a replayed frame a no-op instead of
// applying the same permutation twice. The permutation itself stays in
// inventory (BagRules/SortItems); this workflow owns the transaction.
func SortBag(ctx context.Context, store *database.Store, role database.Character, rules inventory.BagRules, key string, r protocol.SortItemRequest) (database.Character, bool, error) {
	if store == nil {
		return role, false, fmt.Errorf("wear storage unavailable")
	}
	saved, applied, e := store.CommitCharacterEvent(ctx, role.AccountID, role.ID, role.ConfigVersion, key, "ordinary-item-sort-v1", func(current database.Character) (json.RawMessage, json.RawMessage, error) {
		b, e := inventory.ReadBag(current.State)
		if e != nil {
			return nil, nil, e
		}
		b, e = inventory.SortItems(b, rules, r)
		if e != nil {
			return nil, nil, e
		}
		raw, e := inventory.SaveBag(current.State, b)
		if e != nil {
			return nil, nil, e
		}
		receipt, e := json.Marshal(map[string]any{"list": r.List, "slots": len(r.Slots)})
		return raw, receipt, e
	})
	saved.WireID = role.WireID
	return saved, applied, e
}
