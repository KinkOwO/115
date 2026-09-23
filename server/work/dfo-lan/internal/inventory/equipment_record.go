package inventory

import (
	"dfolan/internal/game/protocol"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"math"
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
		// 单机裁定（2026-09-23）：宠物/特殊装备不设期限语义。客户端对带
		// [usable period] 的限时宠物模板用行内 offset 56（过期时间戳）计算
		// 剩余期限；服务端不发限时，统一按永久（MaxInt32）下发，避免客户端
		// 判"剩余期限已过 / 无法交易或使用"。（堆叠物同字段 MaxInt32=永久
		// 已在商城路径实机验证；宠物面板能正常渲染说明 space7 行无行尾 period。）
		if space == 7 || (space == 3 && i.Slot == 26) {
			binary.LittleEndian.PutUint32(row[56:], math.MaxInt32)
		}
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
		p = append(p, row[:]...)
		avatar := space == 1 || (space == 3 && i.Slot <= 11 && i.Template != 0)
		if avatar {
			u32(uint32(len(i.AvatarOptions)))
			p = append(p, i.AvatarOptions...)
			u32(uint32(len(i.AvatarSockets)))
			p = append(p, i.AvatarSockets...)
		}
		if (restore && (space == 1 || space == 3)) || (!restore && avatar && i.Template != 0) {
			u32(i.Period)
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
