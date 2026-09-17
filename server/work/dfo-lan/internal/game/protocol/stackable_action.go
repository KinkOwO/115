package protocol

import (
	"encoding/binary"
	"fmt"
)

// CMD507 sender writes u16 slot, u8 list, four u32 fields and 40 bytes.
// Action54's captured request has only the slot and action populated.
func DecodeFatigueAction(p []byte) (uint16, error) {
	if len(p) != 59 && len(p) != 64 {
		return 0, fmt.Errorf("fatigue action length")
	}
	slot := binary.LittleEndian.Uint16(p)
	if slot == 0 || binary.LittleEndian.Uint32(p[7:]) != 54 {
		return 0, fmt.Errorf("unsupported stackable action")
	}
	for i, b := range p {
		if i < 2 || i == 7 {
			continue
		}
		if b != 0 {
			return 0, fmt.Errorf("unsupported fatigue action fields")
		}
	}
	return slot, nil
}
