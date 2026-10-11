package protocol

import (
	"encoding/binary"
	"fmt"
)

// PartyTownVitals115 is N153, current handler14530bb70: count then
// actor:u16, town status percent:u8, two opaque bytes, unavailable:u8.
// Both classic town bars in145555960 read the status percent at info+534.
// This updates the town display only; combat HP/MP must use combat state.
func PartyTownVitals115(actors []uint16) ([]byte, error) {
	if len(actors) < 1 || len(actors) > 3 {
		return nil, fmt.Errorf("invalid robot vitals count")
	}
	p := []byte{byte(len(actors))}
	seen := map[uint16]bool{}
	for _, actor := range actors {
		if actor == 0 || actor == 65535 || seen[actor] {
			return nil, fmt.Errorf("invalid robot vitals identity")
		}
		seen[actor] = true
		p = append(p, byte(actor), byte(actor>>8), 100, 0, 0, 0)
	}
	return p, nil
}

// RobotFollowMove115 selects one authorized owner's SET_MOVE_INFO row.
// Native1466406ed reads count and14-byte row header;145e05df0 consumes
// four6-bit groups (3 two-bit scalar codes). Code0 has a following u32;
// code1 is zero and code2/3 carry prior values. Preserve the opaque values.
// No state, skills, attacks or object creation packets are replayed.
func RobotFollowMove115(p []byte, owner, robot uint16, room [2]uint16) ([]byte, error) {
	if owner == 0 || robot == 0 || owner == 65535 || robot == 65535 || owner == robot || len(p) < 4 || p[0] > 2 || binary.LittleEndian.Uint16(p[1:]) != 4 || p[3] == 0 || p[3] > 32 {
		return nil, fmt.Errorf("invalid movement envelope")
	}
	at := 4
	var selected []byte
	for row := 0; row < int(p[3]); row++ {
		start := at
		if len(p)-at < 14 {
			return nil, fmt.Errorf("short movement header")
		}
		kind := binary.LittleEndian.Uint16(p[at:])
		id := binary.LittleEndian.Uint16(p[at+2:])
		x := binary.LittleEndian.Uint16(p[at+6:])
		y := binary.LittleEndian.Uint16(p[at+8:])
		flags := binary.LittleEndian.Uint32(p[at+10:])
		at += 14
		if kind < 1 || kind > 25 {
			return nil, fmt.Errorf("invalid movement object type")
		}
		for _, shift := range []uint{26, 20, 14, 8} {
			for axis := uint(0); axis < 3; axis++ {
				if (flags>>(shift+2*axis))&3 == 0 {
					at += 4
				}
			}
		}
		if at > len(p) {
			return nil, fmt.Errorf("short movement scalar")
		}
		// Live pinned native145df2df0 selects both players only with type5, so another
		// object's numeric ID must never masquerade as a player movement row.
		if kind == 5 && id == owner {
			if selected != nil {
				return nil, fmt.Errorf("duplicate owner movement")
			}
			if (x != room[0] || y != room[1]) && (x != 65535 || y != 65535) {
				return nil, fmt.Errorf("stale movement room")
			}
			selected = append([]byte(nil), p[start:at]...)
			binary.LittleEndian.PutUint16(selected[2:], robot)
			binary.LittleEndian.PutUint16(selected[6:], room[0])
			binary.LittleEndian.PutUint16(selected[8:], room[1])
		}
	}
	if at != len(p) || selected == nil {
		return nil, fmt.Errorf("movement trailing bytes or owner absent")
	}
	return append([]byte{p[0], 4, 0, 1}, selected...), nil
}
