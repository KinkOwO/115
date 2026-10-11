package protocol

import (
	"encoding/binary"
	"fmt"
)

// DecodePlayerContext115 is only REQUEST_PEER action 15, emitted by
// 146a54c00: u16 target, u8 action, u32 zero, u16 zero, u32 zero.
// Other REQUEST_PEER actions have independent, unimplemented lifecycles.
func DecodePlayerContext115(p []byte) (uint16, error) {
	if len(p) != 13 && len(p) != 16 {
		return 0, fmt.Errorf("unsupported player context request size")
	}
	actor := binary.LittleEndian.Uint16(p)
	if actor == 0 || actor == 65535 || p[2] != 15 {
		return 0, fmt.Errorf("invalid player context target or action")
	}
	for _, b := range p[3:] {
		if b != 0 {
			return 0, fmt.Errorf("unsupported player context options or padding")
		}
	}
	return actor, nil
}

// PlayerContextReply115 is NOTI7 action 15. Reader 14530f540 consumes
// u16 target, u8 action, u32 context, u16 target party, u16 requester.
// Context zero selects the ordinary popup via 146a518b0. Party 0xffff means
// no native party; the requester must be the connection's scene actor.
func PlayerContextReply115(target, requester, party uint16) ([]byte, error) {
	if target == 0 || target == 65535 || requester == 0 || requester == 65535 || target == requester || party == 0 {
		return nil, fmt.Errorf("invalid player context identities")
	}
	p := make([]byte, 11)
	binary.LittleEndian.PutUint16(p, target)
	p[2] = 15
	binary.LittleEndian.PutUint16(p[7:], party)
	binary.LittleEndian.PutUint16(p[9:], requester)
	return p, nil
}
