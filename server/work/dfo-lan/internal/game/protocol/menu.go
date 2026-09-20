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

// ExitDialogReady acknowledges CMD2285, the empty CONTENT_BRIEFING request
// issued by the in-game menu before its local Exit path. The dispatcher
// consumes the common success byte and u16 result code. The native callback at
// 145250c80 then unconditionally reads a 0x80-byte briefing structure through
// the packet reader at 146ea0be0. A shorter reply triggers CMD217
// (CMDPACKET_OVERFLOW_INFO). A zero content id selects the built-in briefing.
func ExitDialogReady() []byte {
	p := make([]byte, 3+0x80)
	p[0] = 1
	return p
}

// CMD682 is emitted after the player confirms the Exit modal. The sender at
// 146df89bd writes exactly one u8 and registers no response callback. On the
// wire that byte is followed by the three zero bytes of the cipher block. It
// is therefore a terminal session signal: the server validates it and closes
// the connection without manufacturing a response packet.
func DecodeExitShutdownSignal(p []byte) (bool, error) {
	if len(p) != 4 {
		return false, fmt.Errorf("exit shutdown signal must be one byte padded to4")
	}
	if p[0] > 1 {
		return false, fmt.Errorf("invalid exit shutdown flag %d", p[0])
	}
	if e := padding(p[1:], 4); e != nil {
		return false, e
	}
	return p[0] != 0, nil
}

// CMD1301 success reads two u32 values at1452862bb/1452862c5 and resolves
// source-map spawn coordinates before issuing the normal CMD36 transition.
func VillageReturnSuccess(town, area uint32) []byte {
	return add32(add32([]byte{1}, town), area)
}
