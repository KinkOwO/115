package main

import (
	"dfolan/internal/game/protocol"
	"dfolan/internal/inventory"
	"encoding/json"
)

// bagRowOrEmpty 取背包里某一格的行；**该格已经被用光（整行移除）时返回空行**
// （槽位 + 模板 0xFFFFFFFF）。
//
// ★ 为什么必须这样：客户端只按「到达的那一行」更新那一格。整行移除后如果
// 什么都不发，客户端会一直保留旧图标，直到玩家整理背包 / 拖动别的物品触发一次
// 整包重算才消失。实机反馈（2026-09-28）：保护券**只有一张**时，失败扣减后图标
// 还在，点整理背包才没（多张时因为 RowAt 仍能取到新数量的行，所以看不出来）。
//
// 凡是「这次结算可能把这一格用光」的行刷新，都必须走这个函数 ——
// 别再手写 `row, ok := bag.RowAt(slot); if ok { ... }`，那一定会漏掉整行移除这一支。
// 强化、增幅、锻造和附魔通过 equipmentRows 复用此规则。
func bagRowOrEmpty(bag inventory.Bag, slot uint16) [protocol.CurrentItemRecordSize]byte {
	if row, ok := bag.RowAt(slot); ok {
		return row
	}
	return protocol.EmptyOrdinaryItem(slot)
}

func equipmentRows(bag inventory.Bag, space byte, equipmentSlot uint16, slots ...uint16) [][protocol.CurrentItemRecordSize]byte {
	rows := make([][protocol.CurrentItemRecordSize]byte, 0, len(slots)+1)
	for _, slot := range slots {
		rows = append(rows, bagRowOrEmpty(bag, slot))
	}
	if space == 0 {
		rows = append(rows, bagRowOrEmpty(bag, equipmentSlot))
	}
	return rows
}

// Callers place the result acknowledgement before or after these updates.
func appendEquipmentUpdates(plan []outboundPacket, state json.RawMessage, rows [][protocol.CurrentItemRecordSize]byte, space byte, inventoryName, wornName string) ([]outboundPacket, error) {
	body, err := protocol.InventoryUpdate(rows)
	if err != nil {
		return nil, err
	}
	plan = append(plan, outboundPacket{inventoryName, 0, 14, body})
	if space == 3 {
		body, err = inventory.WornSpaceUpdate(state)
		if err != nil {
			return nil, err
		}
		if len(body) > 0 {
			plan = append(plan, outboundPacket{wornName, 0, 14, body})
		}
	}
	return plan, nil
}
