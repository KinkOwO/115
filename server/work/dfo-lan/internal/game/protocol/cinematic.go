package protocol

import (
	"encoding/binary"
	"fmt"
)

// 143c671c0 sends one u16 scene ID. NOTI1352/143c65010 consumes a
// length-prefixed 5000-byte bitmap and unpacks exactly 40000 scene flags.
func DecodeCinematicSkip(p []byte) (uint16, error) {
	if len(p) != 2 && len(p) != 8 && len(p) != 16 {
		return 0, fmt.Errorf("cinematic request length")
	}
	if err := padding(p[2:], 16); err != nil {
		return 0, err
	}
	id := binary.LittleEndian.Uint16(p)
	if id >= 40000 {
		return 0, fmt.Errorf("cinematic scene outside bitmap")
	}
	return id, nil
}

func CinematicSkippedScenes(ids []uint16) ([]byte, error) {
	p := append(add32(nil, 5000), make([]byte, 5000)...)
	for _, id := range ids {
		if id >= 40000 {
			return nil, fmt.Errorf("cinematic scene outside bitmap")
		}
		p[4+int(id)/8] |= 1 << (id % 8)
	}
	return p, nil
}
