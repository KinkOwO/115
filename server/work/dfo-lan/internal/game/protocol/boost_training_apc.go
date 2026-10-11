package protocol

import (
	"encoding/binary"
	"fmt"
)

// 140C13B00 writes a single true byte. Captured CMD2333 has zero cipher padding.
func DecodeBoostTrainingAPCRequest(p []byte) error {
	if len(p) < 1 || p[0] != 1 {
		return fmt.Errorf("invalid training APC request")
	}
	return padding(p[1:], 16)
}

// Merge only mode3 into the complete account settings snapshot. N1754 replaces
// the native settings map, so sending just mode3 would erase the other modes.
// 142E5C590 matches the flagged special-APC [index] at source node +0x34,
// then resolves its AIC template (node +0x20) to a client-owned cache slot.
// 142E61EB0 consumes two trailing count tables, even when both are empty.
func BoostTrainingAPCSettings(base []byte, sourceIndices []uint32) ([]byte, error) {
	if len(base) < 1 || base[0] > 4 || len(base) != 1+int(base[0])*531 {
		return nil, fmt.Errorf("invalid account APC snapshot")
	}
	if len(sourceIndices) == 0 || len(sourceIndices) > 3 {
		return nil, fmt.Errorf("invalid training APC count")
	}
	row := make([]byte, 531)
	binary.LittleEndian.PutUint16(row[0xd:], 3)
	for i := 0; i < 4; i++ {
		binary.LittleEndian.PutUint32(row[0x23+i*4:], ^uint32(0))
	}
	seen := map[uint32]bool{}
	for i, id := range sourceIndices {
		if id == 0 || id > 1<<31-1 || seen[id] {
			return nil, fmt.Errorf("invalid training special APC index")
		}
		seen[id] = true
		row[0x10+i] = 1
		binary.LittleEndian.PutUint32(row[0x17+i*4:], id)
	}
	out := []byte{0}
	done := false
	for off := 1; off < len(base); off += 531 {
		r := base[off : off+531]
		mode := binary.LittleEndian.Uint16(r[0xd:])
		if mode == 3 {
			if done {
				return nil, fmt.Errorf("duplicate training APC mode")
			}
			r = row
			done = true
		}
		out = append(out, r...)
		out[0]++
	}
	if !done {
		if out[0] >= 4 {
			return nil, fmt.Errorf("training mode cannot be appended")
		}
		out = append(out, row...)
		out[0]++
	}
	return append(out, 0, 0), nil
}
