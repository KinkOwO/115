package main

import (
	"context"
	"crypto/sha256"
	"dfolan/internal/database"
	"dfolan/internal/game/protocol"
	"dfolan/internal/inventory"
	"encoding/json"
	"fmt"
	"time"
)

// deleteItems performs the general CMD18 inventory deletion (discard) that the
// skill-material branch deliberately refuses: it works in town, accepts any
// main-bag row the client sends, removes the requested amount from the
// authoritative bag inside the character transaction, and replies with the
// same 0x0012 ACK + NOTI14 row-update pair the material path uses.
func (w *worldSession) deleteItems(p, raw []byte) ([]outboundPacket, error) {
	if w == nil || w.role.ID == 0 || w.loot == nil || w.vault == nil {
		return nil, fmt.Errorf("item delete unavailable")
	}
	rows, e := protocol.DecodeDeleteItems(p)
	if e != nil {
		return nil, e
	}
	key := fmt.Sprintf("item-delete:%d:%x", w.role.AccountID, sha256.Sum256(raw))
	model := fmt.Sprintf("item-delete-v1:%x", sha256.Sum256(p))
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	saved, _, e := w.store.CommitCharacterEvent(ctx, w.role.AccountID, w.role.ID, w.role.ConfigVersion, key, model, func(role database.Character) (json.RawMessage, json.RawMessage, error) {
		bag, e := inventory.ReadBag(role.State)
		if e != nil {
			return nil, nil, e
		}
		for _, r := range rows {
			found := false
			for i := range bag.Items {
				v := &bag.Items[i]
				if v.Slot != r.Slot {
					continue
				}
				if v.Template != r.Template || v.Amount < r.Count {
					return nil, nil, fmt.Errorf("insufficient owned item")
				}
				v.Amount -= r.Count
				found = true
				break
			}
			if found {
				continue
			}
			// The equipment bag shares the list0 slot space with stackables
			// (Bag.Rows merges both), so a discard in the equipment page
			// arrives as the same listType-0 CMD18 — live frame: slot 18,
			// pants, count 1. Remove the whole instance row instead of a
			// stack decrement; equipment has no amount to subtract.
			removed := false
			for i := range bag.Equipment {
				v := &bag.Equipment[i]
				if v.Slot != r.Slot {
					continue
				}
				if v.Template != r.Template || r.Count != 1 {
					return nil, nil, fmt.Errorf("insufficient owned item")
				}
				bag.Equipment = append(bag.Equipment[:i], bag.Equipment[i+1:]...)
				removed = true
				break
			}
			if !removed {
				return nil, nil, fmt.Errorf("item slot missing")
			}
		}
		kept := bag.Items[:0]
		for _, v := range bag.Items {
			if v.Amount > 0 {
				kept = append(kept, v)
			}
		}
		bag.Items = kept
		state, e := inventory.SaveBag(role.State, bag)
		if e != nil {
			return nil, nil, e
		}
		receipt, e := json.Marshal(rows)
		return state, receipt, e
	})
	if e != nil {
		return nil, e
	}
	saved.WireID = w.role.WireID
	w.role = saved
	bag, e := inventory.ReadBag(saved.State)
	if e != nil {
		return nil, e
	}
	var delta [][protocol.CurrentItemRecordSize]byte
	for _, r := range rows {
		item := protocol.EmptyOrdinaryItem(r.Slot)
		for _, v := range bag.Items {
			if v.Slot == r.Slot {
				item = protocol.OrdinaryItem(v.Slot, v.Template, v.Amount)
			}
		}
		delta = append(delta, item)
	}
	update, e := protocol.InventoryUpdate(delta)
	if e != nil {
		return nil, e
	}
	// Same ACK + row-update pair as the material branch: the 0x0012 reply
	// releases the client reservation, the NOTI14 rows carry the committed
	// absolute slot values (zeroed rows for fully removed stacks).
	return []outboundPacket{{"item_delete_ack", 1, 18, protocol.DeleteItemsReply(rows, true)}, {"item_delete_inventory", 0, 14, update}}, nil
}
