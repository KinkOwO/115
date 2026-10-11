package protocol

import (
	"encoding/binary"
	"fmt"
)

// DecodePartyPlayResult115 follows the current CMD46 writer at
// 145e28a2b..145e292c8: u8, u16, u8 count, count*(u16 actor,u32 stat,u8 rank),
// then the existing 102-byte result tail. Native writer vectors cover 1/2/4/8
// distinct actors. Membership comes exclusively from the server-owned run.
func DecodePartyPlayResult115(p []byte, self uint16, actors []uint16) (PlayResultRequest, error) {
	var result PlayResultRequest
	if len(p) < 4 || p[0] != 0 || p[3] == 0 || p[3] > 8 || int(p[3]) != len(actors) {
		return result, fmt.Errorf("party result membership count mismatch")
	}
	logical := 106 + 7*len(actors)
	padded := (logical + 4 + 15) / 16 * 16
	if len(p) != logical && len(p) != padded {
		return result, fmt.Errorf("unsupported current party result size")
	}
	if err := digestRequestTail(p, logical, 16); err != nil {
		return result, err
	}
	wanted := make(map[uint16]bool, len(actors))
	for _, actor := range actors {
		if actor == 0 || actor == 65535 || wanted[actor] {
			return result, fmt.Errorf("invalid server-owned party result membership")
		}
		wanted[actor] = true
	}
	if !wanted[self] {
		return result, fmt.Errorf("result requester is not an instance member")
	}
	for row := range actors {
		offset := 4 + 7*row
		actor := binary.LittleEndian.Uint16(p[offset:])
		if !wanted[actor] {
			return PlayResultRequest{}, fmt.Errorf("duplicate or foreign party result actor")
		}
		delete(wanted, actor)
		if actor == self {
			result = PlayResultRequest{Actor: actor, RankPoint: p[offset+6]}
		}
	}
	return result, nil
}
