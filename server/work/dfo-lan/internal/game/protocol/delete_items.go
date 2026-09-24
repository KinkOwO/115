package protocol

import (
	"encoding/binary"
	"fmt"
	"math"
)

// ItemDelete describes one row of a general inventory deletion. The current
// CMD18 body is a length-prefixed protobuf that the old build admitted only
// for the skill-material branch (slot 121..176, template 3037); general
// deletion is the same wire shape with arbitrary main-bag rows.
type ItemDelete struct {
	Slot            uint16
	Template, Count uint32
}

// DecodeDeleteItems parses the current CMD18 length-prefixed delete protobuf
// for general inventory deletion.
//
//	outer: field2 (tag16) list type varint, must be 0 (main bag)
//	       field3 (tag26) nested entry bytes, repeated
//	       field4 (tag32) condition varint, must be 0
//	entry: field1 (tag8)  op type (1=general delete, 2=material consume)
//	       field2 (tag16) slot index
//	       field3 (tag24) item template id
//	       field4 (tag32) count
func DecodeDeleteItems(p []byte) ([]ItemDelete, error) {
	fail := func() ([]ItemDelete, error) { return nil, fmt.Errorf("invalid item deletion") }
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
	var rows []ItemDelete
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
			if len(fields) != 4 {
				return fail()
			}
			op := fields[8]
			if op != 1 && op != 2 {
				return fail()
			}
			slot := fields[16]
			if slot == 0 || slot > 176 {
				return fail()
			}
			tpl := fields[24]
			if tpl == 0 || tpl > math.MaxUint32 {
				return fail()
			}
			cnt := fields[32]
			if cnt == 0 || cnt > 100000 {
				return fail()
			}
			r := ItemDelete{uint16(slot), uint32(tpl), uint32(cnt)}
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

// DeleteItemsReply builds the current CMD18 success ACK in the same shape the
// validated skill-material branch uses: status byte + 32-bit protobuf length
// prefix + {8 0, 24 0} header + one nested row per deleted item (slot, count,
// reason 2). reason stays 2 so the native 0x0012 success handler renders the
// discard as a normal deletion.
func DeleteItemsReply(rows []ItemDelete, success bool) []byte {
	body := []byte{8, 0, 24, 0}
	for _, r := range rows {
		row := binary.AppendUvarint([]byte{8}, uint64(r.Slot))
		row = binary.AppendUvarint(append(row, 16), uint64(r.Count))
		row = append(row, 24, 2)
		body = binary.AppendUvarint(append(body, 18), uint64(len(row)))
		body = append(body, row...)
	}
	header := []byte{1}
	if !success {
		// 失败分发仍会读取通用 u16 错误码，之后才进入 CMD18 protobuf reader。
		header = Refusal(0)
	}
	p := add32(header, uint32(len(body)))
	return append(p, body...)
}
