package character

import (
	"dfolan/internal/game/protocol"
	"encoding/json"
	"fmt"
)

// wornAppearance projects the worn set onto the list-row block the client
// reads when it builds its equipment array.
//
// 更正（2026-09-19，实机否证）：此处曾按 0x145a8a780 返回的集过滤槽位，并把
// 该集读作「客户端接受的装备槽集合」。实机 client_trace 在入场时逐条记录
//
//	equip : 12 - <武器>(101010438)
//	equip : 14/15/16/17/18 - <防具>
//	equip : 24 - 骑士之盾(113370002)
//
// 且写出该日志的循环（0x145640b00）遍历 0x30=48 个槽、凡非空即打印——说明
// 14..25 的装备行客户端照单全收，槽号空间就是 [equipment type] 的序号空间
// （0x1470cb31e 起的枚举表：weapon=12, coat=14, shoulder=15, pants=16,
// shoes=17, waist=18, support weapon=24）。0x145a8a780 的键集
// {1..10,11,12,13,26,32} 是「装扮层」表——1..10 是 hair..weapon avatar、
// 11 是 aura skin avatar、12 是 weapon、13 是 title name、26 是 creature、
// 32 是 creature skin；身体装备（14..25）不在其中，其外观由物品对象自身驱动。
// 两个坐标系互不相交，用前者裁后者是错误前提，故该过滤已删除。
func wornAppearance(raw json.RawMessage) ([]protocol.Equipment, error) {
	var state struct {
		Inventory struct {
			Worn []struct {
				Slot     uint16 `json:"slot"`
				Template uint32 `json:"template"`
			} `json:"worn"`
		} `json:"inventory"`
	}
	if err := json.Unmarshal(raw, &state); err != nil {
		return nil, err
	}
	var rows []protocol.Equipment
	for _, item := range state.Inventory.Worn {
		if item.Slot >= 48 {
			return nil, fmt.Errorf("invalid worn appearance slot")
		}
		rows = append(rows, protocol.Equipment{Slot: byte(item.Slot), Item: item.Template})
	}
	return rows, nil
}
