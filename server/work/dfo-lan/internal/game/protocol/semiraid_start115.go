package protocol

import (
	"encoding/binary"
	"fmt"
)

const SemiRaidStartCommand uint16 = 2284

// Current sender141331BC0 writes one u32 (24 for Moon Lake). It does NOT
// write the party mode27, channel101, or the reward-count key100004137.
func DecodeSemiRaidStart115(p []byte) (uint32, error) {
	if len(p) < 4 || len(p) > 19 {
		return 0, fmt.Errorf("invalid semi-raid start length")
	}
	for _, v := range p[4:] {
		if v != 0 {
			return 0, fmt.Errorf("unsupported semi-raid start suffix")
		}
	}
	return binary.LittleEndian.Uint32(p), nil
}
