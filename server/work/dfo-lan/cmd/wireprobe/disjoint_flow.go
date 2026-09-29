package main

import (
	"context"
	"dfolan/internal/game/protocol"
	"fmt"
	"time"
)

// disjointItem answers CMD26 (ENUM_CMDPACKET_DISJOINT_ITEM).
// It deletes requested equipment, awards clear cube fragments into the
// account material storage (they never stay in the bag: the 115 client pins
// the seventeen shared materials to account list 35), returns the ACK with
// deleted slots and rewards, then republishes the authoritative list35
// storage snapshot followed by the list0 bag snapshot so the client harvest
// moves the stacks into the soul-storage panel.
//
// CMD26 同时就是客户端的「**装备库添加**」（规格 `CMD/0026-DISJOINTITEM`）：收录与
// "扣装备 / 发材料"在同一个事务里落库（见 loot.Disjoint）。所以成功之后必须**补发一次
// NOTI2610 权威快照** —— 客户端的图鉴计数只认 2610，只在入场发的话，玩家当场分解完
// 什么都看不到（实机 2026-09-30 玩家报告："分解没进图鉴"），得重登才刷新。
func (w *worldSession) disjointItem(p []byte, event func(map[string]any)) ([]outboundPacket, error) {
	if w == nil || w.role.ID == 0 || w.loot == nil {
		return nil, fmt.Errorf("disjoint service unavailable")
	}
	r, e := protocol.DecodeDisjointItem(p)
	if e != nil {
		return nil, e
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	saved, receipt, _, e := w.loot.Disjoint(ctx, w.role, r)
	if e != nil {
		return nil, e
	}
	// The disjoint reward templates are account-shared materials; move them
	// out of the bag into the account storage before any snapshot is built.
	saved, materials, e := sweepAccountMaterials(ctx, w.loot.Store, saved)
	if e != nil {
		return nil, e
	}
	for i := range receipt.Rewards {
		receipt.Rewards[i].Slot = storageDestinationSlot(receipt.Rewards[i].Template, receipt.Rewards[i].Slot)
	}
	ack, e := protocol.DisjointItemSuccess(protocol.DisjointItemResult{
		DeletedSlots: receipt.DeletedSlots,
		List:         0,
		ToolSlot:     receipt.ToolSlot,
		Rewards:      receipt.Rewards,
	})
	if e != nil {
		return nil, e
	}
	refresh, e := accountMaterialRefreshPackets(materials, saved)
	if e != nil {
		return nil, e
	}
	w.role = saved
	plan := []outboundPacket{{"disjoint_item_ack", 1, 26, ack}}
	for _, p := range refresh {
		p.Name = "disjoint_item_" + p.Name
		plan = append(plan, p)
	}
	// 收录有变化时补发一次 2610：顺序照参考行为，排在「来源/奖励状态」之后。
	if len(receipt.JournalAdded) > 0 {
		if event != nil {
			event(map[string]any{
				"kind":         "disjoint_journal_added",
				"id":           26,
				"character_id": saved.ID,
				"added":        len(receipt.JournalAdded),
				"skipped":      len(receipt.JournalSkipped),
			})
		}
		body, jErr := equipmentJournalEntryPayload(saved, w.journalRules())
		if jErr != nil {
			// 组包失败**不能吞掉已经提交的分解**：ACK 与材料刷新照发，只把这次失败记进日志。
			// 玩家至少拿得到材料，重登仍会在入场拿到快照。
			if event != nil {
				event(map[string]any{"kind": "disjoint_journal_refresh_failed", "id": 26,
					"character_id": saved.ID, "reason": jErr.Error()})
			}
		} else if len(body) > 0 {
			plan = append(plan, outboundPacket{"equipment_journal_restored", 0, protocol.EquipmentJournalOpcode, body})
		}
	} else if len(receipt.JournalSkipped) > 0 && event != nil {
		// 一件都没登记上时也要留痕：这正是"分解了但图鉴没进"的排查入口。
		event(map[string]any{"kind": "disjoint_journal_skipped", "id": 26,
			"character_id": saved.ID, "skipped": len(receipt.JournalSkipped)})
	}
	return plan, nil
}
