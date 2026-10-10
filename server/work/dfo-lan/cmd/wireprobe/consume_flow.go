package main

import (
	"context"
	"dfolan/internal/catalog"
	"dfolan/internal/database"
	"dfolan/internal/game/protocol"
	"dfolan/internal/inventory"
	"dfolan/internal/legion"
	"dfolan/internal/workflow"
	"fmt"
	"time"
)

// useStackable answers CMD44. The client applies the recovery effect itself,
// so the server owns the durable decrement, the acknowledgement echoing the
// consumed slot, and the authoritative bag refresh that follows it. A box also
// hands out its source lot row in that same transaction; event records both
// shapes so a run can be read back without the capture log.
func (w *worldSession) useStackable(p []byte, event func(map[string]any)) ([]outboundPacket, error) {
	if w == nil || w.role.ID == 0 || w.items == nil {
		return nil, fmt.Errorf("item use before character selection")
	}
	r, e := protocol.DecodeUseStackable(p)
	if e != nil {
		return nil, e
	}
	if refused := w.odysseyConsumableGate(r); refused != nil {
		return refused, nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	before, e := inventory.ReadBag(w.role.State)
	if e != nil {
		return nil, e
	}
	service := &workflow.ItemService{Store: w.store, Items: w.items}
	var saved database.Character
	var receipt inventory.ConsumeReceipt
	var applied bool
	if r.Template == catalog.NativeCharacterSlotTicket {
		if w.characters == nil {
			return nil, fmt.Errorf("character rules unavailable")
		}
		saved, receipt, applied, e = service.ConsumeCharacterSlotTicket(ctx, w.role, r, w.characters.Rules.MaxCharacters)
	} else {
		saved, receipt, applied, e = service.ConsumeWithCheck(ctx, w.role, r, w.checkContentPotionUse)
	}
	if e != nil {
		return nil, e
	}
	// Connections serialize run mutations. This in-memory quota belongs to
	// this owned attempt only; disconnect ends the attempt, not resumes it.
	if applied && w.ispinsLimitedPotion(r) {
		w.ispins.potionUsed++
	}
	if applied && w.forestLimitedPotion(r) {
		w.forest.potionUsed[w.forest.stage]++
	}
	if applied && w.venusLimitedPotion(r) {
		stage, _ := legion.VenusStageOfDungeon(w.activeDungeon.Definition.ID)
		w.venus.potionUsed[stage]++
	}
	// One durable record per use: a box names the source lot row it handed out,
	// an ordinary consumable records the slot it spent.
	if len(receipt.Granted) > 0 {
		event(map[string]any{"kind": "box_opened", "character_id": w.role.ID,
			"box": receipt.Template, "slot": receipt.Slot,
			"granted": receipt.Granted, "points": receipt.Points})
	} else {
		event(map[string]any{"kind": "item_consumed", "character_id": w.role.ID,
			"template": receipt.Template, "slot": receipt.Slot, "remaining": receipt.Remaining})
	}
	ack, e := protocol.UseStackableSuccess(r)
	if e != nil {
		return nil, e
	}
	b, e := inventory.ReadBag(saved.State)
	if e != nil {
		return nil, e
	}
	if r.List == 7 {
		petBody, err := inventory.PetContainerBody(b, false)
		if err != nil {
			return nil, err
		}
		creatures, err := inventory.CreatureListPayload(saved.State)
		if err != nil {
			return nil, err
		}
		w.role = saved
		return []outboundPacket{
			{"pet_feed_ack", 1, 44, ack},
			{"pet_feed_container_updated", 0, 14, petBody},
			{"pet_feed_creature_list_updated", 0, 105, creatures},
		}, nil
	}
	update, e := protocol.InventoryUpdate(inventory.ChangedItemRows(before, b))
	if e != nil {
		return nil, e
	}
	w.role = saved
	if len(receipt.Premiums) > 0 {
		restore, err := protocol.InventoryRestore(b.Rows(), b.Expansion)
		if err != nil {
			return nil, err
		}
		event(map[string]any{"kind": "item_contracts_activated", "character_id": w.role.ID, "premiums": receipt.Premiums})
		return []outboundPacket{
			{"item_use_ack", 1, 44, ack},
			{"item_use_inventory_restored", 0, 13, restore},
		}, nil
	}
	// The absolute committed bag follows the acknowledgement, so a retried
	// hotkey press cannot leave the client's own count drifting.
	plan := []outboundPacket{
		{"item_use_ack", 1, 44, ack},
		{"item_use_inventory_updated", 0, 14, update},
	}
	if receipt.CharacterCapacity != 0 {
		body, err := protocol.CharacterCapacityUpdated(receipt.CharacterCapacity)
		if err != nil {
			return nil, err
		}
		plan = append(plan, outboundPacket{"character_capacity_updated", 0, 66, body})
		event(map[string]any{"kind": "character_slot_expanded", "character_id": w.role.ID, "account_id": w.role.AccountID, "capacity": receipt.CharacterCapacity, "applied": applied})
	}
	if mask, skinTicket := inventory.SkinSlotMaskForTicket(r.Template, w.items.Catalog); skinTicket && b.ExpandEquipFlags&mask != 0 {
		refresh, err := w.unlockRefresh(saved)
		if err != nil {
			return nil, err
		}
		plan = append(plan, refresh...)
	}
	return plan, nil
}
