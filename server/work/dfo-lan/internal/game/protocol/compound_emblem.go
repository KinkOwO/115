package protocol

import (
	"encoding/binary"
	"fmt"
)

type CompoundEmblemInput struct {
	Template uint32 `json:"template"`
	Slot     uint16 `json:"slot"`
}

type CompoundEmblemRequest struct {
	Inputs []CompoundEmblemInput
	Mode   byte
}

type CompoundEmblemReward struct {
	Template uint32 `json:"template"`
	Count    uint32 `json:"count"`
}

// Native sender 145F601B0: u8 count, (u32 template,u16 slot)*count,
// u8 selection mode. The captured CMD256 has zero transport padding.
func DecodeCompoundEmblem(p []byte) (CompoundEmblemRequest, error) {
	var r CompoundEmblemRequest
	if len(p) < 2 || p[0] < 2 || p[0] > 5 {
		return r, fmt.Errorf("invalid emblem compound count")
	}
	n := 2 + 6*int(p[0])
	if len(p) != n && len(p) != (n+7)&^7 {
		return r, fmt.Errorf("invalid emblem compound length")
	}
	for _, v := range p[n:] {
		if v != 0 {
			return r, fmt.Errorf("nonzero emblem compound padding")
		}
	}
	seen := map[uint16]bool{}
	for i := 0; i < int(p[0]); i++ {
		o := 1 + 6*i
		in := CompoundEmblemInput{binary.LittleEndian.Uint32(p[o:]), binary.LittleEndian.Uint16(p[o+4:])}
		if in.Template < 2 || in.Slot < 2 || in.Slot == 65535 || seen[in.Slot] {
			return CompoundEmblemRequest{}, fmt.Errorf("invalid emblem compound input")
		}
		seen[in.Slot] = true
		r.Inputs = append(r.Inputs, in)
	}
	r.Mode = p[n-1]
	return r, nil
}

// Native reader 14526F130: dispatcher success byte, u8 reward count,
// then (u32 template,u32 obtained amount). Slots belong to NOTI13.
func CompoundEmblemSuccess(rewards []CompoundEmblemReward) ([]byte, error) {
	if len(rewards) == 0 || len(rewards) > 255 {
		return nil, fmt.Errorf("invalid emblem compound rewards")
	}
	p := []byte{1, byte(len(rewards))}
	seen := map[uint32]bool{}
	for _, r := range rewards {
		if r.Template < 2 || r.Count == 0 || seen[r.Template] {
			return nil, fmt.Errorf("invalid emblem compound reward")
		}
		seen[r.Template] = true
		p = add32(p, r.Template)
		p = add32(p, r.Count)
	}
	return p, nil
}
