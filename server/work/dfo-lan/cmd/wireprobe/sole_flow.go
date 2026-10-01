package main

import (
	"context"
	"dfolan/internal/game/protocol"
	"dfolan/internal/inventory"
	"encoding/hex"
	"time"
)

// CMD2288 秘宝精度提升（`ENUM_CMDPACKET_SOLE_EQUIPMENT_QUALITY`）。
//
// 协议与规则来源见：
//   - internal/game/protocol/sole.go（24 字节请求 / kind 1 回包，实机与交接书证据在注释里）
//   - internal/catalog/sole_equipment.go（直读 etc/115lvability/soleequipmentsystem.cos）
//
// 回包顺序（照 AI 交接书 §2.3 的硬约束）：
//
//	1. {sole_quality_ack, kind=1, 2288, 01|container|slot}   ← **必须首包**
//	   客户端在「期望回包树」里登记了 2288，收不到同 id 包就清不掉等待态 ⇒ 精度窗口卡死
//	   （交接书症状 1：提升一次后窗口无响应、需关窗重开）。
//	2. 账号共享材料被扣 → accountMaterialRefreshPackets（list35 → list42 → list0，
//	   **list35 必须早于 list0**：list0 的读者会把 363..379 重新收进账号仓库管线）。
//	3. 背包材料 / 装备本体行 → id14 增量行（appendEquipmentUpdates 按容器分流：
//	   容器 0 用背包行、容器 3 整体刷新穿戴空间）。
//	4. 名望（精度直接进名望结算）→ appendFameUpdate。
//
// 失败分支：**仍然发同一个 2288 回包**（状态 1、回显客户端报的 container/slot）。
// 理由：客户端不发回包就卡在等待态，而 2288 的失败语义（状态 0 + u16 错误码）**没有任何
// 反编译依据**（交接书 §8）—— 发一个"无依据的失败码"风险更大；发回显包后客户端会刷新
// 精度条，玩家看到的是"数值没变"，与失败观感一致。真实原因写在服务端
// `sole_quality_refused` 事件里。
func (s *equipmentSession) raiseSoleQuality(service *inventory.WearService, w *worldSession, p, raw []byte, event func(map[string]any)) ([]outboundPacket, error) {
	if service == nil || w == nil || w.role.ID == 0 || w.activeDungeon != nil {
		return nil, inventory.Refuse(inventory.RefusalItems, "秘宝精度提升需要已选角色且位于城镇")
	}
	r, err := protocol.DecodeSoleQuality(p)
	if err != nil {
		return nil, err
	}
	key, err := s.requestKey(raw)
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	saved, out, err := service.ApplySoleQuality(ctx, w.role, "sole-quality:"+key, r)
	if err != nil {
		// 失败也要回同 id 包，否则客户端精度窗口会卡在等待态（见文件头注释）。
		event(map[string]any{"kind": "sole_quality_refused", "character_id": w.role.ID,
			"reason": err.Error(), "container": r.Container, "slot": r.Slot,
			"request_hex": hex.EncodeToString(p), "payload_offset": r.PayloadOffset})
		return []outboundPacket{{"sole_quality_ack_failed", 1, protocol.SoleQualityOpcode,
			protocol.SoleQualityReply(r.Container, r.Slot)}}, err
	}
	w.role = saved

	plan := []outboundPacket{{"sole_quality_ack", 1, protocol.SoleQualityOpcode,
		protocol.SoleQualityReply(out.Container, out.Slot)}}

	bag, err := inventory.ReadBag(saved.State)
	if err != nil {
		return plan, err
	}
	var rows [][protocol.CurrentItemRecordSize]byte
	storageTouched := false
	for _, spent := range out.Spent {
		if spent.Template == 0 {
			continue // 金币行在下面统一处理
		}
		if spent.StorageAmount > 0 {
			storageTouched = true
		}
		// ⚠️ **不因为"这个模板属于账号材料品类"就跳过背包行**：账号材料被清扫进仓库之前
		// 就是以背包堆叠存在的，那时被扣掉的是背包那一份 —— 2026-10-02 实机踩过：
		// 灵魂（10361515）被判定为账号材料后直接 `continue`，结果客户端材料面板数量不变，
		// 玩家看到的是"材料没扣"（DB 里其实扣了）。
		if slot, ok := bagSlotOfTemplate(bag, spent.Template); ok {
			rows = append(rows, bagRowOrEmpty(bag, slot))
		}
	}
	if out.Spent != nil {
		// 扣过金币就刷新金币行（与装备调适同一约定）。
		if goldRow, ok := bag.RowAt(0); ok {
			rows = append(rows, goldRow)
		}
	}
	if out.Container == 0 {
		rows = append(rows, bagRowOrEmpty(bag, out.Slot))
	}
	// ⚠️ **订单关键**：账号材料刷新（list35 → list42 → **list0**）必须排在 id14 增量行**之前**。
	// `list0` 是整仓快照，它的读者会把 363..379 重新收进账号材料管线 —— 放在 id14 之后会把
	// 刚发出的背包行数量盖回旧值（现象：DB 扣了、客户端数量不变）。
	if storageTouched {
		if counts, e := service.Store.AccountMaterials(ctx, saved.AccountID); e == nil {
			if materials, e2 := inventory.ReadAccountMaterials(counts); e2 == nil {
				refresh, e3 := accountMaterialRefreshPackets(materials, saved)
				if e3 == nil {
					plan = append(plan, refresh...)
				}
			}
		}
	}
	plan, err = appendEquipmentUpdates(plan, saved.State, rows, out.Container,
		"sole_quality_inventory", "sole_quality_worn")
	if err != nil {
		return plan, err
	}
	spentDetail := make([]map[string]any, 0, len(out.Spent))
	for _, spent := range out.Spent {
		spentDetail = append(spentDetail, map[string]any{
			"template": spent.Template, "amount": spent.Amount,
			"from_storage": spent.FromStorage, "storage_amount": spent.StorageAmount})
	}
	event(map[string]any{"kind": "sole_quality_committed", "character_id": saved.ID,
		"template": out.Template, "container": out.Container, "slot": out.Slot,
		"selector": out.Request.Selector, "group_index": out.GroupIndex,
		"quality_before": out.QualityBefore, "quality_after": out.QualityAfter,
		"quality_gain": out.QualityGain, "max_quality": out.MaxQuality,
		"record_healed": out.RecordHealed, "gold": out.Gold, "spent": spentDetail,
		"payload_offset": r.PayloadOffset, "request_hex": hex.EncodeToString(p)})
	return w.appendFameUpdate(plan, event), nil
}
