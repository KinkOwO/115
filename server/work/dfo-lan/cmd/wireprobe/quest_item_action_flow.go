package main

import (
	"context"
	"dfolan/internal/game/protocol"
	"dfolan/internal/inventory"
	"fmt"
	"time"
)

// useQuestAirshipItem handles the captured CMD507 action 206 only when the
// owned item matches an accepted, source-backed single-use quest objective.
func (w *worldSession) useQuestAirshipItem(p []byte, event func(map[string]any)) ([]outboundPacket, error) {
	if w == nil || w.role.ID == 0 || w.quests == nil || w.loot == nil {
		return nil, fmt.Errorf("quest item use before character selection")
	}
	if w.activeDungeon != nil || w.state.Position.Town == 0 {
		return nil, fmt.Errorf("quest item action requires town")
	}
	slot, err := protocol.DecodeQuestAirshipAction(p)
	if err != nil {
		return nil, err
	}
	before, err := inventory.ReadBag(w.role.State)
	if err != nil {
		return nil, err
	}
	var template uint32
	for _, item := range before.Items {
		if item.Slot == slot && item.Amount > 0 {
			template = item.Template
			break
		}
	}
	if template == 0 {
		return nil, fmt.Errorf("quest item slot is empty")
	}
	matching := w.quests.Index().ByUseItem[template]
	if len(matching) == 0 {
		return nil, fmt.Errorf("item has no supported use objective")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	active, err := w.quests.Active(ctx, w.role)
	if err != nil {
		return nil, err
	}
	pending := false
	for _, q := range active {
		if q.Progress != 1 {
			continue
		}
		for _, id := range matching {
			if id == q.ID {
				pending = true
				break
			}
		}
	}
	if !pending {
		return nil, fmt.Errorf("no accepted quest requires this item")
	}
	saved, receipt, _, err := w.loot.Consume(ctx, w.role,
		protocol.UseStackableRequest{Slot: slot, List: 0, Template: template})
	if err != nil {
		return nil, err
	}
	w.role = saved
	event(map[string]any{"kind": "item_consumed", "character_id": saved.ID,
		"template": receipt.Template, "slot": receipt.Slot, "remaining": receipt.Remaining})
	advanced, err := w.quests.UseItem(ctx, saved, template, receipt.EventKey)
	if err != nil {
		return nil, err
	}
	after, err := inventory.ReadBag(saved.State)
	if err != nil {
		return nil, err
	}
	update, err := protocol.InventoryUpdate(inventory.ChangedItemRows(before, after))
	if err != nil {
		return nil, err
	}
	plan := []outboundPacket{{"quest_item_inventory_updated", 0, 14, update}}
	if len(advanced) != 0 {
		active, err = w.quests.Active(ctx, saved)
		if err != nil {
			return nil, err
		}
		triggers, err := protocol.QuestTriggers(active)
		if err != nil {
			return nil, err
		}
		plan = append(plan, outboundPacket{"quest_item_objective", 0, 291, triggers})
		event(map[string]any{"kind": "quest_item_objective", "character_id": saved.ID,
			"template": template, "quests": advanced})
	}
	return plan, nil
}
