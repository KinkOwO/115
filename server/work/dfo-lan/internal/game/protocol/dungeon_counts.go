package protocol

import (
	"encoding/binary"
	"fmt"
)

// NOTI537: current1452CBE20 reads dungeon/u8/u8 and writes the actor's two
// counter maps. Test override is explicit; it is not a persisted weekly ledger.
func DungeonTestRemaining(dungeon, configured uint32) ([]byte, error) {
	if dungeon == 0 || configured == 0 {
		return nil, fmt.Errorf("explicit dungeon/count override required")
	}
	projected := byte(min(configured, 255))
	p := binary.LittleEndian.AppendUint32(nil, dungeon)
	return append(p, projected, projected), nil
}
