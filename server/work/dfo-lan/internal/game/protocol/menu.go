package protocol

import "fmt"

// CMD7 is bodyless (live15: thirteen-byte client frame). CMD1301 is
// bodyless at143ca6672..143ca668e. CMD3 appends one byte at14517c832.
func DecodeMenuRequest(id uint16, p []byte) (byte, error) {
	switch id {
	case 7, 1301:
		if len(p) != 0 {
			return 0, fmt.Errorf("menu command %d requires an empty body", id)
		}
		return 0, nil
	case 3:
		if len(p) != 16 {
			return 0, fmt.Errorf("exit request must be one byte padded to16")
		}
		if e := padding(p[1:], 16); e != nil {
			return 0, e
		}
		return p[0], nil // Native exit option, retained for diagnostics only.
	default:
		return 0, fmt.Errorf("unsupported menu command %d", id)
	}
}

// Both native CMD3/7 success handlers read a u32 before their independent
// exit/return cleanup. Zero bypasses the optional145d2a120 presentation.
func MenuLeaveSuccess() []byte { return []byte{1, 0, 0, 0, 0} }

// CMD1301 success reads two u32 values at1452862bb/1452862c5 and resolves
// source-map spawn coordinates before issuing the normal CMD36 transition.
func VillageReturnSuccess(town, area uint32) []byte {
	return add32(add32([]byte{1}, town), area)
}
