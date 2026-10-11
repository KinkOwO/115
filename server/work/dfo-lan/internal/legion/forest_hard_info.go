package legion

import (
	"encoding/binary"
	"fmt"
)

// ForestHardNativeInfo follows current reader141D42030 (54 bytes) and
// manager141D362A0. Unlike N2563, this packet has no operation-choice byte.
// The independent state/outcome/phase u32s start at2/6/10. Retain native
// constructor defaults for the opaque following field and three12-byte rows.
// No normal melody records, captured timestamps, or fabricated phase ticks
// are copied into the hard manager.
func ForestHardNativeInfo(state, outcome uint32, phase int32) ([]byte, error) {
	if phase < -1 || phase > 2 {
		return nil, fmt.Errorf("forest hard native phase %d out of range", phase)
	}
	valid := state == 14 && outcome == 4 && phase == -1 || state == 2 && outcome == 0 && phase >= 0 ||
		state == 2 && outcome == 3 && phase == 2 || state == 3 && outcome == 1 && phase == 2 || state == 5 && outcome == 1 && phase == 2
	if !valid {
		return nil, fmt.Errorf("forest hard native state/outcome/phase mismatch")
	}
	p := make([]byte, 54)
	binary.LittleEndian.PutUint16(p, 0xffff)
	binary.LittleEndian.PutUint32(p[2:], state)
	binary.LittleEndian.PutUint32(p[6:], outcome)
	binary.LittleEndian.PutUint32(p[10:], uint32(phase))
	binary.LittleEndian.PutUint32(p[14:], ^uint32(0))
	for i := 0; i < 3; i++ {
		at := 18 + 12*i
		p[at] = 0xff
		binary.LittleEndian.PutUint64(p[at+4:], ^uint64(0))
	}
	return p, nil
}
