package protocol

import (
	"encoding/binary"
	"fmt"
)

// NOTI2856 copies a fixed 70-u32 list at 1402ce896 into the Odyssey
// journal. Consumers stop at zero; this is not a count-prefixed vector.
func OdysseyCharacterProgress(ids []uint32) ([]byte, error) {
	if len(ids) > 70 {
		return nil, fmt.Errorf("Odyssey journal exceeds 70 entries")
	}
	b := make([]byte, 280)
	seen := map[uint32]bool{}
	for i, id := range ids {
		if id == 0 || seen[id] {
			return nil, fmt.Errorf("invalid Odyssey journal entry")
		}
		seen[id] = true
		binary.LittleEndian.PutUint32(b[i*4:], id)
	}
	return b, nil
}
