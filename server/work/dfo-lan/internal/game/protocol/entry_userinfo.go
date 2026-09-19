package protocol

import "fmt"

// EntryBasicProbe is the current x64 client's minimum actor-information
// notification. It is deliberately separate from roster rows and world rules.
// Unknown fields are empty probe values, not recovered official defaults.
// Native: 0x145637a20 mode 0 -> 0x14563ec60, through 0x145641038.
type EntryBasicProbe struct {
	ActorServerID uint16
	Context       [2]byte
	Character     CharacterRow
}

func UserInfoBasicProbe(s EntryBasicProbe) ([]byte, error) {
	r := s.Character
	if s.ActorServerID == 0 || s.ActorServerID == 0xffff || r.Level == 0 {
		return nil, fmt.Errorf("invalid entry actor identity or level")
	}
	appearance, err := EquipmentAppearance(r.Equipment)
	if err != nil {
		return nil, err
	}
	if _, _, err := parseName(addName(nil, r.Name)); err != nil {
		return nil, err
	}
	p := append(add16([]byte{0}, 1), s.Context[:]...)
	// 0x14563ecd2 consumes all 160 bytes before the actor ID. Two inline
	// zero-terminated strings start at +0x1b and +0x5f; these remain empty.
	p = append(p, make([]byte, 160)...)
	p = addName(add16(p, s.ActorServerID), r.Name)
	p = append(p, r.Profession, r.Advancement, r.Level, 0, 0)
	p = append(p, appearance...) // 145639840, including mandatory blob lengths
	p = add32(p, 0)              // 0x14563f0f1
	p = append(p, 0, 0, 0, 0)
	p = append(p, 0) // 0x145639b70: cosmetic count
	p = add32(add32(p, 0), 0)
	p = append(p, 0)
	p = add32(p, 0)
	p = append(p, 0)
	creatureItemID := r.CreatureItemID
	creatureName := r.CreatureName
	p = append(addName(add32(p, creatureItemID), creatureName), 0) // 0x1456394b0: creature fields (item_id, dstr name, u8 isDead=0)
	p = append(p, 0)                                               // 0x14563be50: premium PC-room byte
	p = add32(add32(p, 0), 0)
	p = add16(add32(addName(p, ""), 0), 0) // 0x14563a0b0
	p = add32(p, 0)
	p = append(p, 1, 0, 0)     // 0x14563f338 uses native initialized flag 1
	p = add32(append(p, 0), 0) // 0x145639dc0
	p = add32(append(p, 0), 0)
	p = add16(add32(p, 0), 0)
	p = append(add32(p, 0), make([]byte, 8)...) // 0x14563a240: u32 + raw8
	p = append(p, 1)                            // 0x14563fc24 uses native initialized flag 1
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
