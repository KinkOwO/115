package protocol

import (
	"encoding/binary"
	"fmt"
	"math"
)

// Native144CE0120: u8 raids; per raid u8 kind, three u32, u8 subrecords;
// per subrecord u8 key and three u32. N1434 replaces its quota cache.
// 144CF16E0(kind,1) reads the second outer u32 (used weekly entries).
// 144CF3450(kind,1,0) reads key0's second u32 (used weekly rewards).
// Creation UI1420B2C90 subtracts these from the same native script limits;
// absent rows return -1 and incorrectly produce the observed "2/1".
// Keys1/2 and the unconsumed fields retain the captured native zero values.
func RaidWeeklyClearInfo115(kind byte, clears, rewards uint32) ([]byte, error) {
	if clears > math.MaxInt32 || rewards > math.MaxInt32 {
		return nil, fmt.Errorf("invalid native raid weekly counters")
	}
	p := []byte{1, kind}
	p = binary.LittleEndian.AppendUint32(p, 0)
	p = binary.LittleEndian.AppendUint32(p, clears)
	p = binary.LittleEndian.AppendUint32(p, 0)
	p = append(p, 3)
	for key := byte(0); key < 3; key++ {
		p = append(p, key)
		var reward uint32
		if key == 0 {
			reward = rewards
		}
		p = binary.LittleEndian.AppendUint32(p, 0)
		p = binary.LittleEndian.AppendUint32(p, reward)
		p = binary.LittleEndian.AppendUint32(p, 0)
	}
	return p, nil
}
