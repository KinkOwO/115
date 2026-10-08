package protocol

import (
	"encoding/binary"
	"fmt"
)

// Current14254C960 writes the chosen i32 at+13 and sends17B as CMD2072.
func DecodeBakalCommanderBuff(p []byte) (int, error) {
	if len(p) < 17 || len(p) > 24 {
		return 0, fmt.Errorf("invalid native commander request size")
	}
	for _, v := range p[17:] {
		if v != 0 {
			return 0, fmt.Errorf("nonzero commander request padding")
		}
	}
	index := int32(binary.LittleEndian.Uint32(p[13:17]))
	if index < 0 || index >= 5 {
		return 0, fmt.Errorf("commander index outside native namespace")
	}
	return int(index), nil
}
