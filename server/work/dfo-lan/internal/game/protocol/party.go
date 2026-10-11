package protocol

import (
	"encoding/binary"
	"fmt"
)

// SoloPartyInfo is the current99-byte ordinary PARTY_INFO block for one
// owned local actor in slot0. Empty optional groups are distinct from the
// old90 layout. The native reader passes slot0/actor to145f10390.
func SoloPartyInfo(actor uint16) ([]byte, error) {
	if actor == 0 || actor == 65535 {
		return nil, fmt.Errorf("invalid solo party actor")
	}
	p := make([]byte, 99)
	binary.LittleEndian.PutUint16(p, 1)
	binary.LittleEndian.PutUint16(p[4:], 1)
	p[15] = 1 // one-player capacity
	p[61] = 1 // one member
	binary.LittleEndian.PutUint16(p[63:], actor)
	return p, nil
}
