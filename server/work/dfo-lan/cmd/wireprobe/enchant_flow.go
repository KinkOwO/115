package main

import (
	"context"
	"crypto/sha256"
	"dfolan/internal/game/protocol"
	"dfolan/internal/inventory"
	"fmt"
	"time"
)

// enchantRefusalCode 把附魔的拒绝原因翻译成客户端认得的错误码。
//
// 客户端 CMD272 handler sub_1452762c0 的失败分支（a2==0）只对三个码有专属文案，
// 其余落 default：
//
//	17 → dstr 22223 "The item cannot be Enchanted."
//	19 → dstr 46371 "You cannot Enchant on this equipment."
//	23 → dstr 27162 "This item cannot be used."
//	default → dstr 44171 "Unknown Error"
//
// 所以「宝珠不对 / 宝珠不在背包」走 17，「目标不是装备」走 19，兜底 23。
func enchantRefusalCode(err error) uint16 {
	switch inventory.RefusalOf(err) {
	case inventory.RefusalItems:
		return cmd272ErrItem
	case inventory.RefusalEquipment:
		return cmd272ErrEquipment
	default:
		return cmd272ErrGeneric
	}
}

const (
	cmd272ErrItem      uint16 = 17 // "The item cannot be Enchanted."
	cmd272ErrEquipment uint16 = 19 // "You cannot Enchant on this equipment."
	cmd272ErrGeneric   uint16 = 23 // "This item cannot be used."
)

// CMD 272 = 附魔宝珠（ENCHANT_BY_BEAD）。
//
// 请求 16 字节：u8 宝珠空间 + u16 宝珠槽 + u8 装备空间 + u16 装备槽。
// 成功回包 4 字节：status(1) + u8 装备空间 + u16 装备槽（客户端 handler 只读这两个字段）。
// 成功后照既有约定补发宝珠行/装备行刷新 —— 附魔属性靠刷新行重新反序列化后显示。
func (w *worldSession) enchantByBead(service *inventory.WearService, p, raw []byte, event func(map[string]any)) ([]outboundPacket, error) {
	if service == nil || w == nil || w.role.ID == 0 || w.activeDungeon != nil {
		return nil, fmt.Errorf("附魔需要已选角色且位于城镇")
	}
	r, err := protocol.DecodeEnchantByBead(p)
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	key := fmt.Sprintf("enchant:%x", sha256.Sum256(raw))
	saved, out, err := service.ApplyEnchantByBead(ctx, w.role, key, r)
	if err != nil {
		return nil, err
	}
	w.role = saved
	plan := []outboundPacket{}

	bag, err := inventory.ReadBag(saved.State)
	if err != nil {
		return nil, err
	}
	rows := [][protocol.CurrentItemRecordSize]byte{}
	// 宝珠行：被扣完时该格已移除，用空行让客户端同步移除。
	rows = append(rows, bagRowOrEmpty(bag, out.BeadSlot))
	if out.EquipmentSpace == 0 {
		rows = append(rows, bagRowOrEmpty(bag, out.EquipmentSlot))
	}
	plan, err = appendEquipmentUpdates(plan, saved.State, rows, out.EquipmentSpace, "enchant_inventory", "enchant_worn")
	if err != nil {
		return nil, err
	}
	// ★ 回包放在物品行刷新之后：客户端是收到回包才去读那件装备并刷新附魔窗口的，
	// 若回包先到，窗口会拿到还没更新的旧装备 —— 表现就是「附魔成功了，但预览窗口还是旧的」。
	plan = append(plan, outboundPacket{"enchant_result", 1, 272,
		protocol.EnchantByBeadReply(out.EquipmentSpace, out.EquipmentSlot)})
	event(map[string]any{
		"kind": "enchant_committed", "character_id": saved.ID,
		"bead": out.BeadTemplate, "bead_slot": out.BeadSlot, "bead_remaining": out.BeadRemaining,
		"equipment": out.Equipment.Template, "space": out.EquipmentSpace, "slot": out.EquipmentSlot,
		"card": out.Card, "prev_enchant": out.PrevCard,
		"row_before": out.RowBefore, "row_after": out.RowAfter,
	})
	return w.appendFameUpdate(plan, event), nil
}
