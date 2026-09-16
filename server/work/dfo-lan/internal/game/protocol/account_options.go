package protocol

import (
	_ "embed"
	"encoding/binary"
	"fmt"
)

// NativeAccountOptions comes from constructor 1475757f0, not a zero table.
// NOTI2826 reads all 3648 bytes. Sparse flags leave other options to the
// client's original UnifiedOption.ctp defaults (merge 147578720).
//
//go:embed templates/account-options-current.bin
var nativeAccountOptions []byte

func AccountOptions(overrides map[uint16]uint16) ([]byte, error) {
	if len(nativeAccountOptions) != 3648 {
		return nil, fmt.Errorf("current account option template size mismatch")
	}
	p := append([]byte(nil), nativeAccountOptions...)
	for index, value := range overrides {
		if index >= 286 || value == 65535 {
			return nil, fmt.Errorf("invalid account option %d", index)
		}
		p[0] = 1
		binary.LittleEndian.PutUint16(p[2+int(index)*2:], value)
		p[574+int(index)] = 1
	}
	return p, nil
}
