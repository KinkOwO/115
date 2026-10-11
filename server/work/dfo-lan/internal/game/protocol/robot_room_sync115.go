package protocol

import (
	"encoding/binary"
	"fmt"
)

// Current dispatcher14663ef80: cases77 at14664d10d,192 at14664fdb5,
// 195 at146653f5f. Only native party-slot identity changes; preserve
// positions, loading stage/tick and room-transition state. No combat replay.
func RobotRoomSync115(p []byte, slot byte) ([]byte, error) {
	if slot < 1 || slot > 3 || len(p) < 3 || p[0] > 2 {
		return nil, fmt.Errorf("room sync envelope")
	}
	q := append([]byte(nil), p...)
	switch binary.LittleEndian.Uint16(p[1:]) {
	case 77:
		if len(p) != 8 || p[3] != 0 {
			return nil, fmt.Errorf("entry-complete identity")
		}
		q[3] = slot
	case 192:
		if len(p) != 13 || binary.LittleEndian.Uint16(p[3:]) != 0 {
			return nil, fmt.Errorf("loading-sync identity")
		}
		state := binary.LittleEndian.Uint32(p[5:])
		if state != 2 && state != 3 {
			return nil, fmt.Errorf("loading-sync stage")
		}
		binary.LittleEndian.PutUint16(q[3:], uint16(slot))
	case 195:
		if len(p) != 5 || p[3] != 0 || p[4] > 1 {
			return nil, fmt.Errorf("map-change identity")
		}
		q[3] = slot
	default:
		return nil, fmt.Errorf("unsupported room sync")
	}
	return q, nil
}
