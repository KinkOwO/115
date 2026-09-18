package protocol

import (
	"encoding/binary"
	"fmt"
)

// CMD40 sender 145d29bbc writes x, y and a mode-specific actor word.
// Ordinary solo reports zero for the latter (145d29dd8). Slot1 pads to16.
func DecodePlayerDeath(p []byte) ([2]uint16, error) {
	var xy [2]uint16
	if len(p) != 16 {
		return xy, fmt.Errorf("player death requires 16-byte solo record")
	}
	for _, b := range p[4:] {
		if b != 0 {
			return xy, fmt.Errorf("unsupported player death mode or padding")
		}
	}
	xy[0], xy[1] = binary.LittleEndian.Uint16(p), binary.LittleEndian.Uint16(p[2:])
	return xy, nil
}

// NOTI32 at1452aa080 reads u16 actor, u8 state, u8 flag, u16 hp, u16 mp.
// State0 marks the actor dead and enters the native resurrection UI path.
// Sending only an ACK40 never runs that path. No currency is changed here.
func PlayerDeathState(actor uint16) ([]byte, error) {
	if actor == 0 || actor == 65535 {
		return nil, fmt.Errorf("invalid death actor")
	}
	p := make([]byte, 8)
	binary.LittleEndian.PutUint16(p, actor)
	return p, nil
}
