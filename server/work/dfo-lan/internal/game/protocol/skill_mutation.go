package protocol

import (
	"encoding/binary"
	"fmt"
)

type SkillMove struct{ Tree, From, To byte }

func DecodeSkillMove(p []byte) (SkillMove, error) {
	var r SkillMove
	if len(p) < 7 {
		return r, fmt.Errorf("short skill move")
	}
	// Current 145ef7b91 writes seven bytes. Tree FF means selected tree zero.
	r = SkillMove{p[0], p[1], p[2]}
	if r.Tree == 255 {
		r.Tree = 0
	}
	if r.Tree != 0 || int32(binary.LittleEndian.Uint32(p[3:])) != -1 {
		return r, fmt.Errorf("unsupported skill move context")
	}
	return r, digestRequestTail(p, 7, 16)
}
func SkillMoveSuccess(r SkillMove) []byte { return []byte{1, r.Tree, r.From, r.To} }

type SkillPurchaseEntry struct {
	ID            uint16
	Refund, Delta byte
}
type SkillPurchase struct {
	Tree    byte
	Entries []SkillPurchaseEntry
}

func DecodeSkillPurchase(p []byte) (SkillPurchase, error) {
	var r SkillPurchase
	if len(p) < 6 {
		return r, fmt.Errorf("short skill purchase")
	}
	r.Tree = p[0]
	if r.Tree == 255 {
		r.Tree = 0
	}
	n := int(p[1])
	end := 2 + n*4
	// 1456cedb0/145ee8022 write four tail bytes (ordinary manual path all zero).
	if r.Tree != 0 || n == 0 || n > 128 || len(p) < end+4 {
		return r, fmt.Errorf("unsupported skill purchase")
	}
	for i := 2; i < end; i += 4 {
		v := SkillPurchaseEntry{binary.LittleEndian.Uint16(p[i:]), p[i+2], p[i+3]}
		if v.ID == 0 || v.Delta == 0 || v.Refund > 1 {
			return r, fmt.Errorf("invalid learning delta/refund")
		}
		r.Entries = append(r.Entries, v)
	}
	for _, b := range p[end : end+4] {
		if b != 0 {
			return r, fmt.Errorf("skill preset/talisman mode pending")
		}
	}
	return r, digestRequestTail(p, end+4, 8)
}
func SkillPurchaseSuccess(tree byte, sp, tp uint16, entries []LearnedSkill) ([]byte, error) {
	if tree > 1 || len(entries) > 255 {
		return nil, fmt.Errorf("invalid skill purchase response")
	}
	p := add16(add16([]byte{1, tree}, sp), tp)
	p = append(p, byte(len(entries)))
	for _, s := range entries {
		slot := s.Slot
		if slot == 65535 {
			slot = 255
		}
		if slot > 255 || s.ID == 0 {
			return nil, fmt.Errorf("invalid learned skill response")
		}
		p = append(p, byte(slot))
		p = add16(p, s.ID)
		p = append(p, s.Level, 0)
	}
	// Current 14526ac70 additionally reads two extension-presence bytes after
	// final mode. The reference90 short response misses these fields.
	return append(p, 0, 0, 0), nil
}
