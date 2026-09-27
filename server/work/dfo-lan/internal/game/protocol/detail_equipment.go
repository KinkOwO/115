package protocol

import (
	"encoding/binary"
	"fmt"
)

// DetailedWorn is the mode1/1452c1540 avatar representation, not NOTI13.
// Only avatar slots are initialized here; other item types have different
// template-dependent extensions. Rich records require a separate projection.
type DetailedWorn struct {
	Slot       uint16 `json:"slot"`
	Template   uint32 `json:"template"`
	Durability uint16 `json:"durability"`
	// HeaderTemplateA is the row+24 visual override. For ordinary avatars
	// (including clear avatars) the native reader writes it to the appearance
	// field. Only random clear avatars (item-category bit 25) use it as a
	// random-avatar template and then consume HeaderTemplateB at row+28.
	HeaderTemplateA uint32 `json:"header_template_a,omitempty"`
	HeaderTemplateB uint32 `json:"header_template_b,omitempty"`
	Period          uint32 `json:"period,omitempty"`
	AvatarOptions   []byte `json:"avatar_options,omitempty"`
	AvatarSockets   []byte `json:"avatar_sockets,omitempty"`
	Record          []byte `json:"record,omitempty"`
}

// Row layouts pinned instruction by instruction against the native mode-1
// equipment-block reader sub_1452C1540 (115us client, DFO.exe 2.38.2.34):
//
//	header 40B (all rows): u8 slot, u32 template, u32, u8, u16 durability,
//	  u8, u8, 10xu8, u32, u32, u32, u8, u8, u16
//	avatar rows (item type <= 11, dispatcher sub_1459A01E0/sub_1459A0220 ->
//	  sub_145A83FA0: itemdef+2120 <= 0xB): + u32len options + u32len sockets
//	creature rows (itemdef+2120 == 26, sub_1452C1540 @0x1452c186d):
//	  + u32 + u8 extension, NO avatar blobs
//	tail 87B (all rows): u8 vector count, u32, u8 nested collection count,
//	  then u8/u16/u32 scalars interleaved with len-prefixed blobs; the
//	  current rows carry only zero/empty cells
//
// The reader is driven by the item DEFINITION type, not the slot number. A
// worn creature always sits on slot 26 (WearRules), so slot 26 <=> type 26
// here. Worn rows above slot 11 carry their instance Record over NOTI 13/14;
// the mode-1 row has no cells for it, so Record is ignored there.
const (
	detailedWornHeaderSize      = 40
	detailedWornTailSize        = 87
	detailedWornCreatureExtSize = 5
	detailedWornAvatarSlotMax   = 11
	detailedWornCreatureSlot    = 26
	detailedWornCreatureSlotMax = 29 // creature body 26 + creature gear 27..29
	// detailedWornCreatureSkinSlot 是宠物幻化栏（list 3 槽 32）。它装的同样是
	// [creature] 物品，itemdef+2120 与槽 26 一样是 26，所以按定义类型分派的原生
	// reader sub_1452C1540 给它的也是 creature 行布局（5 字节扩展、无头像 blob）。
	detailedWornCreatureSkinSlot = 32
	detailedWornBlockTrailerSize = 13 // u32 scalar + u8 collection count + u64 flags
)

func DetailedEquipment(rows []DetailedWorn) ([]byte, error) {
	if len(rows) > 48 {
		return nil, fmt.Errorf("too many detailed worn items")
	}
	p := []byte{byte(len(rows))}
	seen := map[uint16]bool{}
	for _, v := range rows {
		avatar := v.Slot <= detailedWornAvatarSlotMax
		creature := v.Slot == detailedWornCreatureSlot || v.Slot == detailedWornCreatureSkinSlot
		supported := avatar || (v.Slot >= detailedWornCreatureSlot && v.Slot <= detailedWornCreatureSlotMax) || v.Slot == detailedWornCreatureSkinSlot
		if !supported || v.Template == 0 || seen[v.Slot] {
			return nil, fmt.Errorf("unsupported detailed equipment instance")
		}
		if avatar && len(v.Record) != 0 {
			return nil, fmt.Errorf("unsupported detailed equipment instance")
		}
		if !avatar && (len(v.AvatarOptions) != 0 || len(v.AvatarSockets) != 0) {
			return nil, fmt.Errorf("avatar blob on non-avatar detailed row")
		}
		if !avatar && (v.HeaderTemplateA != 0 || v.HeaderTemplateB != 0) {
			return nil, fmt.Errorf("avatar attachment on non-avatar detailed row")
		}
		if len(v.AvatarOptions) > 4096 || len(v.AvatarSockets) > 4096 {
			return nil, fmt.Errorf("oversized avatar data")
		}
		seen[v.Slot] = true
		row := make([]byte, detailedWornHeaderSize)
		row[0] = byte(v.Slot)
		binary.LittleEndian.PutUint32(row[1:], v.Template)
		binary.LittleEndian.PutUint16(row[10:], v.Durability)
		binary.LittleEndian.PutUint32(row[24:], v.HeaderTemplateA)
		binary.LittleEndian.PutUint32(row[28:], v.HeaderTemplateB)
		p = append(p, row...)
		if avatar {
			p = append(add32(p, uint32(len(v.AvatarOptions))), v.AvatarOptions...)
			p = append(add32(p, uint32(len(v.AvatarSockets))), v.AvatarSockets...)
		}
		if creature {
			// @0x1452c186f: u32 (read into a local the row parser never
			// stores) + u8 (item struct creature cell, high half of the
			// u16 whose low half comes from the tail). Zero is the unset
			// shape; value semantics remain unverified, do not invent.
			p = append(p, make([]byte, detailedWornCreatureExtSize)...)
		}
		p = append(p, 0) // auxiliary pair count
		p = add32(p, v.Period)
		p = append(p, 0) // 1451aa420 reads a nested collection count for EVERY row.
		p = append(p, make([]byte, detailedWornTailSize-6)...)
	}
	return append(p, make([]byte, detailedWornBlockTrailerSize)...), nil
}
