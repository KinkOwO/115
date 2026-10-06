package protocol

import "fmt"

const CloneAvatarSourceOpcode uint16 = 1437

// CloneAvatarSources implements the current reader's clear-and-read contract.
// Count11 clears old mappings AND reads 11 rows. Slot11 is a subsequent delta.
// Values are actual avatar bag positions +12; FFFF has no source object.
func CloneAvatarSources(sources map[byte]uint16) (full, last []byte, err error) {
	for slot, source := range sources {
		if slot > 11 || source != 0xffff && source >= AvatarInventorySlots(MaxAvatarInventoryExpansion) {
			return nil, nil, fmt.Errorf("invalid Clone avatar source")
		}
	}
	full = []byte{11}
	for slot := byte(0); slot < 12; slot++ {
		value := uint16(0xffff)
		if source, ok := sources[slot]; ok && source != 0xffff {
			value = source + 12
		}
		full = add16(append(full, slot), value)
	}
	return full[:34], append([]byte{1}, full[34:]...), nil
}

func ItemMoveSuccessMode(r ItemMoveRequest, count uint32, mode byte) ([]byte, error) {
	if mode > 1 {
		return nil, fmt.Errorf("unsupported Clone move success mode")
	}
	p := ItemMoveSuccess(r, count)
	p[len(p)-1] = mode
	return p, nil
}
