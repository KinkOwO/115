package protocol

import (
	"encoding/binary"
	"fmt"
)

// Minimal current-build one-member conquest grammar, not a recorded player.
// Mode occurs TWICE; the later assignment otherwise erases the earlier27.
func MoonSoloParty115(actor uint16, context [2]byte, remaining byte) ([]byte, error) {
	if actor == 0 || actor == 65535 || remaining == 0 {
		return nil, fmt.Errorf("invalid Moon party actor/count")
	}
	p := make([]byte, 113)
	binary.LittleEndian.PutUint16(p, 1)
	binary.LittleEndian.PutUint16(p[2:], 9999)
	binary.LittleEndian.PutUint16(p[4:], 1)
	copy(p[6:8], context[:])
	p[20] = 1
	p[29] = 27
	p[69] = 1
	binary.LittleEndian.PutUint16(p[71:], actor)
	copy(p[46:54], []byte{1, 1, 2, 4, 7, 7, 7, 7})
	p[95] = 27
	p[106] = 41
	binary.LittleEndian.PutUint32(p[107:], 1)
	p[111], p[112] = remaining, remaining
	return p, nil
}
func MoonSoloPartyGone115(context [2]byte) []byte {
	p := make([]byte, 12)
	binary.LittleEndian.PutUint16(p, 1)
	binary.LittleEndian.PutUint16(p[2:], 9999)
	binary.LittleEndian.PutUint16(p[4:], 1)
	copy(p[6:8], context[:])
	p[9] = 3
	p[10] = 1
	return p
}
