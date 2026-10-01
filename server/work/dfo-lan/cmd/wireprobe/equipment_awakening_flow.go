package main

import (
	"context"
	"dfolan/internal/game/protocol"
	"dfolan/internal/inventory"
	"dfolan/internal/workflow"
	"encoding/hex"
	"time"
)

// CMD2258 装备调适（`ENUM_CMDPACKET_EQUIPMENT_AWAKENING`）。
//
// 协议与规则来源见：
//   - internal/game/protocol/equipment_awakening.go（25 字节请求 / 应答语义，IDA 证据在注释里）
//   - internal/catalog/equipment_awakening.go（直读 etc/115lvability/equipmentawakeningoptionsystem.cos）
//
// 回包形状（客户端 handler sub_140B899B0）：
//
//	kind = 1，体 = u8 状态 + u16 结果码；**状态 0 = 成功**，非 0 = 失败收尾。
//
// 成功后按既有约定补发增量行（装备行 + 金币行 + 被扣材料行），
// 若材料在账号共享材料仓库则再补一帧 list35 快照；穿戴装备升品会改名望，
// 所以统一走 appendFameUpdate。
func (s *equipmentSession) awakenEquipment(service *workflow.WearService, w *worldSession, p, raw []byte, event func(map[string]any)) ([]outboundPacket, error) {
	if service == nil || w == nil || w.role.ID == 0 || w.activeDungeon != nil {
		return nil, inventory.Refuse(inventory.RefusalItems, "装备调适需要已选角色且位于城镇")
	}
	r, err := protocol.DecodeEquipmentAwakening(p)
	if err != nil {
		return nil, err
	}
	key, err := s.requestKey(raw)
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	saved, out, err := service.ApplyEquipmentAwakening(ctx, w.role, "equipment-awakening:"+key, r)
	if err != nil {
		event(map[string]any{"kind": "equipment_awakening_refused", "character_id": w.role.ID,
			"reason": err.Error(), "request_hex": hex.EncodeToString(p), "payload_offset": r.PayloadOffset})
		return nil, err
	}
	w.role = saved
	ack := protocol.EquipmentAwakeningSuccess()
	plan := []outboundPacket{{"equipment_awakening_result", 1, protocol.EquipmentAwakeningOpcode, ack}}
	bag, err := inventory.ReadBag(saved.State)
	if err != nil {
		return nil, err
	}
	var rows [][protocol.CurrentItemRecordSize]byte
	// 金币行：调适会扣金币，必须刷。
	if goldRow, ok := bag.RowAt(0); ok {
		rows = append(rows, goldRow)
	}
	storageTouched := false
	for _, spent := range out.Spent {
		if spent.Template == protocol.EquipmentAwakeningNoTarget {
			continue
		}
		if spent.Template == 0 {
			continue // 金币行已在上面处理
		}
		if _, _, ok := inventory.AccountMaterialTarget(spent.Template); ok {
			// 账号共享材料：只要这次动过这类模板就补发 list35 快照（即使只扣了背包那份）。
			storageTouched = true
			continue
		}
		if slot, ok := bagSlotOfTemplate(bag, spent.Template); ok {
			rows = append(rows, bagRowOrEmpty(bag, slot))
		}
	}
	rows = append(rows, equipmentRows(bag, r.Space, r.Slot)...)
	plan, err = appendEquipmentUpdates(plan, saved.State, rows, r.Space, "equipment_awakening_inventory", "equipment_awakening_equipment")
	if err != nil {
		return nil, err
	}
	if storageTouched {
		if counts, e := service.Store.AccountMaterials(ctx, saved.AccountID); e == nil {
			if materials, e2 := inventory.ReadAccountMaterials(counts); e2 == nil {
				if body, e3 := accountMaterialSnapshot(materials); e3 == nil {
					plan = append(plan, outboundPacket{"equipment_awakening_materials", 0, 13, body})
				}
			}
		}
	}
	spentDetail := make([]map[string]any, 0, len(out.Spent))
	for _, s := range out.Spent {
		spentDetail = append(spentDetail, map[string]any{
			"template": s.Template, "amount": s.Amount, "from_storage": s.FromStorage})
	}
	event(map[string]any{"kind": "equipment_awakening_committed", "character_id": saved.ID,
		"mode": r.Mode, "material_group": r.MaterialGroup, "space": r.Space, "slot": r.Slot,
		"equipment": out.TemplateBefore, "target": out.TemplateAfter,
		"stage_before": out.StageBefore, "stage_after": out.StageAfter, "upgraded": out.Upgraded,
		"level": out.Level, "rarity": out.Rarity, "rate": out.Rate, "gold": out.Gold,
		"spent":          spentDetail,
		"payload_offset": r.PayloadOffset, "request_hex": hex.EncodeToString(p)})
	return w.appendFameUpdate(plan, event), nil
}

// bagSlotOfTemplate 找出某模板在背包里的第一个槽位（增量行刷新用）。
func bagSlotOfTemplate(bag inventory.Bag, template uint32) (uint16, bool) {
	for _, item := range bag.Items {
		if item.Template == template {
			return item.Slot, true
		}
	}
	return 0, false
}
