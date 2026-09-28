package main

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"dfolan/internal/game/protocol"
	"dfolan/internal/inventory"
	"dfolan/internal/storage"
	"fmt"
	"time"
)

// 强化的两条并列路径：
//   - ticket：窗口材料位放的是固定等级强化券（背包里的券）；
//   - gold：放的是消耗材料（无色小晶块 3037 / 炉岩核 3171），材料可能在账号共享材料仓库。
const (
	reinforcementTicketBranch = "ticket"
	reinforcementGoldBranch   = "gold"
)

// reinforcementBranch 判断这次 CMD80 走哪条路径。@9 是玩家放进窗口那件东西的槽位：
// 363..379 是账号材料仓库格（无色小晶块固定 367），其余是背包槽位，需要看模板是券还是材料。
func reinforcementBranch(role storage.Character, r protocol.ReinforcementRequest) (string, error) {
	if _, ok := inventory.StorageRowTemplate(r.TicketSlot); ok {
		return reinforcementGoldBranch, nil
	}
	bag, err := inventory.ReadBag(role.State)
	if err != nil {
		return "", err
	}
	for _, item := range bag.Items {
		if item.Slot != r.TicketSlot {
			continue
		}
		if inventory.IsReinforcementTicket(item.Template) {
			return reinforcementTicketBranch, nil
		}
		if inventory.IsGoldMaterial(item.Template) || inventory.IsSafeMaterial(item.Template) {
			return reinforcementGoldBranch, nil
		}
		break
	}
	return "", fmt.Errorf("窗口里的物品既不是强化券也不是强化材料")
}

func (s *equipmentSession) reinforce(service *inventory.WearService, w *worldSession, p, raw []byte, event func(map[string]any)) ([]outboundPacket, error) {
	if service == nil || w == nil || w.role.ID == 0 || w.activeDungeon != nil {
		return nil, fmt.Errorf("强化需要已选角色且位于城镇")
	}
	r, err := protocol.DecodeReinforcement(p)
	if err != nil {
		return nil, err
	}
	if !s.initialized {
		if _, err = rand.Read(s.nonce[:]); err != nil {
			return nil, err
		}
		s.initialized = true
	}
	branch, err := reinforcementBranch(w.role, r)
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	key := fmt.Sprintf("reinforcement-%s:%x:%x", branch, s.nonce, sha256.Sum256(raw))
	if branch == reinforcementGoldBranch {
		return s.reinforceWithMaterial(ctx, service, w, r, key, event)
	}
	return s.reinforceWithTicket(ctx, service, w, r, key, event)
}

// reinforceWithTicket 固定等级强化券：扣券、写等级、刷新券与装备两行。
func (s *equipmentSession) reinforceWithTicket(ctx context.Context, service *inventory.WearService, w *worldSession, r protocol.ReinforcementRequest, key string, event func(map[string]any)) ([]outboundPacket, error) {
	saved, out, err := service.ReinforceWithTicket(ctx, w.role, key, r)
	if err != nil {
		return nil, err
	}
	w.role = saved
	ack, err := protocol.ReinforcementTicketReply(r, out.Remaining, out.Old, out.Level, out.Result)
	if err != nil {
		return nil, err
	}
	// 原生应答先处理旧物品对象，再以权威行刷新；不能先删掉最后一张券。
	plan := []outboundPacket{{"reinforcement_ticket_result", 1, 80, ack}}
	bag, err := inventory.ReadBag(saved.State)
	if err != nil {
		return nil, err
	}
	// 即使重放旧回执，也发送当前槽位；采用增量更新保留强化动画引用的装备对象。
	ticketRow, exists := bag.RowAt(r.TicketSlot)
	if !exists {
		ticketRow = protocol.EmptyOrdinaryItem(r.TicketSlot)
	}
	rows := [][protocol.CurrentItemRecordSize]byte{ticketRow}
	if r.EquipmentSpace == 0 {
		gearRow, exists := bag.RowAt(r.EquipmentSlot)
		if !exists {
			gearRow = protocol.EmptyOrdinaryItem(r.EquipmentSlot)
		}
		rows = append(rows, gearRow)
	}
	body, err := protocol.InventoryUpdate(rows)
	if err != nil {
		return nil, err
	}
	plan = append(plan, outboundPacket{"reinforcement_ticket_inventory", 0, 14, body})
	if r.EquipmentSpace == 3 {
		body, err = inventory.WornSpaceUpdate(saved.State)
		if err != nil {
			return nil, err
		}
		if len(body) > 0 {
			plan = append(plan, outboundPacket{"reinforcement_equipment_updated", 0, 14, body})
		}
	}
	event(map[string]any{"kind": "reinforcement_ticket_committed", "character_id": saved.ID,
		"ticket": out.Ticket, "remaining": out.Remaining, "equipment": r.EquipmentTemplate,
		"space": r.EquipmentSpace, "slot": r.EquipmentSlot, "before": out.Old, "after": out.Level, "result": out.Result})
	return plan, nil
}

// reinforceWithMaterial 金币强化（材料 + 金币）。材料在账号共享仓库时，
// 除了 CMD80 应答，还要按既有约定补发 list35/list42/list0 三连快照，让客户端重新收割。
func (s *equipmentSession) reinforceWithMaterial(ctx context.Context, service *inventory.WearService, w *worldSession, r protocol.ReinforcementRequest, key string, event func(map[string]any)) ([]outboundPacket, error) {
	saved, out, err := service.ReinforceWithMaterial(ctx, w.role, key, r)
	if err != nil {
		return nil, err
	}
	w.role = saved
	ack, err := protocol.ReinforcementGoldReply(r, out.MaterialRemaining, out.Old, out.Level, out.Result)
	if err != nil {
		return nil, err
	}
	plan := []outboundPacket{{"reinforcement_gold_result", 1, 80, ack}}
	bag, err := inventory.ReadBag(saved.State)
	if err != nil {
		return nil, err
	}
	var rows [][protocol.CurrentItemRecordSize]byte
	// 材料行：背包槽与账号材料仓库格（363..379）都按 list0 行回填 —— 客户端的 list0 读取器
	// 会把这些格子收割进材料仓库面板（sub_145ADC2A0）。这里**不发整包快照**：
	// 券路径的注释已经写明整包重建会打断强化动画引用的装备对象。
	rows = append(rows, protocol.OrdinaryItem(out.MaterialSlot, out.MaterialTemplate, out.MaterialRemaining))
	goldRow, _ := bag.RowAt(0)
	rows = append(rows, goldRow)
	if r.EquipmentSpace == 0 {
		gearRow, exists := bag.RowAt(r.EquipmentSlot)
		if !exists {
			gearRow = protocol.EmptyOrdinaryItem(r.EquipmentSlot)
		}
		rows = append(rows, gearRow)
	}
	body, err := protocol.InventoryUpdate(rows)
	if err != nil {
		return nil, err
	}
	plan = append(plan, outboundPacket{"reinforcement_gold_inventory", 0, 14, body})
	if r.EquipmentSpace == 3 {
		body, err = inventory.WornSpaceUpdate(saved.State)
		if err != nil {
			return nil, err
		}
		if len(body) > 0 {
			plan = append(plan, outboundPacket{"reinforcement_equipment_updated", 0, 14, body})
		}
	}
	if out.MaterialFromStorage {
		// 只同步材料仓库面板（list35），不发整包 list0 快照 —— 整包重建会打断强化动画引用的装备对象。
		if counts, e := service.Store.AccountMaterials(ctx, saved.AccountID); e == nil {
			if materials, e2 := inventory.ReadAccountMaterials(counts); e2 == nil {
				if storageBody, e3 := accountMaterialSnapshot(materials); e3 == nil {
					plan = append(plan, outboundPacket{"account_materials_restored", 0, 13, storageBody})
				}
			}
		}
	}
	event(map[string]any{"kind": "reinforcement_gold_committed", "character_id": saved.ID,
		"mode": out.Mode, "material": out.MaterialTemplate, "material_slot": out.MaterialSlot, "material_spent": out.MaterialSpent,
		"remaining": out.MaterialRemaining, "gold_spent": out.GoldSpent, "gold": out.Gold,
		"equipment": r.EquipmentTemplate, "space": r.EquipmentSpace, "slot": r.EquipmentSlot,
		"before": out.Old, "after": out.PostLevel, "reply_level": out.Level, "result": out.Result,
		"rate": out.Rate, "streak": out.Streak, "destroyed": out.Destroyed})
	return plan, nil
}
