package character

import (
	"dfolan/internal/game/protocol"
	"dfolan/internal/inventory"
	"encoding/json"
	"fmt"
	"sort"
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
			Worn []inventory.BagEquipment `json:"worn"`
			// WeaponSkin 是幻化仓库里应用的武器外观（皮肤 id）。入场这条投影把
			// 每行的模板写进装备外观块的 Placeholder，而城镇模型查找（145BEFD60
			// → 145BD63D0 → 145BEE6C0）正是读那个字段，所以覆盖它才能让重登后
			// 仍然显示幻化外观。
			WeaponSkin uint32 `json:"weapon_skin"`
		} `json:"inventory"`
	}
	if err := json.Unmarshal(raw, &state); err != nil {
		return nil, err
	}
	bySlot := make(map[byte]uint32)
	for _, item := range state.Inventory.Worn {
		if item.Slot >= 48 {
			return nil, fmt.Errorf("invalid worn appearance slot")
		}
		slot := byte(item.Slot)
		if item.Group == 0 {
			if _, exists := bySlot[slot]; !exists {
				bySlot[slot] = item.Template
			}
		} else if item.Group == 1 {
			// Look avatar overrides clone avatar for visual paper doll
			bySlot[slot] = item.Template
		}
	}
	// 武器幻化：这条投影的 Item 由 EquipmentAppearance 写进装备外观块的 Placeholder，
	// 而城镇模型查找（145BEFD60 → 145BD63D0 → 145BEE6C0）读的正是它，所以重登后仍要
	// 显示幻化外观，就必须把槽 12 的武器模板换成应用的皮肤 id。只在槽 12 本来就有武器
	// 时覆盖：空武器槽凭空补一行，客户端会给角色装上一把并不存在的武器。
	if state.Inventory.WeaponSkin != 0 {
		if _, worn := bySlot[byte(inventory.WeaponSlot)]; worn {
			bySlot[byte(inventory.WeaponSlot)] = state.Inventory.WeaponSkin
		}
	}
	var rows []protocol.Equipment
	for slot, itemID := range bySlot {
		rows = append(rows, protocol.Equipment{Slot: slot, Item: itemID})
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].Slot < rows[j].Slot })
	return rows, nil
}

func wornCreature(raw json.RawMessage) (uint32, string) {
	var state struct {
		Inventory struct {
			Worn []struct {
				Slot     uint16 `json:"slot"`
				Template uint32 `json:"template"`
			} `json:"worn"`
		} `json:"inventory"`
	}
	if err := json.Unmarshal(raw, &state); err != nil {
		return 0, ""
	}
	for _, item := range state.Inventory.Worn {
		if item.Slot == 26 && item.Template != 0 {
			name := inventory.CreatureDefaultNames[item.Template]
			if name == "" {
				name = "Creature"
			}
			return item.Template, name
		}
	}
	return 0, ""
}
