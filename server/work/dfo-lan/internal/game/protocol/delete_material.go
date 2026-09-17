package protocol

import (
	"encoding/binary"
	"fmt"
)

type MaterialDelete struct {
	Slot            uint16
	Template, Count uint32
}

// Current CMD18 is length-prefixed PB_ENUM_CMDPACKET_DELETE_ITEM.
// Only the observed ordinary-bag skill-cost branch (reason2, material3037)
// is admitted, not general deletion, equipment, or other protobuf actions.
func DecodeMaterialDelete(p []byte) ([]MaterialDelete, error) {
	fail := func() ([]MaterialDelete, error) { return nil, fmt.Errorf("invalid skill material deletion") }
	if len(p) < 4 {
		return fail()
	}
	n := int(binary.LittleEndian.Uint32(p))
	if n < 1 || n > 4096 || n > len(p)-4 {
		return fail()
	}
	for _, b := range p[4+n:] {
		if b != 0 {
			return fail()
		}
	}
	data := p[4 : 4+n]
	var rows []MaterialDelete
	seen := map[uint16]bool{}
	inventory, mode := false, false
	for len(data) > 0 {
		tag, k := binary.Uvarint(data)
		if k <= 0 {
			return fail()
		}
		data = data[k:]
		v, k := binary.Uvarint(data)
		if k <= 0 {
			return fail()
		}
		data = data[k:]
		switch tag {
		case 16:
			if inventory || v != 0 {
				return fail()
			}
			inventory = true
		case 32:
			if mode || v != 0 {
				return fail()
			}
			mode = true
		case 26:
			if v > uint64(len(data)) || len(rows) >= 56 {
				return fail()
			}
			row := data[:int(v)]
			data = data[int(v):]
			fields := map[uint64]uint64{}
			for len(row) > 0 {
				t, a := binary.Uvarint(row)
				if a <= 0 {
					return fail()
				}
				row = row[a:]
				x, b := binary.Uvarint(row)
				if b <= 0 {
					return fail()
				}
				row = row[b:]
				if _, ok := fields[t]; ok {
					return fail()
				}
				fields[t] = x
			}
			if len(fields) != 4 || fields[8] != 2 || fields[16] < 121 || fields[16] > 176 || fields[24] != 3037 || fields[32] == 0 || fields[32] > 1000 {
				return fail()
			}
			r := MaterialDelete{uint16(fields[16]), 3037, uint32(fields[32])}
			if seen[r.Slot] {
				return fail()
			}
			seen[r.Slot] = true
			rows = append(rows, r)
		default:
			return fail()
		}
	}
	if !inventory || !mode || len(rows) == 0 {
		return fail()
	}
	return rows, nil
}

// Native parsers140196000/140196270: RETURN{space1,rows2,error3},
// row{slot1,amount2,reason3}. Handler145272030 decrements the pending cube
// reservation in145ad2360 as well as the displayed stack.
func MaterialDeleteReply(rows []MaterialDelete, success bool) []byte {
	body := []byte{8, 0, 24, 0}
	for _, r := range rows {
		row := binary.AppendUvarint([]byte{8}, uint64(r.Slot))
		row = binary.AppendUvarint(append(row, 16), uint64(r.Count))
		row = append(row, 24, 2)
		body = binary.AppendUvarint(append(body, 18), uint64(len(row)))
		body = append(body, row...)
	}
	status := byte(0)
	if success {
		status = 1
	}
	p := add32([]byte{status}, uint32(len(body)))
	return append(p, body...)
}
