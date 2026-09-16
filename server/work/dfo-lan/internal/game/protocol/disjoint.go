package protocol

import (
	"encoding/binary"
	"fmt"
)

type DisjointItemEntry struct {
	Slot     uint16
	Template uint32
}

type DisjointItemRequest struct {
	Mode     byte
	ToolSlot uint16
	Items    []DisjointItemEntry
}

func DecodeDisjointItem(p []byte) (DisjointItemRequest, error) {
	var r DisjointItemRequest
	if len(p) < 4 {
		return r, fmt.Errorf("short disjoint request")
	}
	r.Mode = p[0]
	r.ToolSlot = binary.LittleEndian.Uint16(p[1:3])
	count := int(p[3])
	if count == 0 {
		return r, fmt.Errorf("empty disjoint item list")
	}
	if len(p) < 4+count*6 {
		return r, fmt.Errorf("short disjoint item list")
	}
	for i := 0; i < count; i++ {
		offset := 4 + i*6
		slot := binary.LittleEndian.Uint16(p[offset : offset+2])
		template := binary.LittleEndian.Uint32(p[offset+2 : offset+6])
		if slot == 0 {
			return r, fmt.Errorf("invalid disjoint item slot")
		}
		r.Items = append(r.Items, DisjointItemEntry{Slot: slot, Template: template})
	}
	return r, nil
}

type DisjointRewardEntry struct {
	Slot     uint16
	Template uint32
	Count    uint32
}

type DisjointItemResult struct {
	DeletedSlots []uint16
	List         byte
	ToolSlot     uint16
	Rewards      []DisjointRewardEntry
}

func DisjointItemSuccess(r DisjointItemResult) ([]byte, error) {
	if len(r.DeletedSlots) == 0 || len(r.DeletedSlots) > 255 {
		return nil, fmt.Errorf("invalid disjoint deleted slots count")
	}
	if len(r.Rewards) > 255 {
		return nil, fmt.Errorf("too many disjoint rewards")
	}
	buf := []byte{1, byte(len(r.DeletedSlots))}
	for _, slot := range r.DeletedSlots {
		buf = add16(buf, slot)
	}
	buf = append(buf, r.List)
	buf = add16(buf, r.ToolSlot)
	buf = append(buf, byte(len(r.Rewards)))
	for _, reward := range r.Rewards {
		buf = add16(buf, reward.Slot)
		buf = add32(buf, reward.Template)
		buf = add32(buf, reward.Count)
	}
	return buf, nil
}

func DisjointItemRefused(code uint16) []byte {
	return Refusal(code)
}
