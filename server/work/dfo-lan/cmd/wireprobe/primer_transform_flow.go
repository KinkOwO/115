package main

import (
	"context"
	"dfolan/internal/game/protocol"
	"dfolan/internal/inventory"
	"dfolan/internal/workflow"
	"log"
	"time"
)

// 装备库「誓约 / 晶体变换」（CMD2381 ENUM_CMDPACKET_PRIMER_TRANSFORM）。
//
// 协议几何与语义见：
//   - internal/game/protocol/primer_transform.go（C2S 203 字节布局 + 7 字节应答，IDA 定案）
//   - internal/inventory/primer_transform.go（执行侧：行 0 = 槽 47、记录 i = 槽 36+i、成本/返还）
//   - internal/catalog/equipment_transform_system.go（费用/返还真源）
//
// 应答形状与 2259 **逐字节同形**（handler `sub_145287A40` 读同样的 6 字节：`payload[4]`
// 选窗口 3937 / 2145，`payload[5]` 只在失败分支被读）。
//
// ★ 窗口必须发 **0**（→ 窗口 2145 = EquipmentTransformWindow）：只有那一支会
// `setState(3)` → 弹 DSTR 101039076「Oath/Crystal conversion completed! Moving to the
// Oath settings.」并切到窗口 2144。发 1 会走另一簇窗口（3937，弹的是"Extraction complete."），
// 玩家看到的就是"点了没反应"。
//
// ⚠️ 与 2259 同一条硬纪律：**失败分支绝不回 Error 包**。客户端在这条链上没有失败分支，
// 2259 两次收到 Error 都以 `exit=0xC0000005` 结束。所以无论成败都回"窗口应答"，
// 只把拒因写进 events.jsonl。
//
// 开关（默认按"真的变换"跑）：
//
//	-primer-transform / DFO_PRIMER_TRANSFORM_APPLY = apply | observe
//	-primer-transform-window / DFO_PRIMER_TRANSFORM_WINDOW（默认 0 → 窗口 2145）
//	-primer-transform-variant / DFO_PRIMER_TRANSFORM_VARIANT（默认 0，成功时不用）
var (
	primerTransformWindow  byte = 0
	primerTransformVariant byte = 0
)

// primerTransformApply 决定是否真的执行变换（apply | observe）。
var primerTransformApply = "apply"

// primerTransformPayOptionDefault 是取不到线上字段时的兜底支付组号
// （源 `[need primer materials]` 的 `[group] N`：1 = 金币、2 = 巡礼之印 `10401346`）。
const primerTransformPayOptionDefault = 1

// primerTransformPayOption 取本次变换的支付组号。
//
// ★ **已闭环（2026-10-04，IDA 指令级）**：正文 `+13` **就是支付组号**，不是窗口常量 ——
//
//	写入    0x14150c8cb：`mov eax,[rbx+19E4h]` → `mov [var_203],eax`（var_203 = 正文 +13）
//	来源    `win+0x19E4`：构造器 `0x1414f30fb` 置 1；玩家点 `win+0x12E0` 上的开关时由
//	        `sub_141503B70` 写成 `(旧值 == 1) + 1`（写入点 `0x141503bba`）⇒ 1 ↔ 2 切换
//	唯一性  0x1414E0000–0x141530000 全函数逐指令位移扫描里，`win+0x19E4` 的写者**只有**
//	        构造器与 `sub_141503B70` 两处，其余 56 处全是读/比较
//	同义    2259 与 2381 由**同一个发包器、同一条指令**写同一字段 ⇒ 两边同义，不存在"只给 2259 用"
//
// 取值口径（客户端 `.cos` 解析器 `sub_14769EAF0` 直接读 `[group]` 列编号并原样上传）：
// **1 = 金币那一支、2 = 巡礼之印那一支**。所以这里直接采信 1..2；域外值（理论上不该出现）
// 回落 `primerTransformPayOptionDefault` 并记日志，便于实机时一眼发现异常。
// 证据与产物索引见 docs/protocol/primer-transform-20261003.md §4.3。
func primerTransformPayOption(r protocol.PrimerTransformRequest) int {
	if r.WindowKey >= 1 && r.WindowKey <= 2 {
		return int(r.WindowKey)
	}
	log.Printf("primer transform: 支付组号(+13)=%d 不在 1..2，按 %d（金币）处理",
		r.WindowKey, primerTransformPayOptionDefault)
	return primerTransformPayOptionDefault
}

func (w *worldSession) primerTransform(p []byte, event func(map[string]any)) ([]outboundPacket, error) {
	plan := []outboundPacket{{
		Name:    "primer_transform_opened",
		Kind:    1,
		ID:      protocol.PrimerTransformOpcode,
		Payload: protocol.PrimerTransformReply(primerTransformWindow, primerTransformVariant),
	}}
	if w == nil {
		return plan, nil
	}
	r, e := protocol.DecodePrimerTransformRequest(p)
	if e != nil {
		log.Printf("primer transform DECODE-REFUSED: %v", e)
		if event != nil {
			event(map[string]any{"kind": "primer_transform_decode_refused", "reason": e.Error()})
		}
		return plan, nil
	}
	indexes, entries, templates := r.Wanted()
	log.Printf("primer transform: window_key=%d select_id=%d tail=%d caller=%d oath={space=%d slot=%d template=%d extra=%d row=%d} rows=%v templates=%v spaces=%v slots=%v",
		r.WindowKey, r.SelectID, r.Tail, r.CallerArg,
		r.Oath.Space, r.Oath.Slot, r.Oath.Template, r.Oath.Extra, r.Oath.RowIndex,
		indexes, templates, entrySpaces(entries), entrySlots(entries))
	if len(templates) == 0 && r.Oath.Empty() {
		log.Printf("primer transform NOTHING-REQUESTED: 行 0 与 11 条记录都是空的")
		if event != nil {
			event(map[string]any{"kind": "primer_transform_empty", "character_id": w.role.ID})
		}
		return plan, nil
	}
	slots := primerTransformSlots(indexes)
	payOption := primerTransformPayOption(r)
	if primerTransformApply == "observe" {
		log.Printf("primer transform PLAN (observe): slots=%v templates=%v oath=%d pay_option=%d",
			slots, templates, r.Oath.Template, payOption)
		if event != nil {
			event(map[string]any{"kind": "primer_transform_planned", "character_id": w.role.ID,
				"slots": slots, "templates": templates, "oath": r.Oath.Template,
				"pay_option": payOption})
		}
		return plan, nil
	}
	if w.items == nil || w.store == nil || w.role.ID == 0 {
		log.Printf("primer transform REFUSED: item service unavailable")
		return plan, nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	saved, receipt, applied, e := (&workflow.ItemService{Store: w.store, Items: w.items}).
		TransformPrimers(ctx, w.role, r, payOption)
	if e != nil {
		log.Printf("primer transform REFUSED: %v", e)
		if event != nil {
			event(map[string]any{"kind": "primer_transform_refused", "character_id": w.role.ID,
				"slots": slots, "templates": templates, "oath": r.Oath.Template, "reason": e.Error()})
		}
		return plan, nil
	}
	if applied {
		w.role = saved
	}
	log.Printf("primer transform DONE: pairs=%d gold=%d option=%d materials=%v refunds=%v skipped=%v applied=%t",
		len(receipt.Pairs), receipt.Gold, receipt.Option, receipt.Materials, receipt.Refunds, receipt.Skipped, applied)
	if event != nil {
		event(map[string]any{"kind": "primer_transform_done", "character_id": w.role.ID,
			"pairs": receipt.Pairs, "gold": receipt.Gold, "option": receipt.Option,
			"materials": receipt.Materials, "refunds": receipt.Refunds,
			"skipped": receipt.Skipped, "applied": applied, "source": receipt.Source})
	}
	if !applied {
		// 幂等重放：状态没变、也没再扣一次，**不重复刷新**（与 2259 同一条纪律）。
		return plan, nil
	}
	plan = append(plan, w.primerTransformRefresh(ctx, oathCoreChanged(receipt))...)
	return plan, nil
}

// primerTransformSlots 把记录位次翻成穿戴槽（36+i）；越界位次直接丢弃（协议层已限 11 条）。
func primerTransformSlots(indexes []int) []uint32 {
	out := make([]uint32, 0, len(indexes))
	for _, i := range indexes {
		if i < 0 || i >= inventory.PrimerCrystalSlotCount {
			continue
		}
		out = append(out, uint32(inventory.PrimerCrystalSlotBase+i))
	}
	return out
}

func entrySpaces(entries []protocol.PrimerTransformEntry) []uint32 {
	out := make([]uint32, 0, len(entries))
	for _, e := range entries {
		out = append(out, uint32(e.Space))
	}
	return out
}

func entrySlots(entries []protocol.PrimerTransformEntry) []uint32 {
	out := make([]uint32, 0, len(entries))
	for _, e := range entries {
		out = append(out, uint32(e.Slot))
	}
	return out
}

// oathCoreChanged 报告这次变换是否换掉了誓约核心（槽 47）。
//
// 换了核心就必须补发 NOTI2839（OATH_SYSTEM_INFO，handler 读 15 字节）：oath 选项账本
// 是**按穿戴核心**存的（storage.character_oath_options 的 core_instance_key），
// 客户端随后会打开 Oath settings（2144）并按新核心重选。
func oathCoreChanged(receipt inventory.PrimerTransformReceipt) bool {
	for _, p := range receipt.Pairs {
		if p.Slot == inventory.PrimerOathSlot {
			return true
		}
	}
	return false
}

// primerTransformRefresh 组变换后的刷新下发。
//
// 晶体/核心进的是**穿戴栏**（NOTI14 走 `WornSpaceUpdate`），成本/返还动到账号材料仓
// （`accountMaterialRefreshPackets` 的 list35 → list42 → list0），被换下去的旧件会改装备库
// 账本（补 NOTI2610）。handler 自己不回请任何包（**只发这些刷新**），所以这三处缺一不可。
func (w *worldSession) primerTransformRefresh(ctx context.Context, oathChanged bool) []outboundPacket {
	var plan []outboundPacket
	if wornBody, e := inventory.WornSpaceUpdate(w.role.State); e == nil && len(wornBody) > 0 {
		plan = append(plan, outboundPacket{"primer_transform_worn_refreshed", 0, 14, wornBody})
	}
	if bag, e := inventory.ReadBag(w.role.State); e == nil {
		if body, e := protocol.InventoryRestore(bag.Rows(), bag.Expansion); e == nil {
			plan = append(plan, outboundPacket{"primer_transform_inventory_refreshed", 0, 13, body})
		}
	}
	if body, e := equipmentJournalEntryPayload(w.role, w.journalRules()); e == nil && len(body) > 0 {
		plan = append(plan, outboundPacket{"primer_transform_journal_refreshed", 0, protocol.EquipmentJournalOpcode, body})
	} else if e != nil {
		log.Printf("primer transform: build journal snapshot: %v", e)
	}
	if oathChanged {
		ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()
		selection, e := w.store.EquippedOathSelection(ctx, w.account, w.role.ID)
		if e != nil {
			log.Printf("primer transform: oath selection after core change: %v", e)
		} else if info, e := protocol.OathSystemInfo(selection.Level, selection.Option); e != nil {
			log.Printf("primer transform: oath info after core change: %v", e)
		} else {
			plan = append(plan, outboundPacket{"primer_transform_oath_info", 0, 2839, info})
		}
	}
	accountRaw, e := w.store.AccountMaterials(ctx, w.role.AccountID)
	if e != nil {
		log.Printf("primer transform: read account materials after transform: %v", e)
		return plan
	}
	account, e := inventory.ReadAccountMaterials(accountRaw)
	if e != nil {
		log.Printf("primer transform: decode account materials after transform: %v", e)
		return plan
	}
	refresh, e := accountMaterialRefreshPackets(account, w.role)
	if e != nil {
		log.Printf("primer transform: build account material refresh: %v", e)
		return plan
	}
	for _, p := range refresh {
		p.Name = "primer_transform_" + p.Name
		plan = append(plan, p)
	}
	return plan
}
