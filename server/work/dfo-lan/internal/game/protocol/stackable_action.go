package protocol

import (
	"encoding/binary"
	"fmt"
)

// AddSkinStorageAction is the CMD507 action code the client sends when using an
// `[add skin storage]` stackable (damage-font and other skin-register items).
// Live capture 2026-09-26: using 10305398/10358669 sends CMD507 with the slot
// at p[0:2] and action 169 (0xA9) at p[7:11], every other byte zero. The
// fatigue potion shares this frame with action 54.
const AddSkinStorageAction = 169

// DecodeStackableAction parses the shared CMD507 "use stackable" frame and
// returns its slot and action code. The frame is 59 or 64 bytes; the slot is a
// u16 at offset 0 and the action a u32 at offset 7. All remaining bytes must be
// zero, which also constrains the action to a single byte (both 54 and 169 fit).
func DecodeStackableAction(p []byte) (slot uint16, action uint32, err error) {
	if len(p) != 59 && len(p) != 64 {
		return 0, 0, fmt.Errorf("stackable action length")
	}
	slot = binary.LittleEndian.Uint16(p)
	if slot == 0 {
		return 0, 0, fmt.Errorf("unsupported stackable action")
	}
	action = binary.LittleEndian.Uint32(p[7:])
	for i, b := range p {
		if i < 2 || i == 7 {
			continue
		}
		if b != 0 {
			return 0, 0, fmt.Errorf("unsupported stackable action fields")
		}
	}
	return slot, action, nil
}

// CMD507 sender writes u16 slot, u8 list, four u32 fields and 40 bytes.
// Action54's captured request has only the slot and action populated.
func DecodeFatigueAction(p []byte) (uint16, error) {
	slot, action, err := DecodeStackableAction(p)
	if err != nil {
		return 0, err
	}
	if action != 54 {
		return 0, fmt.Errorf("unsupported stackable action")
	}
	return slot, nil
}

// DecodeAddSkinStorageAction accepts only the `[add skin storage]` action (169).
func DecodeAddSkinStorageAction(p []byte) (uint16, error) {
	slot, action, err := DecodeStackableAction(p)
	if err != nil {
		return 0, err
	}
	if action != AddSkinStorageAction {
		return 0, fmt.Errorf("unsupported stackable action")
	}
	return slot, nil
}

// DecodeQuestAirshipAction accepts the exact native CMD507 shape observed
// when the level-94 airship communicator was used in town: list 0, action 206,
// and no extra parameters. The item identity is resolved from the owned bag
// slot, not inferred from the request's other u32 fields.
func DecodeQuestAirshipAction(p []byte) (uint16, error) {
	if len(p) != 64 && len(p) != 59 {
		return 0, fmt.Errorf("airship action length")
	}
	slot := binary.LittleEndian.Uint16(p)
	if slot == 0 || p[2] != 0 || binary.LittleEndian.Uint32(p[7:]) != 206 {
		return 0, fmt.Errorf("unsupported airship action")
	}
	for i, b := range p {
		if i < 2 || i >= 7 && i < 11 {
			continue
		}
		if b != 0 {
			return 0, fmt.Errorf("unsupported airship action fields")
		}
	}
	return slot, nil
}
