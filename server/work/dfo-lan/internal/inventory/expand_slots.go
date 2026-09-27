package inventory

import (
	"encoding/json"
)

// Extended equipment-slot unlock bits, as stored in Bag.ExpandEquipFlags.
//
// Two paths award them: quest [slot expansion] rewards (internal/quest) and
// Odyssey dungeon clears (internal/character, see OdysseyExpandEquipMask).
// USERINFO1 / EntryAddition then projects the byte back to the armoury padlocks.
// Slot numbers: support 22, magic stone 23, earring 25.
const (
	ExpandSupport    byte = 1 << 0 // equipment slot 22
	ExpandMagicStone byte = 1 << 1 // equipment slot 23
	ExpandEarring    byte = 1 << 4 // equipment slot 25
	// ExpandCreatureSkin 是宠物幻化栏（穿戴槽 32）的开启位。
	ExpandCreatureSkin byte = 1 << 5
)

// CreatureSkinSlot 是宠物幻化栏在穿戴容器（list 3）里的槽号。
//
// 实机 2026-09-26 全量会话统计：客户端对 dst >= 26 只用过 26/27/28/29/32，
// 其中 32 只出现在 [creature] 本体（63003/63008）上，没有别的用途。
const CreatureSkinSlot uint16 = 32

// CreatureSkinFallbackKey 是幻化槽宠物在存档里没有实例 key 时用的兜底 key。
//
// 取值沿用宠物栏（list 7）的约定 key = 槽位+2，只是这里按穿戴槽号 32 算，得到 34。
// 关键是**不能沿用宠物本体的 1**：槽 26 与槽 32 会共享同一个 key，客户端按 key
// 归并生物对象时会把本体那只挤掉，且 CreatureListPayload 的去重分支会把后看到的
// 那条改成 len(seenKeys)+10，导致 list 3 的行与 NOTI105 的条目对不上。
//
// 实机上幻化槽宠物的 key 是它还在宠物栏时写进实例字节的那个（例如 3，见
// equipment_record.go 的归一），这个常量只在存档缺 key 时兜底。
const CreatureSkinFallbackKey uint32 = uint32(CreatureSkinSlot) + 2

// CreatureSkinTicket 是宠物幻化栏扩展券的模板 id（源 stackable/10309001/10309084.stk
// 的 [open creature skin slot]）。
const CreatureSkinTicket uint32 = 10309084

// SkinSlotMaskForTicket 把一张幻化栏扩展券模板映射到它开启的那一位。
// 客户端用同一个 USERINFO1 解锁字节驱动装备栏挂锁与幻化栏
// （原生 145f02500 读 bit5），所以券只对应位图里的一个 bit。
func SkinSlotMaskForTicket(template uint32) (byte, bool) {
	if template == CreatureSkinTicket {
		return ExpandCreatureSkin, true
	}
	return 0, false
}

// SkinSlotTicketSlot 找出主背包里能开启 mask 那一栏的扩展券所在格子。
//
// 客户端不报券在哪一格，所以只能用券的模板反查。
func SkinSlotTicketSlot(b Bag, mask byte) (uint16, bool) {
	for _, item := range b.Items {
		if m, ok := SkinSlotMaskForTicket(item.Template); ok && m == mask {
			return item.Slot, true
		}
	}
	return 0, false
}

// UnlockEquipSlots accumulates extended equipment-slot unlock bits into the
// saved bag and returns the rewritten character state.
//
// The mask is OR-ed into a byte the armoury rebuilds its padlocks from, so
// repeated awards and out-of-order quest completion are both safe: finishing
// 650 before 649 leaves 3, and a third award leaves 19. A zero mask is a
// no-op and returns the state untouched rather than rewriting it.
//
// ReadBag/SaveBag bracket the change the same way Awarder.Grant does, so the
// new bits land in the caller's character transaction and roll back with it.
func UnlockEquipSlots(raw json.RawMessage, mask byte) (json.RawMessage, error) {
	if mask == 0 {
		return raw, nil
	}
	b, e := ReadBag(raw)
	if e != nil {
		return nil, e
	}
	b.ExpandEquipFlags |= mask
	return SaveBag(raw, b)
}
