package main

import (
	"context"
	"crypto/sha256"
	"dfolan/internal/game/protocol"
	"dfolan/internal/inventory"
	"fmt"
	"strings"
	"time"
)

// refineRefusalCode 把锻造的拒绝原因翻译成客户端认得的错误码。
//
// 锻造与其它升级命令共用同一张错误码表（客户端 handler sub_145886FF0 的 switch），
// **唯一差别是 17 那格**：锻造弹出的是 35076 "The equipment cannot be refined."，
// 而 CMD80 弹的是 1652。所以「不是武器」「已满级」这两条官方限制都走 17 最贴切。
//
// 绝不能落进 4（"No items are available."）—— 强化/增幅那次全弹 1658 的坑就在这里：
// 玩家看见「没有可用物品」根本想不到是材料位放错了。
func refineRefusalCode(err error) uint16 {
	if err == nil {
		return cmd80ErrGeneric
	}
	msg := err.Error()
	switch {
	case strings.Contains(msg, "只有武器"), strings.Contains(msg, "上限"):
		return cmd80ErrNotEquipment // 17 → "The equipment cannot be refined."
	case strings.Contains(msg, "材料不足"), strings.Contains(msg, "不是 Powerful Energy"),
		strings.Contains(msg, "取不到材料消耗"):
		return cmd80ErrNoMaterials // 22 → "You do not have the materials required to Refine."
	case strings.Contains(msg, "不在背包"):
		return cmd80ErrNoItems
	case strings.Contains(msg, "目标不是装备"):
		return cmd80ErrNotUpgradable
	}
	return cmd80ErrGeneric
}

// CMD 430 = 锻造（Refine，NPC Kiri）。
//
// 与 CMD80（强化/增幅）不同，锻造是独立 opcode：
//   - 请求里没有 mode 字节，材料只带一个槽（Powerful Energy 3326）；
//   - 回包只有 13 字节，布局与 CMD80 的 35 字节完全不同（客户端 handler sub_145886FF0）；
//   - 官方规则：仅武器、上限 +8、失败等级不变且装备不碎。
//
// 成功后照既有约定补发装备行/材料行刷新 —— 客户端不会用回包里的等级去改物品对象，
// 它只在结果面板上显示 [7]/[9]，真正的等级靠刷新行重新反序列化。
func (w *worldSession) refine(service *inventory.WearService, p, raw []byte, event func(map[string]any)) ([]outboundPacket, error) {
	if service == nil || w == nil || w.role.ID == 0 || w.activeDungeon != nil {
		return nil, fmt.Errorf("锻造需要已选角色且位于城镇")
	}
	r, err := protocol.DecodeRefine(p)
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	key := fmt.Sprintf("refine:%x", sha256.Sum256(raw))
	saved, out, err := service.ApplyRefine(ctx, w.role, key, r)
	if err != nil {
		return nil, err
	}
	w.role = saved
	ack, err := protocol.RefineReply(out.MaterialSlot, out.MaterialRemaining,
		out.LevelBefore, out.LevelAfter, out.Result, out.EquipmentSpace, out.EquipmentSlot)
	if err != nil {
		return nil, err
	}
	plan := []outboundPacket{{"refine_result", 1, 430, ack}}

	bag, err := inventory.ReadBag(saved.State)
	if err != nil {
		return nil, err
	}
	rows := [][protocol.CurrentItemRecordSize]byte{}
	// 材料行：被扣完时该格已移除，用空行让客户端同步移除。
	matRow, exists := bag.RowAt(out.MaterialSlot)
	if !exists {
		matRow = protocol.EmptyOrdinaryItem(out.MaterialSlot)
	}
	rows = append(rows, matRow)
	if out.EquipmentSpace == 0 {
		gearRow, ok := bag.RowAt(out.EquipmentSlot)
		if !ok {
			gearRow = protocol.EmptyOrdinaryItem(out.EquipmentSlot)
		}
		rows = append(rows, gearRow)
	}
	body, err := protocol.InventoryUpdate(rows)
	if err != nil {
		return nil, err
	}
	plan = append(plan, outboundPacket{"refine_inventory", 0, 14, body})
	if out.EquipmentSpace == 3 {
		wornBody, werr := inventory.WornSpaceUpdate(saved.State)
		if werr != nil {
			return nil, werr
		}
		if len(wornBody) > 0 {
			plan = append(plan, outboundPacket{"refine_worn", 0, 14, wornBody})
		}
	}
	event(map[string]any{
		"kind": "refine_committed", "character_id": saved.ID,
		"equipment": r.EquipmentTemplate, "space": out.EquipmentSpace, "slot": out.EquipmentSlot,
		"before": out.LevelBefore, "after": out.LevelAfter, "result": out.Result,
		"success": out.Success, "rate": out.SuccessPercent,
		"material": inventory.RefineMaterialTemplate(), "material_slot": out.MaterialSlot,
		"material_spent": out.MaterialSpent, "material_remaining": out.MaterialRemaining,
		"record_offset": out.RecordOffset,
		"row_before":    out.RowBefore, "row_after": out.RowAfter,
	})
	return plan, nil
}
