package protocol

import (
	"encoding/binary"
	"fmt"
)

type RecastAvatarItem struct {
	Slot     uint16
	Template uint32
}

type RecastAvatarRequest struct {
	Mode   uint16
	Items  []RecastAvatarItem
	Option uint16
}

// Current sender 1413F0550: u16 mode, fixed 6/1 (slot,template) pairs,
// then the ability combo's u16 item value. Padding belongs to transport.
func DecodeRecastAvatar(p []byte) (RecastAvatarRequest, error) {
	var r RecastAvatarRequest
	if len(p) < 2 {
		return r, fmt.Errorf("short avatar recast request")
	}
	r.Mode = binary.LittleEndian.Uint16(p)
	count := 1
	switch r.Mode {
	case 0:
		count = 6
	case 1:
	default:
		return r, fmt.Errorf("invalid avatar recast mode")
	}
	size := 4 + count*6
	if len(p) != size && len(p) != (size+7)&^7 {
		return r, fmt.Errorf("invalid avatar recast length")
	}
	for _, v := range p[size:] {
		if v != 0 {
			return r, fmt.Errorf("nonzero avatar recast padding")
		}
	}
	seen := map[uint16]bool{}
	for i := 0; i < count; i++ {
		o := 2 + i*6
		item := RecastAvatarItem{binary.LittleEndian.Uint16(p[o:]), binary.LittleEndian.Uint32(p[o+2:])}
		if item.Template == 0 || item.Slot == 65535 || seen[item.Slot] {
			return r, fmt.Errorf("invalid or duplicate avatar recast target")
		}
		seen[item.Slot] = true
		r.Items = append(r.Items, item)
	}
	r.Option = binary.LittleEndian.Uint16(p[2+count*6:])
	return r, nil
}

type RecastAvatarResult struct {
	Slot     uint16 `json:"slot"`
	Template uint32 `json:"template"`
	Instance uint32 `json:"instance"`
	Option   uint16 `json:"option"`
	Options  []byte `json:"options,omitempty"`
	Sockets  []byte `json:"sockets,omitempty"`
}

// Native CMD795 reader 145266E10 consumes removed (space,slot,count) entries,
// followed by a compact avatar and two length-prefixed fixed-capacity blocks.
// This result has no separate period word (unlike NOTI13's full avatar rows).
func RecastAvatarSuccess(r RecastAvatarRequest, result RecastAvatarResult) ([]byte, error) {
	if len(r.Items) == 0 || len(r.Items) > 6 || result.Slot == 65535 || result.Template == 0 || result.Option != r.Option || len(result.Options) > 30 || len(result.Sockets) > 4 {
		return nil, fmt.Errorf("invalid avatar recast receipt")
	}
	p := []byte{1, byte(len(r.Items))}
	for _, item := range r.Items {
		if item.Template == 0 || item.Slot == 65535 {
			return nil, fmt.Errorf("invalid avatar recast removal")
		}
		p = add16(append(p, 1), item.Slot)
		p = append(p, 1)
	}
	p = add16(p, result.Slot)
	p = add32(p, result.Template)
	p = add32(p, result.Instance)
	p = add16(p, result.Option)
	p = append(add32(p, uint32(len(result.Options))), result.Options...)
	p = append(add32(p, uint32(len(result.Sockets))), result.Sockets...)
	return p, nil
}

// Emblems are ordinary items and must never be inserted through CMD795's
// avatar-result field. Native factory 14603E2D0 treats template -1 as the
// explicit empty object without its missing-template warning. The reader
// still removes the inputs and completes pending window 537. NOTI14 carries
// actual emblem rows separately, using its established ordinary-item reader.
func RecastAvatarConsumed(r RecastAvatarRequest) ([]byte, error) {
	if r.Mode != 1 || len(r.Items) != 1 || r.Items[0].Template == 0 || r.Items[0].Slot == 65535 {
		return nil, fmt.Errorf("invalid avatar recast removal")
	}
	p := []byte{1, 1, 1}
	p = add16(p, r.Items[0].Slot)
	p = append(p, 1)
	p = add16(p, 65535)
	p = add32(p, ^uint32(0))
	p = add32(p, 0)
	p = add16(p, 0)
	p = add32(p, 0)
	return add32(p, 0), nil
}

// Native CMD1807 reader 145278000 opens reward window 2706, shared with
// avatar disjoint CMD202. This presentation-only result reads u16 count
// followed by (u32 template,u32 awarded quantity); it never reads bag slots
// or modifies inventory. The recast ACK and NOTI14 carry actual changes.
func AvatarRewardPopup(rewards []DisjointRewardEntry) ([]byte, error) {
	if len(rewards) == 0 || len(rewards) > 65535 {
		return nil, fmt.Errorf("invalid avatar reward popup")
	}
	p := add16([]byte{1}, uint16(len(rewards)))
	for _, reward := range rewards {
		if reward.Template <= 1 || reward.Template == ^uint32(0) || reward.Count == 0 {
			return nil, fmt.Errorf("invalid avatar popup reward")
		}
		p = add32(p, reward.Template)
		p = add32(p, reward.Count)
	}
	return p, nil
}
