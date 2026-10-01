package main

import (
	"context"
	"dfolan/internal/game/protocol"
	"dfolan/internal/inventory"
	"dfolan/internal/storage"
	"dfolan/internal/workflow"
	"encoding/json"
	"time"
)

func (w *worldSession) movePetStack(rules inventory.BagRules, r protocol.ItemMoveRequest, key string) ([]outboundPacket, bool, error) {
	if w == nil || w.role.ID == 0 || w.loot == nil {
		return nil, false, nil
	}
	bag, err := inventory.ReadBag(w.role.State)
	if err != nil {
		return nil, false, err
	}
	if !inventory.IsPetContainerMove(bag, r) {
		return nil, false, nil
	}
	catalog := w.loot.Catalog
	if w.vault != nil {
		catalog = w.vault.Catalog
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	saved, applied, err := w.loot.Store.CommitCharacterEvent(ctx, w.role.AccountID, w.role.ID, w.role.ConfigVersion, key, "pet-move-v1",
		func(current storage.Character) (json.RawMessage, json.RawMessage, error) {
			currentBag, e := inventory.ReadBag(current.State)
			if e != nil {
				return nil, nil, e
			}
			moved, e := currentBag.MovePetStackRequest(catalog, rules, r)
			if e != nil {
				return nil, nil, e
			}
			state, e := inventory.SaveBag(current.State, moved)
			if e != nil {
				return nil, nil, e
			}
			if _, e = inventory.PetContainerBody(moved, true); e != nil {
				return nil, nil, e
			}
			return state, json.RawMessage(`{}`), nil
		})
	if err != nil {
		return nil, true, err
	}
	saved.WireID = w.role.WireID
	w.role = saved
	bag, err = inventory.ReadBag(saved.State)
	if err != nil {
		return nil, true, err
	}
	bagBody, err := protocol.InventoryRestore(bag.Rows(), bag.Expansion)
	if err != nil {
		return nil, true, err
	}
	petBody, err := inventory.PetContainerBody(bag, true)
	if err != nil {
		return nil, true, err
	}
	packets := []outboundPacket{}
	if applied {
		packets = append(packets, outboundPacket{"pet_stack_move_committed", 1, 19, protocol.ItemMoveSuccess(r, max(1, r.Count))})
	}
	return append(packets, outboundPacket{"pet_stack_bag_restored", 0, 13, bagBody}, outboundPacket{"pet_stack_container_restored", 0, 13, petBody}), true, nil
}

// moveStack answers the CMD19 moves that carry a stack rather than a piece of
// equipment: dragging a consumable onto the quick-use belt and back. It
// reports whether it recognised the request, so the equipment path keeps
// every move it already handles.
func (w *worldSession) moveStack(rules inventory.BagRules,
	r protocol.ItemMoveRequest, key string) ([]outboundPacket, bool, error) {
	if w == nil || w.role.ID == 0 || w.loot == nil {
		return nil, false, nil
	}
	if r.SourceList != 0 || r.DestinationList != 0 {
		return nil, false, nil
	}
	b, e := inventory.ReadBag(w.role.State)
	if e != nil {
		return nil, false, nil
	}
	// The dragged stack sits in the slot the client names as the destination;
	// the source fields carry the belt slot it is going to.
	carries := false
	for _, v := range b.Items {
		if v.Slot == r.DestinationSlot || v.Slot == r.SourceSlot {
			carries = true
		}
	}
	if !carries {
		return nil, false, nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	bagCatalog := w.loot.Catalog
	if w.vault != nil {
		bagCatalog = w.vault.Catalog
	}
	if w.loot.Currency != nil {
		rules = w.loot.Currency.BagRules(rules)
	}
	// The candidate carries only the source-backed creation box in this
	// extra category; do not widen the monster drop catalog to move it.
	if bagCatalog.Items[10417789].StackableType == "[booster selection]" {
		slots := make(map[string][2]uint16, len(rules.Slots)+1)
		for k, v := range rules.Slots {
			slots[k] = v
		}
		slots["[booster selection]"] = [2]uint16{65, 120}
		if w.progression != nil && w.progression.Odyssey != nil {
			slots["[booster]"] = [2]uint16{65, 120}
		}
		rules.Slots = slots
	}
	saved, _, applied, e := workflow.MoveStack(ctx, w.loot.Store, w.role, bagCatalog, rules, r, key)
	if e != nil {
		return nil, true, e
	}
	plan, e := stackMovePackets(saved, r, applied)
	if e != nil {
		return nil, true, e
	}
	w.role = saved
	return plan, true, nil
}

func stackMovePackets(saved storage.Character, r protocol.ItemMoveRequest, applied bool) ([]outboundPacket, error) {
	updated, e := inventory.ReadBag(saved.State)
	if e != nil {
		return nil, e
	}
	update, e := protocol.InventoryRestore(updated.Rows(), updated.Expansion)
	if e != nil {
		return nil, e
	}
	plan := []outboundPacket{}
	if applied {
		count := r.Count
		if count == 0 {
			count = 1
		}
		plan = append(plan, outboundPacket{"stack_move_committed", 1, 19, protocol.ItemMoveSuccess(r, count)})
	}
	// NOTI14 is an acquisition/update path: it leaves absent source rows and
	// emits obtained-item effects. NOTI13 replaces the authoritative bag.
	return append(plan, outboundPacket{"stack_move_inventory_restored", 0, 13, update}), nil
}
