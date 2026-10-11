package main

import (
	"context"
	"dfolan/internal/database"
	"dfolan/internal/game/protocol"
	"dfolan/internal/inventory"
	"dfolan/internal/loot"
	"dfolan/internal/workflow"
	"fmt"
	"log"
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
	return w.finishPickup(ctx, saved, receipt, applied, true)
}

func (w *worldSession) finishPickup(ctx context.Context, saved database.Character, receipt loot.PickupReceipt, applied, acknowledge bool) ([]outboundPacket, error) {
	previousState := w.role.State
	// 落账成功后先保留最新状态，后续通知构造失败也不能恢复旧背包。
	w.role = saved
	var e error
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
	// 一次拾取可能填满多个旧堆叠再占新格；只刷新变动格子，保留幂等回显。
	var update []byte
	if !isAccountMaterial && !isPetConsumable {
		row, ok := b.RowAt(receipt.Destination)
		if !ok {
			return nil, fmt.Errorf("pickup destination slot %d missing", receipt.Destination)
		}
		before, err := inventory.ReadBag(previousState)
		if err != nil {
			return nil, err
		}
		rows := inventory.ChangedItemRows(before, b)
		if len(rows) == 0 {
			rows = append(rows, row)
		}
		update, e = protocol.InventoryUpdate(rows)
		if e != nil {
			return nil, e
		}
	}
	var plan []outboundPacket
	if acknowledge {
		plan = append(plan, outboundPacket{"pickup_ack", 1, 43, []byte{1}})
	}
	// NOTI39's parser changes branch after the scene object has been removed.
	// Replaying a gold tail there would corrupt the ordinary fallback cursor.
	if applied {
		body, e := protocol.PickupConfirmed(receipt.Object, w.role.WireID, receipt.Destination, receipt.Award.Template == 0)
		if receipt.Award.Template == 0 {
			body, e = protocol.GoldPickupConfirmed(receipt.Object, w.role.WireID, receipt.Award.Amount)
		}
		if e != nil {
			return nil, e
		}
		plan = append(plan, outboundPacket{"pickup_scene_removed", 0, 39, body})
	}
	// NOTI39 applies the displayed delta first; the absolute committed balance
	// follows it, avoiding double-counting and repairing retried pickups.
	if isAccountMaterial {
		refresh, e := accountMaterialRefreshPackets(materials, saved, w.activeDungeon != nil)
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

func (w *worldSession) autoPickupDrops(rows []protocol.SceneDrop) []outboundPacket {
	if !w.autoPickup || w.loot == nil || w.activeDungeon == nil {
		return nil
	}
	var plan []outboundPacket
	for _, row := range rows {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		saved, receipt, applied, err := (&workflow.LootService{Store: w.store, Loot: w.loot}).AutoPickup(ctx, w.role, w.drops, w.activeDungeon, row.Object)
		if err != nil {
			cancel()
			log.Printf("自动拾取未入包，保留地面物品：角色=%d 对象=%d 错误=%v", w.role.ID, row.Object, err)
			continue
		}
		packets, err := w.finishPickup(ctx, saved, receipt, applied, false)
		cancel()
		if err != nil {
			log.Printf("自动拾取已落账但同步失败：角色=%d 对象=%d 错误=%v", w.role.ID, row.Object, err)
			continue
		}
		plan = append(plan, packets...)
	}
	return plan
}
