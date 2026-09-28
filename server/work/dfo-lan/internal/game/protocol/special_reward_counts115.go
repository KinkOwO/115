package protocol

import (
	"encoding/binary"
	"fmt"
)

const NotiDungeonSpecialRewardInfo115 uint16 = 1726

type SpecialRewardRemaining115 struct {
	Dungeon   uint32
	Remaining byte
}

// 142DA3700 clears BOTH maps, then reads u8 N, N*(u32,u8), u8 M,
// M*(u32,u8). This is a complete snapshot, not an incremental per-dungeon
// notification. The first map supplies142DA3A40's available reward count.
// The second map's business meaning is unresolved; preserve separate input.
func DungeonSpecialRewardInfo115(first, second []SpecialRewardRemaining115) ([]byte, error) {
	if len(first) > 255 || len(second) > 255 {
		return nil, fmt.Errorf("special reward snapshot exceeds u8 row count")
	}
	out := make([]byte, 0, 2+5*(len(first)+len(second)))
	for _, rows := range [][]SpecialRewardRemaining115{first, second} {
		out = append(out, byte(len(rows)))
		seen := map[uint32]bool{}
		for _, r := range rows {
			if r.Dungeon == 0 || r.Dungeon > 0x7fffffff || seen[r.Dungeon] {
				return nil, fmt.Errorf("invalid/duplicate special reward key")
			}
			seen[r.Dungeon] = true
			out = binary.LittleEndian.AppendUint32(out, r.Dungeon)
			out = append(out, r.Remaining)
		}
	}
	return out, nil
}
