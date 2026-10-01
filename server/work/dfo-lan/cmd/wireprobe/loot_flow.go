package main

import (
	"context"
	"dfolan/internal/game/protocol"
	"dfolan/internal/inventory"
	"dfolan/internal/workflow"
	"fmt"
	"time"
)

func (w *worldSession) pickup(p []byte) ([]outboundPacket, error) {
	if w.loot == nil || w.activeDungeon == nil {
		return nil, fmt.Errorf("pickup service/run unavailable")
	}
	r, e := protocol.DecodePickup(p)
	if e != nil {
		return nil, e
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	saved, receipt, applied, e := (&workflow.LootService{Store: w.store, Loot: w.loot}).Pickup(ctx, w.role, w.drops, w.activeDungeon, r)
	if e != nil {
		return nil, e
	}
	// Account-shared materials (colored cube fragments, souls, old souls)
	// never stay in the bag: sweep the freshly picked stack into the account
	// storage and republish the list35+list0 snapshots instead of NOTI14.
	_, isAccountMaterial := inventory.AccountMaterialSlot(receipt.Award.Template)
	var materials inventory.AccountMaterials
	if isAccountMaterial {
		saved, materials, e = sweepAccountMaterials(ctx, w.store, saved)
		if e != nil {
			return nil, e
		}
		receipt.Destination = storageDestinationSlot(receipt.Award.Template, receipt.Destination)
	}
	b, e := inventory.ReadBag(saved.State)
	if e != nil {
		return nil, e
	}
	isPetConsumable := false
	for _, item := range b.PetItems {
		if item.Slot == receipt.Destination && item.Template == receipt.Award.Template {
			isPetConsumable = true
			break
		}
	}
	// NOTI14 is an incremental slot update: publish only the pickup
	// destination row so the client marks just that slot as newly obtained.
	// A full-bag update makes every slot flash the new-item highlight on every
	// pickup, which the client shows as a highlight on all items.
	var update []byte
	if !isAccountMaterial && !isPetConsumable {
		row, ok := b.RowAt(receipt.Destination)
		if !ok {
			return nil, fmt.Errorf("pickup destination slot %d missing", receipt.Destination)
		}
		update, e = protocol.InventoryUpdate([][protocol.CurrentItemRecordSize]byte{row})
		if e != nil {
			return nil, e
		}
	}
	plan := []outboundPacket{{"pickup_ack", 1, 43, []byte{1}}}
	// NOTI39's parser changes branch after the scene object has been removed.
	// Replaying a gold tail there would corrupt the ordinary fallback cursor.
	if applied {
		body, e := protocol.PickupConfirmed(r.Object, w.role.WireID, receipt.Destination, receipt.Award.Template == 0)
		if receipt.Award.Template == 0 {
			body, e = protocol.GoldPickupConfirmed(r.Object, w.role.WireID, receipt.Award.Amount)
		}
		if e != nil {
			return nil, e
		}
		plan = append(plan, outboundPacket{"pickup_scene_removed", 0, 39, body})
	}
	// NOTI39 applies the displayed delta first; the absolute committed balance
	// follows it, avoiding double-counting and repairing retried pickups.
	if isAccountMaterial {
		refresh, e := accountMaterialRefreshPackets(materials, saved)
		if e != nil {
			return nil, e
		}
		for _, p := range refresh {
			p.Name = "pickup_" + p.Name
			plan = append(plan, p)
		}
	} else if isPetConsumable {
		petBody, e := inventory.PetContainerBody(b, true)
		if e != nil {
			return nil, e
		}
		plan = append(plan, outboundPacket{"pickup_pet_container_restored", 0, 13, petBody})
	} else {
		plan = append(plan, outboundPacket{"pickup_inventory_updated", 0, 14, update})
	}
	w.role = saved
	if w.quests != nil {
		advanced, err := (&workflow.QuestService{Store: w.store, Quest: w.quests}).InventoryProgress(ctx, w.role)
		if err != nil {
			return nil, err
		}
		if len(advanced) > 0 {
			active, err := w.quests.Active(ctx, w.role)
			if err != nil {
				return nil, err
			}
			triggers, err := protocol.QuestTriggers(active)
			if err != nil {
				return nil, err
			}
			plan = append(plan, outboundPacket{"seeking_quest_triggers", 0, 291, triggers})
		}
	}
	return plan, nil
}
