package main

import (
	"context"
	"dfolan/internal/game/protocol"
	"dfolan/internal/inventory"
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
	// 苏醒之森军团口径：副本内消耗品每关限 8 次（用户要求），超限拒绝且
	// 不扣库存。gate 命中时森林计数已 +1。
	if refused := w.forestPotionGate(r); refused != nil {
		return refused, nil
	}
	// 维纳斯军团口径：同款每关 8 次（BUG2，用户确认副本内无法使用任何
	// 消耗品——缺 N1584 许可，见 dungeon_flow）。
	if refused := w.venusPotionGate(r); refused != nil {
		return refused, nil
	}
	// 奥德赛模式口径（业主 2026-10-06，由服务端 mod 打开）：副本内禁止使用任何
	// 消耗品、可以携带；城镇不受影响。客户端本来就不发 N1584 = 界面已灰，
	// 这里挡的是权威侧（改过的客户端绕过界面也拿不到药）。
	if refused := w.odysseyConsumableGate(r); refused != nil {
		return refused, nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	before, e := inventory.ReadBag(w.role.State)
	if e != nil {
		return nil, e
	}
	var check func() error
	limited := false
	if w.bakal != nil && w.activeDungeon != nil && w.activeDungeon.RaidManaged && r.List == 0 {
		script, err := w.items.Catalog.ItemScript(r.Template)
		if err != nil {
			return nil, err
		}
		for _, token := range script.Cells {
			if token.Type == 3 && token.Text == "[stackable dungeon limit]" {
				limited = true
				break
			}
		}
		if limited {
			check = w.bakal.CheckPotionBudget
		}
	}
	var committed func()
	if limited {
		committed = func() {
			w.bakal.SpendPotionBudget()
			coins, potions := w.bakal.Budget()
			event(map[string]any{"kind": "bakal_consumable_budget", "raid": w.bakalRun, "coins": coins, "potions": potions})
		}
	}
	saved, receipt, _, e := (&workflow.ItemService{Store: w.store, Items: w.items}).ConsumeChecked(ctx, w.role, r, check, committed)
	if e != nil {
		return nil, e
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
	return plan, nil
}
