package protocol

import (
	"encoding/binary"
	"fmt"
)

type TutorialChange struct {
	Index     byte
	Completed bool
}

// CMD143 sender 146cc7275..72b2 writes u8 operation(0), u32 index,
// u8 completed, followed by slot3 padding. This records UI guide progress,
// not a dungeon-clear result or a reward claim.
func DecodeTutorialChange(p []byte) (TutorialChange, error) {
	var out TutorialChange
	if len(p) < 6 || p[0] != 0 || p[5] > 1 {
		return out, fmt.Errorf("unsupported tutorial change")
	}
	index := binary.LittleEndian.Uint32(p[1:])
	if index >= 101 {
		return out, fmt.Errorf("tutorial index outside native array")
	}
	if e := padding(p[6:], 16); e != nil {
		return out, e
	}
	out.Index = byte(index)
	out.Completed = p[5] == 1
	return out, nil
}

// Native success handler 146cc6e80 reads an optional reward count. This
// presentation-only acknowledgement has zero rewards; no tutorial completion
// or dungeon rewards are invented by persisting a watched guide.
func TutorialChangeSaved() []byte { return []byte{1, 0} }
