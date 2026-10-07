package protocol

import (
	"encoding/binary"
	"fmt"
)

// BorderRewardInfo is NOTI2756's fixed 12-byte record. Native handler
// 1406b18d0 stores word 1/2 at +50/+54; getHellDungeonItemMaxRarity uses
// their maximum. Word 0 preserves +58's constructor state (-1); its content
// feature is not implemented and must not acquire an invented index.
func BorderRewardInfo(maximum uint32) ([]byte, error) {
	if maximum < 40 || maximum > 45 {
		return nil, fmt.Errorf("invalid Border reward grade")
	}
	p := make([]byte, 12)
	binary.LittleEndian.PutUint32(p, 0xffffffff)
	binary.LittleEndian.PutUint32(p[4:], maximum)
	binary.LittleEndian.PutUint32(p[8:], 40)
	return p, nil
}
