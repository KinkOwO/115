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
	// 宠物幻化槽（穿戴槽 32）的按槽绑定值：生物实例 key。它和 NOTI105 里那条生物
	// 条目必须同源（都用 inventory.CreatureSkinKey），否则客户端对不上。
	var skinModel uint32
	skinSlot := byte(inventory.CreatureSkinSlot)
	for _, item := range state.Inventory.Worn {
		if item.Slot >= 48 {
			return nil, fmt.Errorf("invalid worn appearance slot")
		}
		slot := byte(item.Slot)
		if item.Group == 0 {
			if _, exists := bySlot[slot]; !exists {
				bySlot[slot] = item.Template
				if slot == skinSlot {
					skinModel = inventory.CreatureSkinKey(item)
				}
			}
		} else if item.Group == 1 {
			// Look avatar overrides clone avatar for visual paper doll
			bySlot[slot] = item.Template
			if slot == skinSlot {
				skinModel = inventory.CreatureSkinKey(item)
			}
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
		row := protocol.Equipment{Slot: slot, Item: itemID}
		if slot == skinSlot {
			row.Model = skinModel
		}
		rows = append(rows, row)
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].Slot < rows[j].Slot })
	return rows, nil
}

// wornCreature 给出 mode-0 生物段要显示的生物：**身份**取穿戴槽 26（宠物本体），
// **模板**在有宠物幻化栏（穿戴槽 32，inventory.CreatureSkinSlot）时取幻化栏里的那只。
//
// 为什么幻化栏必须顶替槽 26 的模板：mode-0 生物段是客户端拿到「显示哪只生物」的
// **唯一**下行通道 —— u32 item_id + dstr name + u8 present，客户端读取器
// 0x1456394b0 把它写进 actor+0x504（名字写 +0x508，present 取反写 +0x518 的隐藏位）。
// 另外两条路都到不了这里：外观块 0x145639840 有 slot ≤ 25 的上限（槽 26/32 都超表，
// 见 protocol.maxEquippedAppearanceSlot），NOTI105（creature list）行里根本没有
// 模板字段（只有 key/饱食度/经验/等级/名字）。
//
// 实机 2026-09-26 取证：Charp(63003) 放进幻化栏后，存档里 worn slot 32 =
// {template 63003, key 3}，但服务端这次换装刷新发出的生物段仍是
// 63008(Botis) + "Botis" + present=1 —— 客户端收到的就是要显示 Botis，F6 预览的
// 模型因此不变（玩家报「宠物可以放进幻化栏了 但是宠物外观没有变」）。
//
// 名字仍取槽 26 的宠物名：幻化只换外观，本体宠物的名字/等级/饱食度都来自 NOTI105
// 里那条 key=1 的记录，不能跟着换。没有穿戴宠物时槽 32 不单独生效 —— 幻化栏是
// 外观覆盖，不是第二只宠物。
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
	var equipped, skin uint32
	var name string
	for _, item := range state.Inventory.Worn {
		switch item.Slot {
		case 26:
			if item.Template != 0 {
				equipped = item.Template
				name = inventory.CreatureDefaultNames[item.Template]
				if name == "" {
					name = "Creature"
				}
			}
		case inventory.CreatureSkinSlot:
			if item.Template != 0 {
				skin = item.Template
			}
		}
	}
	if equipped == 0 {
		return 0, ""
	}
	if skin != 0 {
		return skin, name
	}
	return equipped, name
}
