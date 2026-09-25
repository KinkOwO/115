package inventory

import (
	"dfolan/internal/game/protocol"
	"encoding/binary"
	"encoding/json"
	"fmt"
)

// Preserve server-owned instance bytes (quality, reinforcement, enchantments,
// sockets and other source effects) instead of zeroing them at every move.
func (i BagEquipment) ValidateRecord() error {
	if len(i.Record) != 0 && len(i.Record) != protocol.CurrentItemRecordSize {
		return fmt.Errorf("invalid equipment instance record")
	}
	if len(i.Record) > 0 && binary.LittleEndian.Uint32(i.Record[2:]) != i.Template {
		return fmt.Errorf("equipment instance template mismatch")
	}
	if len(i.AvatarOptions) > 4096 || len(i.AvatarSockets) > 4096 {
		return fmt.Errorf("equipment extension too large")
	}
	return nil
}

// secondsPerDay 把脚本里的天数换成客户端要的秒数。
const secondsPerDay = 86400

// creaturePeriodDays 由启动期注入：给定模板返回脚本声明的 `[usable period]`（天）。
// cmd/wireprobe 装载全量装备目录后调 SetCreaturePeriodSource 安装；未安装时
// declaredCreaturePeriodSeconds 返回 false，行为与修复前一致。
var creaturePeriodDays func(template uint32) (int32, bool)

// SetCreaturePeriodSource 安装生物期限查表。
func SetCreaturePeriodSource(fn func(template uint32) (int32, bool)) {
	creaturePeriodDays = fn
}

// declaredCreaturePeriodSeconds 把脚本声明的 `[usable period] N`（天）换成 N*86400 秒。
//
// 客户端把行内偏移 56 当「剩余秒数」，除以 86400 后用 `Expires in : %d Day(s)`
// （dstr 1057）渲染成「过期时间:N天」，不减当前时间。实机校准：写 2147483647 时
// 面板显示 24856 天 = round(2147483647/86400) = round(24855.6)。
//
// 只接受 (0, 36500] 天（≤100 年）：超出区间的值不是真实期限，不能拿它覆盖
// 既有的兼容兜底。
func declaredCreaturePeriodSeconds(template uint32) (uint32, bool) {
	if creaturePeriodDays == nil {
		return 0, false
	}
	days, ok := creaturePeriodDays(template)
	if !ok || days <= 0 || days > 36500 {
		return 0, false
	}
	return uint32(days) * secondsPerDay, true
}

// creatureRowPeriod 算出宠物行该写进行内偏移 56 的值：**剩余秒数**。
//
// 为什么宠物要单独补这一格：宠物（生物栏 space 7、穿戴槽 26~29）走的是**装备行**，
// 而装备行的偏移 56 一直没有写入者 —— `protocol.OrdinaryItem` 的 expireTime 只在
// 可叠加物那条路径上被填过（internal/inventory/bag.go 的 Bag.Rows 与
// internal/cashshop 的商城时效道具）。于是宠物这一格恒为 0，客户端显示「剩余期限已过」。
//
// 实机 2026-09-23：送的奥德赛宠物 100991331（`equipment/creature/100991331.equ`，
// 脚本里 `[usable period] 7`）在生物栏提示「剩余期限已过。」，存档里
// `special_equipment[7][0]` 只有 slot/template/durability、没有 period —— 就是这一格。
// 只补哨兵值能不再报过期，但面板会变成「过期时间:24856天」，所以改成按脚本真值算。
//
// 取值优先级：服务端自己记过的期限（BagEquipment.Period，例如装扮礼盒那条路径）
// > 行里已有的非零值（Record 里保留的服务端实例字节，不覆盖）
// > 脚本声明的 `[usable period]` 换算成秒
// > MaxItemPeriod（脚本不声明期限时的兼容兜底，该行文案不渲染）。
func creatureRowPeriod(template, stored, existing uint32) uint32 {
	if stored != 0 {
		return stored
	}
	if existing != 0 {
		return existing
	}
	if secs, ok := declaredCreaturePeriodSeconds(template); ok {
		return secs
	}
	return protocol.MaxItemPeriod
}

// Native NOTI14 1452e9b65/85 reads two length-prefixed avatar blocks;
// 1452e9ba2 reads their period. NOTI13 adds a period for every worn row.
func EquipmentPayload(space byte, items []BagEquipment, restore bool) ([]byte, error) {
	if space != 0 && space != 1 && space != 3 && space != 7 {
		return nil, fmt.Errorf("unsupported equipment space")
	}
	if len(items) > 65535 {
		return nil, fmt.Errorf("too many equipment rows")
	}
	p := []byte{space}
	u16 := func(n uint16) { p = binary.LittleEndian.AppendUint16(p, n) }
	u32 := func(n uint32) { p = binary.LittleEndian.AppendUint32(p, n) }
	if restore && (space == 0 || space == 1) {
		u16(0)
	}
	u16(uint16(len(items)))
	seen := map[uint16]bool{}
	for _, i := range items {
		if e := i.ValidateRecord(); e != nil {
			return nil, e
		}
		if seen[i.Slot] {
			return nil, fmt.Errorf("duplicate equipment slot")
		}
		seen[i.Slot] = true
		row := EquipmentRow(i)
		if (space == 3 && i.Slot == 26) || (space == 7 && i.Slot < 140) {
			key := uint32(1)
			if space == 7 {
				key = uint32(i.Slot + 2)
			}
			if k := binary.LittleEndian.Uint32(row[6:10]); k != 0 {
				key = k
			}
			binary.LittleEndian.PutUint32(row[6:10], key)
			binary.LittleEndian.PutUint32(row[24:28], key)
		}
		// 宠物行的期限（见 creatureRowPeriod）。只动宠物行：装扮的期限走行尾那个
		// u32、普通装备没有期限语义，两者都不在这次修复范围内。
		if space == 7 || (space == 3 && i.Slot >= 26 && i.Slot <= 29) {
			expiry := creatureRowPeriod(i.Template, i.Period, binary.LittleEndian.Uint32(row[56:60]))
			binary.LittleEndian.PutUint32(row[56:], protocol.ItemPeriodForWire(i.Template, expiry))
		}
		p = append(p, row[:]...)
		avatar := space == 1 || (space == 3 && i.Slot <= 11 && i.Template != 0)
		if avatar {
			u32(uint32(len(i.AvatarOptions)))
			p = append(p, i.AvatarOptions...)
			u32(uint32(len(i.AvatarSockets)))
			p = append(p, i.AvatarSockets...)
		}
		if (restore && (space == 1 || space == 3)) || (!restore && avatar && i.Template != 0) {
			u32(protocol.ItemPeriodForWire(i.Template, i.Period))
		}
	}
	return p, nil
}
func SpecialEquipmentPayload(state json.RawMessage, space byte) ([]byte, error) {
	b, e := ReadBag(state)
	if e != nil {
		return nil, e
	}
	if len(b.Special[space]) == 0 {
		return nil, nil
	}
	return EquipmentPayload(space, b.Special[space], false)
}

// SpecialEquipmentRestorePayload builds a complete NOTI13 container snapshot,
// including an explicit zero-row container. The client needs the empty avatar
// prefix during town bootstrap and the full avatar snapshot after initialization.
func SpecialEquipmentRestorePayload(state json.RawMessage, space byte) ([]byte, error) {
	b, e := ReadBag(state)
	if e != nil {
		return nil, e
	}
	return EquipmentPayload(space, b.Special[space], true)
}
