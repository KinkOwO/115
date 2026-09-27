package protocol

import (
	"encoding/binary"
	"fmt"
)

type CharacterSlotRequest struct {
	Swap, Before       bool
	FromFixed, ToFixed bool
	From, To           uint32
}

// CMD295 sender 0x1402393a0 writes u8 swap, [u8 before if !swap],
// u8 source kind, u32 source, u8 target kind, u32 target. Kind 0 is a
// zero-based roster index; kind 1 is a one-based fixed grid cell. Writers
// 0x146d75cc0/ce0 establish the widths. Drag callers 0x140223ec0,
// 0x140224cb0, 0x140225d60 and 0x140226b70 establish the move semantics.
func DecodeCharacterSlot(p []byte) (CharacterSlotRequest, error) {
	var r CharacterSlotRequest
	if len(p) != 16 || p[0] > 1 {
		return r, fmt.Errorf("invalid character slot request")
	}
	r.Swap = p[0] == 1
	i := 1
	if !r.Swap {
		if p[i] > 1 {
			return r, fmt.Errorf("invalid character insertion side")
		}
		r.Before = p[i] == 1
		i++
	}
	if p[i] > 1 || p[i+5] > 1 {
		return r, fmt.Errorf("invalid character slot kind")
	}
	r.FromFixed, r.ToFixed = p[i] == 1, p[i+5] == 1
	r.From = binary.LittleEndian.Uint32(p[i+1:])
	r.To = binary.LittleEndian.Uint32(p[i+6:])
	return r, padding(p[i+10:], 16)
}

// CMD295 callback 0x14524fc60 has no success body. The common dispatcher
// consumes the success byte (or zero + u16 error); error 19 has a native UI.
func CharacterSlotSuccess() []byte { return []byte{1} }
