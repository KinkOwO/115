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
	Tree         byte
	Entries      []SkillPurchaseEntry
	Mode, Preset byte
	Intensions   []SkillVariation
	Options      []SkillVariation
}

type SkillVariation struct {
	ID            uint16
	Choice        uint32
	Status, Empty byte
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
	if r.Tree != 0 || n > 128 || len(p) < end+4 {
		return r, fmt.Errorf("unsupported skill purchase")
	}
	for i := 2; i < end; i += 4 {
		v := SkillPurchaseEntry{binary.LittleEndian.Uint16(p[i:]), p[i+2], p[i+3]}
		if v.ID == 0 || v.Delta == 0 || v.Refund > 1 {
			return r, fmt.Errorf("invalid learning delta/refund")
		}
		r.Entries = append(r.Entries, v)
	}
	// Native 1459137c0 sends variable extensions, not four fixed flags.
	r.Mode, r.Preset = p[end], p[end+1]
	if r.Mode > 1 || r.Preset > 3 {
		return r, fmt.Errorf("invalid skill mode/preset")
	}
	pos := end + 2
	for group := 0; group < 2; group++ {
		if pos >= len(p) || p[pos] > 1 {
			return r, fmt.Errorf("invalid variation presence")
		}
		present := p[pos]
		pos++
		if present == 0 {
			continue
		}
		count, width := 3, 7
		if group == 1 {
			count, width = 5, 8
		}
		if len(p)-pos < count*width {
			return r, fmt.Errorf("short skill variation")
		}
		var rows []SkillVariation
		for i := 0; i < count; i++ {
			v := SkillVariation{ID: binary.LittleEndian.Uint16(p[pos:]), Choice: binary.LittleEndian.Uint32(p[pos+2:]), Status: p[pos+6]}
			if group == 1 {
				v.Empty = p[pos+7]
			}
			if v.Choice > 3 || v.Status > 2 || v.Empty > 1 {
				return r, fmt.Errorf("invalid variation row")
			}
			rows = append(rows, v)
			pos += width
		}
		if group == 0 {
			r.Intensions = rows
		} else {
			r.Options = rows
		}
	}
	if n == 0 && r.Intensions == nil && r.Options == nil {
		return r, fmt.Errorf("empty skill purchase")
	}
	return r, digestRequestTail(p, pos, 8)
}

func SkillPurchaseVariations(p []byte, mode byte, intensions, options []SkillVariation) ([]byte, error) {
	if len(p) < 3 || mode > 1 || intensions != nil && len(intensions) != 3 || options != nil && len(options) != 5 {
		return nil, fmt.Errorf("invalid variation response")
	}
	p = append([]byte(nil), p[:len(p)-3]...)
	p = append(p, mode)
	if intensions == nil {
		p = append(p, 0)
	} else {
		p = append(p, 1)
		for _, v := range intensions {
			p = add32(add16(p, v.ID), v.Choice)
			p = append(p, v.Status)
		}
	}
	if options == nil {
		p = append(p, 0)
	} else {
		remaining := uint32(5)
		for _, v := range options {
			if v.ID != 0 && v.Choice >= 1 && v.Choice <= 2 {
				remaining--
			}
		}
		p = add32(append(p, 1), remaining)
		for _, v := range options {
			p = add32(add16(p, v.ID), v.Choice)
			p = append(p, v.Status, v.Empty)
		}
	}
	return p, nil
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
