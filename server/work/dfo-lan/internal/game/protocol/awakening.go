package protocol

import (
	"encoding/binary"
	"fmt"
)

// 143f387ba..e7 writes a 17-byte stack structure. Only u32+13 is
// initialized by this sender; the prefix is opaque, not trusted identity.
func DecodeSystemAwakening(p []byte) (byte, error) {
	if len(p) != 17 && len(p) != 24 {
		return 0, fmt.Errorf("awakening request length")
	}
	if err := padding(p[17:], 8); err != nil {
		return 0, err
	}
	n := binary.LittleEndian.Uint32(p[13:])
	if n < 1 || n > 3 {
		return 0, fmt.Errorf("invalid awakening stage")
	}
	return byte(n), nil
}
