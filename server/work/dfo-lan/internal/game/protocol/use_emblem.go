package protocol

import (
	"encoding/binary"
	"fmt"
)

type UseEmblemInput struct {
	Slot     uint16
	Template uint32
	Socket   byte
}

type UseEmblemRequest struct {
	Space      byte
	AvatarSlot uint16
	Template   uint32
	Inputs     []UseEmblemInput
}

type EmblemStackAmount struct {
	Slot   uint16
	Amount uint32
}

func (r UseEmblemRequest) Validate() error {
	// This flow updates the avatar bag. Worn/other storage spaces require their
	// own equipment/stat refresh and are deliberately not treated as bag slots.
	if r.Space != 1 || r.AvatarSlot == 65535 || r.Template < 2 || len(r.Inputs) < 1 || len(r.Inputs) > 5 {
		return fmt.Errorf("invalid avatar emblem target or count")
	}
	var sockets [5]bool
	slots := map[uint16]uint32{}
	for _, in := range r.Inputs {
		if in.Slot < 2 || in.Slot == 65535 || in.Template < 2 || in.Socket >= 5 || sockets[in.Socket] {
			return fmt.Errorf("invalid avatar emblem input")
		}
		if id, ok := slots[in.Slot]; ok && id != in.Template {
			return fmt.Errorf("conflicting avatar emblem stack")
		}
		sockets[in.Socket] = true
		slots[in.Slot] = in.Template
	}
	return nil
}

// Native sender 1469E6C00: u8 space, u16 avatar slot, u32 template,
// u8 count, then count * (u16 emblem slot, u32 template, u8 socket index).
// The transport may leave zero padding through the next eight-byte boundary.
func DecodeUseEmblem(p []byte) (UseEmblemRequest, error) {
	var r UseEmblemRequest
	if len(p) < 8 || p[7] < 1 || p[7] > 5 {
		return r, fmt.Errorf("invalid avatar emblem request header")
	}
	n := 8 + int(p[7])*7
	if len(p) < n || len(p) > (n+7)&^7 {
		return r, fmt.Errorf("invalid avatar emblem request length")
	}
	for _, b := range p[n:] {
		if b != 0 {
			return r, fmt.Errorf("nonzero avatar emblem request padding")
		}
	}
	r.Space, r.AvatarSlot, r.Template = p[0], binary.LittleEndian.Uint16(p[1:]), binary.LittleEndian.Uint32(p[3:])
	for i := 0; i < int(p[7]); i++ {
		o := 8 + i*7
		r.Inputs = append(r.Inputs, UseEmblemInput{binary.LittleEndian.Uint16(p[o:]), binary.LittleEndian.Uint32(p[o+2:]), p[o+6]})
	}
	return r, r.Validate()
}

// Native ACK reader 14529E4D0 sets each ordinary stack's absolute amount
// through vtable+208, then completes emblem window 537. It does not carry
// the avatar extension, which must be refreshed separately with NOTI14.
func UseEmblemSuccess(stacks []EmblemStackAmount) ([]byte, error) {
	if len(stacks) < 1 || len(stacks) > 5 {
		return nil, fmt.Errorf("invalid avatar emblem stack count")
	}
	p := make([]byte, 2+len(stacks)*6)
	p[0], p[1] = 1, byte(len(stacks))
	seen := map[uint16]bool{}
	for i, row := range stacks {
		if row.Slot < 2 || row.Slot == 65535 || seen[row.Slot] {
			return nil, fmt.Errorf("invalid avatar emblem result stack")
		}
		seen[row.Slot] = true
		binary.LittleEndian.PutUint16(p[2+i*6:], row.Slot)
		binary.LittleEndian.PutUint32(p[4+i*6:], row.Amount)
	}
	return p, nil
}
