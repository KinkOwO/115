package protocol

import (
	"encoding/binary"
	"fmt"
)

type DisjointAvatarRequest struct {
	Slot     uint16
	Template uint32
}

// Current sender 145AE34F0 writes u16 slot and u32 template; the observed
// CMD202 body 0000423ee51d0000 contains two transport padding bytes.
func DecodeDisjointAvatar(p []byte) (DisjointAvatarRequest, error) {
	if len(p) != 6 && len(p) != 8 {
		return DisjointAvatarRequest{}, fmt.Errorf("invalid avatar disjoint length")
	}
	for _, v := range p[6:] {
		if v != 0 {
			return DisjointAvatarRequest{}, fmt.Errorf("nonzero avatar disjoint padding")
		}
	}
	r := DisjointAvatarRequest{binary.LittleEndian.Uint16(p), binary.LittleEndian.Uint32(p[2:])}
	if r.Template == 0 || r.Slot == 65535 {
		return r, fmt.Errorf("invalid avatar disjoint target")
	}
	return r, nil
}

// CMD202 reader 145273560: success flag is consumed by the dispatcher, then
// u16 deleted slot, u16 reward count, and (u16 slot,u32 template,u32 count).
func DisjointAvatarSuccess(slot uint16, rewards []DisjointRewardEntry) ([]byte, error) {
	if slot == 65535 || len(rewards) == 0 || len(rewards) > 65535 {
		return nil, fmt.Errorf("invalid avatar disjoint receipt")
	}
	p := add16([]byte{1}, slot)
	p = add16(p, uint16(len(rewards)))
	for _, r := range rewards {
		if r.Slot == 65535 || r.Template == 0 || r.Count == 0 {
			return nil, fmt.Errorf("invalid avatar disjoint reward")
		}
		p = add16(p, r.Slot)
		p = add32(p, r.Template)
		p = add32(p, r.Count)
	}
	return p, nil
}
