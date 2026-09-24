package protocol

import (
	"encoding/binary"
	"fmt"
)

// EquippedAppearance is one entry of the native 0x145639840 block.
//
// The entry is VARIABLE width: 35 + n bytes, where n is a length carried
// inside the entry. Recovered instruction by instruction from the loop body
// 0x1456398c0..0x145639ac8 (record:
// work/p8-equip-refresh/revert38/外观块-指令级定案.md):
//
//	+0     u8   Slot        index for every write target below
//	+1     u32  Placeholder 历史命名；实际为装备模板，保存到 slot*8+0x30。
//	                       145BEFD60 -> 145BD63D0 -> 145BEE6C0 用于城镇模型查找。
//	+5     u32  Len         0x145639906 -> 0x1459a0220 -> 0x146d77f50, which
//	                       reads this u32 then copies Len bytes
//	+9     n    Payload     the copied bytes; its first 4 become the index
//	                       stored at [actor + slot*4 + 0x405]
//	+9+n   u8   Flags       split: [actor+slot*8+0x34] = v>>1,
//	                       [actor+slot*4+0x334] = v&1
//	+10+n  u8   WeaponA     weapon slot only (slot == 12)
//	+11+n  u8   WeaponB     weapon slot only
//	+12+n  10   WeaponTail  weapon slot only, forwarded to one scratch buffer
//	+22+n  u32  AttachA     -> [actor+slot*4+0x1b0], written only when non-zero
//	+26+n  u32  AttachB     -> [actor+slot*4+0x270], written only when non-zero
//	+30+n  u32  Reserved    read into a local; not written by this reader
//	+34+n  u8   Tail        read into a local; not written by this reader
//
// The dispatch that decides whether the payload is read at all is worth
// stating exactly, because it has been misread twice. 0x1459a0220 is called
// with r8d = 0x2e and:
//
//	0x1459a0226  lea eax, [r8-1]        ; 0x2d
//	0x1459a022d  test eax, 0xffffffdf   ; 0x0d, non-zero
//	0x1459a0232  je  0x1459a0248        ; not taken
//	0x1459a0234  cmp r8d, 3
//	0x1459a0238  jne 0x1459a0255        ; taken, 0x2e != 3
//	0x1459a0255  cmp r8d, 0x2e
//	0x1459a0259  je  0x1459a0248        ; TAKEN
//	0x1459a0248  call 0x146d77f50       ; reads Len, then Len bytes
//
// so a Len of n consumes n bytes. Len = 0 is legal and leaves the model index
// at whatever 0x147045640 cleared it to (that helper is three instructions:
// xor eax,eax / mov [rcx],eax / ret), which is the "no binding for this slot"
// encoding. Len of 4 is the shortest shape that carries an index.
type EquippedAppearance struct {
	Slot        byte
	Placeholder uint32
	Model       uint32
	Flags       byte
	WeaponA     byte
	WeaponB     byte
	WeaponTail  [10]byte
	AttachA     uint32
	AttachB     uint32
	Reserved    uint32
	Tail        byte
}

// EquippedAppearanceBaseSize is the entry length when Len is zero.
const EquippedAppearanceBaseSize = 1 + 4 + 4 + 1 + 1 + 1 + 10 + 4 + 4 + 4 + 1 // = 35

// equippedAppearanceModelSize is the payload length used to carry a model
// index: exactly one dword, which is what the reader takes from the head of
// the copied block.
const equippedAppearanceModelSize = 4

// equippedAppearanceEntrySize reports the on-wire length of one entry.
func equippedAppearanceEntrySize(row EquippedAppearance) int {
	if row.Model == 0 {
		return EquippedAppearanceBaseSize
	}
	return EquippedAppearanceBaseSize + equippedAppearanceModelSize
}

// maxEquippedAppearanceSlot is the largest slot the client's tables cover:
// the wear table ends at [earring] = 25. The native reader performs no bounds
// check on the slot it reads: it computes [actor + slot*4 + 0x405] from the raw
// zero-extended byte. A slot past this value would be an out-of-bounds write in
// the client, so it is rejected here.
const maxEquippedAppearanceSlot = 25

// weaponSlot is the only slot whose dedicated tail fields the client reads.
const weaponSlot = 12

// EntryBasicProbe is the current x64 client's minimum actor-information
// notification. It is deliberately separate from roster rows and world rules.
// Unknown fields are empty probe values, not recovered official defaults.
// Native: 0x145637a20 mode 0 -> 0x14563ec60, through 0x145641038.
type EntryBasicProbe struct {
	ActorServerID uint16
	Context       [2]byte
	Character     CharacterRow
	Fame          uint32

	// Appearance is the per-slot state the native 0x145639840 block carries,
	// one entry per slot the packet speaks about. Slots the packet does not
	// mention are left as the block's zero count implies, so an absent slot
	// keeps whatever the client already had rather than being reset.
	Appearance []EquippedAppearance
}

// UserInfoBasicProbe serialises the own-actor entry packet.
//
// The 0x145639840 equipped-appearance block IS carried here. Three earlier
// revisions of this file got its entry width wrong and each crashed the client
// differently; the width is now pinned instruction by instruction:
//
//	revision     entry   why it was wrong
//	dungeon38    —       shipped an empty block
//	751cd05      15      counted only slot + one u32 + ten u8
//	35           35      read 0x147045640 as a wire payload length
//	35+n current 35+n    0x1459a0220 dispatches r8d=0x2e to the variable
//	                     reader (cmp r8d,0x2e / je), so a declared Len of n
//	                     consumes n payload bytes; Len=0 keeps the slot
//
// Both wrong widths make the client's read of this block end at the wrong
// offset, every following field shifts, and the trailing collection at
// 0x14563c140 reads a garbage count (observed loop index ~439,000), driving
// the u32 reader 0x146ea0ba0 into an invalid pointer (rdx = 0x7FE90000).
//
// The layout is not inferred from byte counts alone. The consumer at
// 0x1452d3980 independently confirms the field placement: it reads
// [actor+slot*4+0x405], [actor+slot*4+0x334], [actor+slot*4+0x1b0] and
// [actor+slot*8+0x34], which are exactly this block's write targets. And the
// writer at 0x145acc960 stores to [table+slot*8+0x30], [table+slot*8+0x34],
// [table+slot*4+0x405] and [table+slot*4+0x270] from a gear object's own
// fields — the same four offsets, reached from the other direction.
//
// The block is where the client learns the per-slot model binding, so a slot
// the character actually wears is emitted with its appearance index. A slot the
// packet leaves out keeps whatever binding the client already holds, and a
// piece with no appearance cell in its source row contributes no entry at all -
// i.e. "apply what is present, ignore what is not". The reader's own dword at
// +5 is written from a local it clears with a three-instruction helper
// (0x147045640: xor eax,eax / mov [rcx],eax / ret), which is why a wrong entry
// width desynchronises the global read cursor rather than merely mis-binding a
// model.
//
// Full record: work/p8-equip-refresh/revert38/外观块-指令级定案.md.
func UserInfoBasicProbe(s EntryBasicProbe) ([]byte, error) {
	r := s.Character
	if s.ActorServerID == 0 || s.ActorServerID == 0xffff || r.Level == 0 {
		return nil, fmt.Errorf("invalid entry actor identity or level")
	}
	if _, _, err := parseName(addName(nil, r.Name)); err != nil {
		return nil, err
	}
	// Two producers feed the 0x145639840 block. The C9 post-move refresh
	// (AppearanceProbe) passes explicit worn-slot rows in s.Appearance and they
	// are emitted verbatim. The entry-time path leaves s.Appearance empty on
	// purpose - the client fills every slot's model itself from the worn item
	// objects (live-verified 2026-09-18) - so only the list-row equipment
	// projection runs there.
	var appearance []byte
	var err error
	if len(s.Appearance) > 0 {
		appearance, err = equippedAppearanceBlock(s.Appearance)
	} else {
		appearance, err = EquipmentAppearance(r.Equipment)
	}
	if err != nil {
		return nil, err
	}
	p := append(add16([]byte{0}, 1), s.Context[:]...)
	// 0x14563ecd2 consumes all 160 bytes before the actor ID. Two inline
	// zero-terminated strings start at +0x1b and +0x5f; these remain empty.
	p = append(p, make([]byte, 160)...)
	// 14563ecd2 读取到 14dc67340；145640f2c 从 +0x80 取名望并调用 145f05f60。
	binary.LittleEndian.PutUint32(p[5+0x80:], s.Fame)
	p = addName(add16(p, s.ActorServerID), r.Name)
	p = append(p, r.Profession, r.Advancement, r.Level, 0, 0)
	p = append(p, appearance...) // 0x145639840: equipped appearance block (incl. mandatory blob lengths)
	p = add32(p, 0)              // 0x14563f0f1
	p = append(p, 0, 0, 0, 0)
	p = append(p, 0) // 0x145639b70: cosmetic count
	p = add32(add32(p, 0), 0)
	p = append(p, 0)
	p = add32(p, 0)
	p = append(p, 0)
	creatureItemID := r.CreatureItemID
	creatureName := r.CreatureName
	// 0x1456394b0: creature fields (item_id, dstr name, u8 present).
	// The trailing byte is the actor's creature visibility gate, not a dead
	// flag: the client reader stores its inverse as the companion's hidden
	// state, so 0 keeps a fully created creature invisible (GF115 history
	// "object created but hidden"; docs/宠物显示实现-G0198 §2.1 pins the
	// official value shape: item_id=slot-26 template, name, present=1).
	var creaturePresent byte
	if creatureItemID != 0 {
		creaturePresent = 1
	}
	p = append(addName(add32(p, creatureItemID), creatureName), creaturePresent)
	p = append(p, 0) // 0x14563be50: premium PC-room byte
	p = add32(add32(p, 0), 0)
	p = add16(add32(addName(p, ""), 0), 0) // 0x14563a0b0
	p = add32(p, 0)
	p = append(p, 1, 0, 0)     // 0x14563f338 uses native initialized flag 1
	p = add32(append(p, 0), 0) // 0x145639dc0
	p = add32(append(p, 0), 0)
	p = add16(add32(p, 0), 0)
	p = append(add32(p, 0), make([]byte, 8)...) // 0x14563a240: u32 + raw8
	p = append(p, nativeGrowthStateFlags)       // 0x14563fc24: bit0 + growth-appearance bit1
	p = add32(p, 0)
	p = append(p, 0)
	p = add16(p, 0)
	p = append(p, 0)
	p = append(add16(p, 0), 0) // 0x14563bdc0: u16 + u8
	p = add16(p, 0)
	p = append(p, 0xff) // 0x14563a4a0 native unset value
	p = append(p, 0, 0, 0)
	p = add32(p, 0)
	p = add32(p, 0) // 0x14563c140: count, no nested u32 pairs
	// Native 145640932 reads the sixth byte into info+672.
	// 1402cd740 tests info+672 == 5 for [is arad odyssey user].
	var mode byte
	if r.Odyssey {
		mode = 5
	}
	p = append(p, 0, 0, 0, 0, 0, mode)
	p = add32(add32(add32(p, 0), 0), 0)
	return p, nil
}

// equippedAppearanceBlock encodes the 0x145639840 block: a u8 count followed by
// count entries whose length is 35 plus the payload each entry declares.
//
// The block is counted, so at most 255 entries fit. Beyond that, for a
// duplicate or out-of-range slot, or for weapon-only tail data on a non-weapon
// slot, the payload would not round-trip through the native reader; all of
// those are reported instead of being truncated silently.
func equippedAppearanceBlock(rows []EquippedAppearance) ([]byte, error) {
	if len(rows) > 0xff {
		return nil, fmt.Errorf("equipped appearance block overflows its u8 count")
	}
	seen := make(map[byte]bool, len(rows))
	total := 1
	for _, row := range rows {
		total += equippedAppearanceEntrySize(row)
	}
	p := make([]byte, 0, total)
	p = append(p, byte(len(rows)))
	for _, row := range rows {
		if seen[row.Slot] {
			return nil, fmt.Errorf("duplicate equipped appearance slot %d", row.Slot)
		}
		seen[row.Slot] = true
		if row.Slot > maxEquippedAppearanceSlot {
			return nil, fmt.Errorf("equipped appearance slot %d exceeds the client table", row.Slot)
		}
		// The tail fields exist only for the weapon slot; anything left in them
		// for another slot would be read as a different piece of state. They are
		// zeroed rather than rejected so a plain piece stays legal.
		if row.Slot != weaponSlot && (row.WeaponA != 0 || row.WeaponB != 0 || row.WeaponTail != [10]byte{}) {
			return nil, fmt.Errorf("weapon-only appearance tail set on slot %d", row.Slot)
		}
		p = append(p, row.Slot)
		p = add32(p, row.Placeholder)
		// Len and payload move together: a zero Len means "leave this slot's
		// model index alone", a Len of four carries the index the client stores
		// at [actor+slot*4+0x405].
		if row.Model == 0 {
			p = add32(p, 0)
		} else {
			p = add32(p, equippedAppearanceModelSize)
			p = add32(p, row.Model)
		}
		p = append(p, row.Flags, row.WeaponA, row.WeaponB)
		p = append(p, row.WeaponTail[:]...)
		p = add32(add32(add32(p, row.AttachA), row.AttachB), row.Reserved)
		p = append(p, row.Tail)
	}
	return p, nil
}
