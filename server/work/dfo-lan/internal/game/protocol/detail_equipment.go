package protocol

import (
	"encoding/binary"
	"fmt"
)

// DetailedWorn is the mode1/1452c1540 avatar representation, not NOTI13.
// Only avatar slots are initialized here; other item types have different
// template-dependent extensions. Rich records require a separate projection.
type DetailedWorn struct {
	Slot          uint16 `json:"slot"`
	Template      uint32 `json:"template"`
	Durability    uint16 `json:"durability"`
	Period        uint32 `json:"period,omitempty"`
	AvatarOptions []byte `json:"avatar_options,omitempty"`
	AvatarSockets []byte `json:"avatar_sockets,omitempty"`
	Record        []byte `json:"record,omitempty"`
}

func DetailedEquipment(rows []DetailedWorn) ([]byte, error) {
	if len(rows) > 12 {
		return nil, fmt.Errorf("too many detailed worn items")
	}
	p := []byte{byte(len(rows))}
	seen := map[uint16]bool{}
	for _, v := range rows {
		if v.Slot > 11 || v.Template == 0 || seen[v.Slot] || len(v.Record) != 0 {
			return nil, fmt.Errorf("unsupported detailed equipment instance")
		}
		if len(v.AvatarOptions) > 4096 || len(v.AvatarSockets) > 4096 {
			return nil, fmt.Errorf("oversized avatar data")
		}
		seen[v.Slot] = true
		row := make([]byte, 40)
		row[0] = byte(v.Slot)
		binary.LittleEndian.PutUint32(row[1:], v.Template)
		binary.LittleEndian.PutUint16(row[10:], v.Durability)
		p = append(p, row...)
		p = append(add32(p, uint32(len(v.AvatarOptions))), v.AvatarOptions...)
		p = append(add32(p, uint32(len(v.AvatarSockets))), v.AvatarSockets...)
		p = append(p, 0) // auxiliary pair count
		p = add32(p, v.Period)
		p = append(p, 0) // 1451aa420 reads a nested collection count for EVERY row.
		p = append(p, make([]byte, 81)...)
	}
	return append(p, make([]byte, 13)...), nil
}
