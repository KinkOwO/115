package protocol

import (
	"encoding/binary"
	"fmt"
)

const MoonFloorRequest115 uint16 = 2062

// 146D46650 appends46 bytes, including an internal opaque13-byte prefix.
// This decoder is intentionally ONLY the observed Moon portal variant, not
// generic authorization to enter any dungeon or accept arbitrary coordinates.
func DecodeMoonFloorRequest115(p []byte) error {
	if len(p) < 46 || len(p) > 61 {
		return fmt.Errorf("invalid Moon portal request size")
	}
	if binary.LittleEndian.Uint32(p[13:17]) != 100004137 {
		return fmt.Errorf("Moon portal target is not second floor")
	}
	for _, off := range []int{17, 21, 25} {
		if binary.LittleEndian.Uint32(p[off:off+4]) != 0 {
			return fmt.Errorf("unsupported Moon portal variant")
		}
	}
	if p[45] != 0 {
		return fmt.Errorf("unsupported Moon portal mode")
	}
	// Four source presentation coordinates are NOT a trusted teleport target.
	// The server always selects the source-defined second-floor entry.
	for _, v := range p[46:] {
		if v != 0 {
			return fmt.Errorf("nonzero Moon portal padding")
		}
	}
	return nil
}

// N2281's current handler1452ACAA0 consumes38 bytes then releases the direct
// move UI latch. It does not use the copied fields as the dungeon/map authority.
// Do not copy the official sample's unidentified per-process leading values.
func MoonFloorDirectMoveNotice115() []byte { return make([]byte, 38) }
