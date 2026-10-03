package protocol

import (
	"encoding/binary"
	"fmt"
)

// CMD1421 sender 1436436D0 writes six bytes; live 20260930 bodies have
// two zero alignment bytes. Kind 48/list 46/slot FFFF selects the skill;
// the same list/slot with an equipment kind clears that registration.
type BuffEnhancementRequest struct {
	Skill      uint16
	Kind, List byte
	Slot       uint16
}

func DecodeBuffEnhancement(p []byte) (BuffEnhancementRequest, error) {
	var r BuffEnhancementRequest
	if len(p) != 6 && len(p) != 8 {
		return r, fmt.Errorf("buff enhancement requires six-byte body")
	}
	for _, b := range p[6:] {
		if b != 0 {
			return r, fmt.Errorf("nonzero buff enhancement padding")
		}
	}
	r = BuffEnhancementRequest{binary.LittleEndian.Uint16(p), p[2], p[3], binary.LittleEndian.Uint16(p[4:])}
	if r.Kind > 26 && r.Kind != 48 {
		return r, fmt.Errorf("unsupported buff equipment kind %d", r.Kind)
	}
	if r.List == 46 && r.Slot == 65535 {
		return r, nil
	}
	if r.Kind == 48 || (r.List != 0 && r.List != 1 && r.List != 3 && r.List != 7) || r.Slot == 65535 {
		return r, fmt.Errorf("unsupported buff equipment reference")
	}
	return r, nil
}

func (r BuffEnhancementRequest) EmptyReference() bool { return r.List == 46 && r.Slot == 65535 }

type BuffEnhancementItem struct {
	Exists     bool
	Kind, List byte
	Slot       uint16
}

// NOTI1361 14363EED0 reads skill then calls 14363E4B0. The latter reads
// count + count*(exists:u8, kind:u8, list:u8, slot:u16), resolving live items.
func BuffEnhancementAllData(skill uint16, items []BuffEnhancementItem) ([]byte, error) {
	if len(items) > 255 {
		return nil, fmt.Errorf("too many buff enhancement items")
	}
	p := binary.LittleEndian.AppendUint16(nil, skill)
	p = append(p, byte(len(items)))
	seen := map[byte]bool{}
	for _, i := range items {
		if i.Kind > 26 || seen[i.Kind] {
			return nil, fmt.Errorf("invalid or duplicate buff equipment kind")
		}
		seen[i.Kind] = true
		exists := byte(0)
		if i.Exists {
			exists = 1
		}
		p = append(p, exists, i.Kind, i.List)
		p = binary.LittleEndian.AppendUint16(p, i.Slot)
	}
	return p, nil
}
