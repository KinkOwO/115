package main

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"dfolan/internal/game/protocol"
	"dfolan/internal/inventory"
	"dfolan/internal/storage"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"strings"
	"time"
)

// CMD80 错误分支：客户端 sub_14529B2F0 拿到回包 [1..2] 的 u16 码后走 switch(v3)，
// 每个 case 再取一条 dstr 文案。所以**要发的是 switch 的小码，不是 dstr id**：
//
//	v3=4   → dstr 1658  No items are available.
//	v3=10  → dstr 1660  You do not have enough Gold.
//	v3=13  → dstr 1654  This item cannot be reinforced any further.   （95 同）
//	v3=17  → dstr 1652  Can be used on equipment only.
//	v3=22  → dstr 1661  You do not have enough materials.
//	v3=23  → dstr 1653  You may not upgrade or enchant this item.
//	default→ dstr 1662  An error occurred ... Code: %d.（会把 v3 原样打出来）
//
// 之前一律发 22，而 22 正好是「材料不足」——于是任何拒绝（等级上限、券不支持、
// 安全增幅超限……）都被玩家看成「材料不够」。现在按真实原因分流。
// 兜底段刻意用 90xx：客户端会把这个码原样显示在提示里，玩家回报即可定位。
const (
	cmd80ErrNoItems       uint16 = 4    // 东西不在背包 / 找不到
	cmd80ErrNoGold        uint16 = 10   // 金币不足
	cmd80ErrNoFurther     uint16 = 13   // 不能再强化/增幅
	cmd80ErrNotEquipment  uint16 = 17   // 只能对装备使用
	cmd80ErrNoMaterials   uint16 = 22   // 材料不足
	cmd80ErrNotUpgradable uint16 = 23   // 这件物品不能强化/附魔
	cmd80ErrGeneric       uint16 = 9000 // 兜底，客户端会显示 Code: 9000
)

// reinforcementRefusalCode 把拒绝原因映射成客户端能正确显示的错误码。
func reinforcementRefusalCode(err error) uint16 {
	if err == nil {
		return cmd80ErrGeneric
	}
	msg := err.Error()
	switch {
	case strings.Contains(msg, "金币不足"):
		return cmd80ErrNoGold
	// 「材料位放的东西不对」跟「材料不够」一样都是材料问题，不能归到「没有物品」：
	// 分流到 4 会弹 1658「No items are available.」，玩家根本看不出是材料位放错了；
	// 22 弹 1661「You do not have enough materials.」，至少把人引到材料位上去。
	case strings.Contains(msg, "材料不足"), strings.Contains(msg, "不是矛盾结晶体"),
		strings.Contains(msg, "不是安全增幅材料"), strings.Contains(msg, "既不是强化券"):
		return cmd80ErrNoMaterials
	case strings.Contains(msg, "上限"), strings.Contains(msg, "最高到"),
		strings.Contains(msg, "无法继续强化"), strings.Contains(msg, "超出规则表范围"),
		strings.Contains(msg, "不在安全增幅表内"), strings.Contains(msg, "不支持该固定强化券"):
		return cmd80ErrNoFurther
	case strings.Contains(msg, "不是装备"), strings.Contains(msg, "目标不是装备"):
		return cmd80ErrNotEquipment
	case strings.Contains(msg, "没有次元属性"), strings.Contains(msg, "不符合安全增幅条件"),
		strings.Contains(msg, "找不到"), strings.Contains(msg, "未找到"),
		strings.Contains(msg, "不支持"), strings.Contains(msg, "无法核对"),
		strings.Contains(msg, "保护券不能用于"):
		return cmd80ErrNotUpgradable
	case strings.Contains(msg, "不在背包"), strings.Contains(msg, "不在所属角色背包"),
		strings.Contains(msg, "不在背包或已穿戴槽位"), strings.Contains(msg, "需要已选角色"),
		strings.Contains(msg, "保护券槽位放的不是保护券"):
		return cmd80ErrNoItems
	}
	return cmd80ErrGeneric
}

// 强化的两条并列路径：
//   - ticket：窗口材料位放的是固定等级强化券（背包里的券）；
//   - gold：放的是消耗材料（无色小晶块 3037 / 炉岩核 3171），材料可能在账号共享材料仓库；
//   - amplify：**请求 mode=1**（增幅），放的是增幅材料矛盾结晶体 3242。
//     增幅与强化共用 CMD80 与同一个请求结构，唯一差别就是 mode。
const (
	reinforcementTicketBranch        = "ticket"
	reinforcementGoldBranch          = "gold"
	reinforcementAmplifyBranch       = "amplify"
	reinforcementAmplifyTicketBranch = "amplify_ticket"
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
		// 槽位里确实有东西、只是不是券也不是材料：把模板带出来，便于定位玩家放错了什么。
		return "", fmt.Errorf("窗口里的物品既不是强化券也不是强化材料（槽 %d 里是模板 %d）", r.TicketSlot, item.Template)
	}
	return "", fmt.Errorf("窗口里的物品既不是强化券也不是强化材料（槽 %d 里没有物品）", r.TicketSlot)
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
	// 增幅（mode=1）先判定：它与强化共用同一个请求结构，但材料、规则、等级偏移都不同，
	// 所以不能靠「窗口里放了什么东西」来区分，必须先看 mode。
	//
	// mode=1 内部还要再分一次：窗口「券位」放的是**增幅券**（跳级到券上写死的等级）
	// 还是**增幅材料**（矛盾结晶体 3242 / 安全增幅材料，每级 +1）。
	// 早期这里一律走材料增幅，于是券被当成材料校验 —— 不是 3242 就直接报
	// 「增幅材料槽位放的不是矛盾结晶体」，表现就是「增幅券不能直接附加到装备上」。
	branch := ""
	if r.Mode == 1 {
		b, berr := amplifyBranch(w.role, r)
		if berr != nil {
			return nil, berr
		}
		branch = b
	} else {
		b, berr := reinforcementBranch(w.role, r)
		if berr != nil {
			return nil, berr
		}
		branch = b
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	key := fmt.Sprintf("reinforcement-%s:%x:%x", branch, s.nonce, sha256.Sum256(raw))
	if branch == reinforcementAmplifyTicketBranch {
		return s.amplifyTicket(ctx, service, w, r, key, event)
	}
	if branch == reinforcementAmplifyBranch {
		return s.amplifyUpgrade(ctx, service, w, r, key, event)
	}
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
	ticketRow := bagRowOrEmpty(bag, r.TicketSlot)
	rows := [][protocol.CurrentItemRecordSize]byte{ticketRow}
	if r.EquipmentSpace == 0 {
		gearRow := bagRowOrEmpty(bag, r.EquipmentSlot)
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
	// ⚠️ 材料被扣完时该格已移除，必须发空行，否则图标留在原地（与保护券同一个坑）。
	// 判据只能用「不是账号材料仓库 且 剩余量 0」：账号材料仓库的格子**不在背包行里**，
	// 它用「省略该格」表示空格（见 AccountMaterials.Rows 的注释），
	// 拿 bag.RowAt 找不到就当空行会在剩余量 > 0 时误发空行。
	matRow := protocol.OrdinaryItem(out.MaterialSlot, out.MaterialTemplate, out.MaterialRemaining)
	if !out.MaterialFromStorage && out.MaterialRemaining == 0 {
		matRow = protocol.EmptyOrdinaryItem(out.MaterialSlot)
	}
	rows = append(rows, matRow)
	// 保护券触发时刷新保护券行，让客户端立即看到扣减。
	if out.Protected {
		// ⚠️ 必须走 bagRowOrEmpty：只有一张保护券时这一行会被整行移除，
		// 那时若什么都不发，客户端会把图标留在原地（实机 2026-09-28）。
		rows = append(rows, bagRowOrEmpty(bag, out.ProtectionSlot))
	}
	goldRow, _ := bag.RowAt(0)
	rows = append(rows, goldRow)
	if r.EquipmentSpace == 0 {
		gearRow := bagRowOrEmpty(bag, r.EquipmentSlot)
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
		"rate": out.Rate, "streak": out.Streak, "destroyed": out.Destroyed,
		"protected": out.Protected, "protection_slot": out.ProtectionSlot,
		"request_protection_slot": r.ProtectionSlot})
	return plan, nil
}

// amplifyUpgrade 增幅（CMD80 mode=1）：扣矛盾结晶体 3242 与金币，按官方成功率判定，
// 成功 +1、+7~+9 失败降一级、+10 以上失败摧毁。
//
// 回包沿用强化那套 35 字节布局（客户端处理 CMD80 回包的 handler 只有一个
// sub_14529B2F0），只把 mode 写成 1；随后照既有约定补发装备行刷新。
func (s *equipmentSession) amplifyUpgrade(ctx context.Context, service *inventory.WearService, w *worldSession, r protocol.ReinforcementRequest, key string, event func(map[string]any)) ([]outboundPacket, error) {
	saved, out, err := service.ApplyAmplifyUpgrade(ctx, w.role, key, r)
	if err != nil {
		return nil, err
	}
	w.role = saved
	ack, err := protocol.AmplifyUpgradeReply(r, out.MaterialRemaining, out.LevelBefore, out.LevelAfter, out.Result)
	if err != nil {
		return nil, err
	}
	plan := []outboundPacket{{"amplify_upgrade_result", 1, 80, ack}}
	bag, err := inventory.ReadBag(saved.State)
	if err != nil {
		return nil, err
	}
	rows := [][protocol.CurrentItemRecordSize]byte{}
	// 材料行：材料被扣完时该格已移除，用空行让客户端同步移除。
	matRow := bagRowOrEmpty(bag, out.MaterialSlot)
	rows = append(rows, matRow)
	// 保护券触发时刷新保护券行，让客户端立即看到扣减。
	if out.Protected {
		// ⚠️ 必须走 bagRowOrEmpty：只有一张保护券时这一行会被整行移除，
		// 那时若什么都不发，客户端会把图标留在原地（实机 2026-09-28）。
		rows = append(rows, bagRowOrEmpty(bag, out.ProtectionSlot))
	}
	if out.EquipmentSpace == 0 {
		gearRow := bagRowOrEmpty(bag, out.EquipmentSlot)
		rows = append(rows, gearRow)
	}
	body, err := protocol.InventoryUpdate(rows)
	if err != nil {
		return nil, err
	}
	plan = append(plan, outboundPacket{"amplify_upgrade_inventory", 0, 14, body})
	if out.EquipmentSpace == 3 {
		wornBody, werr := inventory.WornSpaceUpdate(saved.State)
		if werr != nil {
			return nil, werr
		}
		if len(wornBody) > 0 {
			plan = append(plan, outboundPacket{"amplify_upgrade_worn", 0, 14, wornBody})
		}
	}
	// ★ 金币行单独一包、放在最后发。
	// 实机：+10 以上增幅失败摧毁装备时，客户端会把金币显示清成 0
	// （存档里的金币是对的 —— 实测 549,632,620，重新登录即恢复）。
	// 原因还在查（疑似客户端处理「装备行被删除」的增量更新时把 list0 槽 0 一起冲掉），
	// 但无论根因如何，把金币放到最后一包单独下发都能确保客户端最终拿到正确余额。
	goldRow, _ := bag.RowAt(0)
	goldBody, gerr := protocol.InventoryUpdate([][protocol.CurrentItemRecordSize]byte{goldRow})
	if gerr != nil {
		return nil, gerr
	}
	plan = append(plan, outboundPacket{"amplify_upgrade_gold", 0, 14, goldBody})
	// 摧毁时再挂一次「延后补发」：客户端播完破坏动画后才刷 UI，那一步会把金币显示清 0。
	if out.Destroyed {
		s.pendingGoldBody = append([]byte(nil), goldBody...)
		s.pendingGoldDue = time.Now().Add(3 * time.Second)
	}
	event(map[string]any{"kind": "amplify_upgrade_inventory_rows", "character_id": saved.ID,
		"destroyed": out.Destroyed, "gold": bag.Gold,
		"rows":     amplifyRowSummary(rows),
		"body_hex": hex.EncodeToString(goldBody)})
	event(map[string]any{"kind": "amplify_upgrade_committed", "character_id": saved.ID,
		"mode": r.Mode, "material_slot": out.MaterialSlot, "material_spent": out.MaterialSpent,
		"remaining": out.MaterialRemaining, "gold_spent": out.GoldSpent, "gold": out.Gold,
		"equipment": r.EquipmentTemplate, "space": out.EquipmentSpace, "slot": out.EquipmentSlot,
		"amplify_type": out.AmplifyType, "before": out.LevelBefore, "after": out.LevelAfter,
		"result": out.Result, "penalty": out.Penalty, "destroyed": out.Destroyed,
		"safe": out.Safe, "rate": out.SuccessPercent,
		"protected": out.Protected, "protection_slot": out.ProtectionSlot,
		"request_protection_slot": r.ProtectionSlot})
	return plan, nil
}

// amplifyBranch 在 mode=1（增幅）内部再分一次流：看窗口「券位」里放的是
// 增幅券还是增幅材料。
//   - 增幅券（PVF 段 [equipment amplify reinforcement ticket]）→ 跳级到券的目标等级；
//   - 其余一律按增幅材料处理，由 applyAmplifyUpgrade 给出准确报错
//     （例如「增幅材料槽位放的不是矛盾结晶体」），这里不提前拦。
func amplifyBranch(role storage.Character, r protocol.ReinforcementRequest) (string, error) {
	bag, err := inventory.ReadBag(role.State)
	if err != nil {
		return "", err
	}
	for _, item := range bag.Items {
		if item.Slot != r.TicketSlot {
			continue
		}
		if inventory.IsAmplifyTicket(item.Template) {
			return reinforcementAmplifyTicketBranch, nil
		}
		return reinforcementAmplifyBranch, nil
	}
	return "", fmt.Errorf("增幅窗口里的物品不在背包里（槽 %d 里没有物品）", r.TicketSlot)
}

// amplifyTicket 增幅券（CMD80 mode=1，窗口里放的是券）：扣券、把装备增幅到券的
// 目标等级、刷新券与装备两行。
//
// 与 reinforceWithTicket（普通强化券）同套路：先用权威回包让客户端播结果动画，
// 再以增量行刷新，避免整包重建打断动画引用的装备对象。
func (s *equipmentSession) amplifyTicket(ctx context.Context, service *inventory.WearService, w *worldSession, r protocol.ReinforcementRequest, key string, event func(map[string]any)) ([]outboundPacket, error) {
	saved, out, err := service.ApplyAmplifyTicket(ctx, w.role, key, r)
	if err != nil {
		return nil, err
	}
	w.role = saved
	ack, err := protocol.AmplifyTicketReply(r, out.Remaining, out.Old, out.Level, out.Result)
	if err != nil {
		return nil, err
	}
	plan := []outboundPacket{{"amplify_ticket_result", 1, 80, ack}}
	bag, err := inventory.ReadBag(saved.State)
	if err != nil {
		return nil, err
	}
	rows := [][protocol.CurrentItemRecordSize]byte{}
	// 券行：券被扣完时该格已移除，用空行让客户端同步移除。
	ticketRow := bagRowOrEmpty(bag, r.TicketSlot)
	rows = append(rows, ticketRow)
	if out.EquipmentSpace == 0 {
		gearRow := bagRowOrEmpty(bag, r.EquipmentSlot)
		rows = append(rows, gearRow)
	}
	body, err := protocol.InventoryUpdate(rows)
	if err != nil {
		return nil, err
	}
	plan = append(plan, outboundPacket{"amplify_ticket_inventory", 0, 14, body})
	if out.EquipmentSpace == 3 {
		wornBody, werr := inventory.WornSpaceUpdate(saved.State)
		if werr != nil {
			return nil, werr
		}
		if len(wornBody) > 0 {
			plan = append(plan, outboundPacket{"amplify_ticket_worn", 0, 14, wornBody})
		}
	}
	event(map[string]any{"kind": "amplify_ticket_committed", "character_id": saved.ID,
		"mode": r.Mode, "ticket": out.Ticket, "ticket_slot": r.TicketSlot, "remaining": out.Remaining,
		"equipment": r.EquipmentTemplate, "space": out.EquipmentSpace, "slot": r.EquipmentSlot,
		"amplify_type": out.AmplifyType, "before": out.Old, "after": out.Level,
		"result": out.Result, "rate": out.SuccessPercent})
	return plan, nil
}

// amplifyRowSummary 把增量刷新包里的行压成可读摘要，便于抓包定位（槽位/模板/数量）。
func amplifyRowSummary(rows [][protocol.CurrentItemRecordSize]byte) []map[string]any {
	out := make([]map[string]any, 0, len(rows))
	for _, r := range rows {
		out = append(out, map[string]any{
			"slot":     binary.LittleEndian.Uint16(r[:]),
			"template": binary.LittleEndian.Uint32(r[2:]),
			"amount":   binary.LittleEndian.Uint32(r[6:]),
		})
	}
	return out
}
