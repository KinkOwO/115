package protocol

import (
	"encoding/binary"
	"fmt"
)

// CMD20 SORT_ITEM asks the server to adopt the arrangement the client already
// applied locally. Captured body (768 bytes, plaintext 2026-09-21):
//
//	+0   u8  list                     (0 = ordinary inventory)
//	+1   u8  ?                        (1 in the capture)
//	+2   u16 slot count               (380 in the capture)
//	+4       count x u16 permutation  (slot i takes the item of slot perm[i])
//	+4+2n    4 zero bytes
//
// The table is total over 0..count-1: the capture has 376 identity entries and
// one four cycle over 18..22 (18->21, 19->22, 21->18, 22->19).
type SortItemRequest struct {
	List  byte
	Slots []uint16
}

func DecodeSortItem(p []byte) (SortItemRequest, error) {
	var r SortItemRequest
	if len(p) < 8 {
		return r, fmt.Errorf("short sort item request")
	}
	r.List = p[0]
	count := int(binary.LittleEndian.Uint16(p[2:]))
	end := 4 + count*2
	if count < 2 || count > 4096 || len(p) < end+4 || len(p) > end+8 {
		return r, fmt.Errorf("unsupported sort item length")
	}
	for _, b := range p[end:] {
		if b != 0 {
			return r, fmt.Errorf("nonzero sort item padding")
		}
	}
	r.Slots = make([]uint16, count)
	seen := make([]bool, count)
	for i := 0; i < count; i++ {
		v := binary.LittleEndian.Uint16(p[4+i*2:])
		if int(v) >= count || seen[v] {
			return r, fmt.Errorf("sort item table is not a permutation")
		}
		seen[v] = true
		r.Slots[i] = v
	}
	return r, nil
}

// SortItemSuccess acknowledges the requested container, not the protobuf
// default (ordinary inventory 0). Current PE 515fefa1: CMD20 at 1452982C0 reads
// u32 length + RETURN; 140196D40 reads field 1 into the container used to select
// the pending-operation manager. See docs/protocol/sort-ack-20261010.md.
func SortItemSuccess(list byte) []byte {
	return sortItemReply([]byte{1}, list)
}

// SortItemRefusal keeps the existing outer refusal code 4. The generic command
// dispatcher consumes that u16 before CMD20 reads its length-prefixed RETURN.
// Only field 1 is supplied: no unsupported mapping to CMD20-specific field 2
// errors is invented. Its protobuf default 0 takes the native default failure
// branch, which releases the requested container's pending operation.
func SortItemRefusal(list byte) []byte {
	return sortItemReply(Refusal(4), list)
}

func sortItemReply(header []byte, list byte) []byte {
	body := binary.AppendUvarint([]byte{8}, uint64(list))
	return append(add32(header, uint32(len(body))), body...)
}
