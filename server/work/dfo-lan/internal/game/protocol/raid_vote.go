package protocol

import (
	"encoding/binary"
	"fmt"
)

// The native raw sender includes an opaque 13-byte prefix from its local
// struct. Only choice at +13 and action at +14 are semantic. Do not echo
// that prefix, which can contain stack data. The live cipher pads18 to32.
func DecodeRaidStartVote(p []byte) (byte, error) {
	if len(p) != 18 && len(p) != 32 {
		return 0, fmt.Errorf("invalid raid vote length")
	}
	if p[13] > 2 || binary.LittleEndian.Uint32(p[14:18]) != 1 {
		return 0, fmt.Errorf("unsupported raid vote choice or action")
	}
	for _, b := range p[18:] {
		if b != 0 {
			return 0, fmt.Errorf("nonzero raid vote padding")
		}
	}
	return p[13], nil
}

// 144cdf770/144cdf8d0 consume the whole packed181-byte struct.
// Type1 is start-raid; result0=pending,1=accepted,2=rejected.
// Empty rows have actor0 and choice3. Only the real owner occupies a row.
// The initiating leader has already agreed, even while result is pending.
func RaidSoloStartVote(actor uint16, result uint32) ([]byte, error) {
	if actor == 0 || result > 2 {
		return nil, fmt.Errorf("invalid real solo vote")
	}
	p := make([]byte, 181)
	binary.LittleEndian.PutUint32(p[13:17], 1)
	binary.LittleEndian.PutUint32(p[17:21], result)
	for at := 21; at < len(p); at += 8 {
		binary.LittleEndian.PutUint32(p[at+4:at+8], 3)
	}
	binary.LittleEndian.PutUint16(p[21:23], actor)
	choice := result
	if choice == 0 {
		choice = 1
	}
	binary.LittleEndian.PutUint32(p[25:29], choice)
	return p, nil
}
