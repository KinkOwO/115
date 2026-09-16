package protocol

import (
	"encoding/binary"
	"fmt"
)

// NOTI21 current1452c3100 ->PB1401a1d70. Field2 is the character
// level; field4 is the complete eligible/accepted list. No guessed enum1.
func AvailableQuests(level byte, ids []uint32) ([]byte, error) {
	if level == 0 || len(ids) > 4000 {
		return nil, fmt.Errorf("invalid available quest list")
	}
	p := binary.AppendUvarint([]byte{16}, uint64(level))
	p = append(p, 24, 0)
	var packed []byte
	seen := map[uint32]bool{}
	for _, id := range ids {
		if id == 0 || id >= 40000 || seen[id] {
			return nil, fmt.Errorf("invalid available quest identity")
		}
		seen[id] = true
		packed = binary.AppendUvarint(packed, uint64(id))
	}
	if len(packed) > 0 {
		p = binary.AppendUvarint(append(p, 34), uint64(len(packed)))
		p = append(p, packed...)
	}
	return append(add32(nil, uint32(len(p))), p...), nil
}
