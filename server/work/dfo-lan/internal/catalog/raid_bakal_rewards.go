package catalog

import (
	"dfolan/internal/catalog/pvf"
	"fmt"
)

// The source contains separate normal, hard and solo/APC reward sections.
// Keep all normal categories, even categories that require an auction/PC-room.
type BakalRewardEntry struct {
	Kind         string
	Group        int32
	Weight       uint32
	Template     int32
	Grade, Extra int32
}

func parseBakalRewards(ts []pvf.Token, dungeon uint32) ([]BakalRewardEntry, error) {
	var out []BakalRewardEntry
	matched, normal := false, false
	for i := 0; i < len(ts); i++ {
		switch ts[i].Text {
		case "[RAID]", "[/RAID]":
			matched, normal = false, false
		case "[REPRESENTATIVE DUNGEON INDEX]":
			if i+1 >= len(ts) || ts[i+1].Type != 0 {
				return nil, fmt.Errorf("invalid representative raid dungeon")
			}
			matched = uint32(ts[i+1].Value) == dungeon
		case "[PHASE]":
			normal = matched
		case "[/PHASE]", "[RAID PHASE OF HARDMODE]", "[SINGLE RAID PHASE]", "[SINGLE APC RAID PHASE]", "[PHASE DIFFICULTY]":
			normal = false
		case "[STATE REWARD]":
			if !normal {
				continue
			}
			if i+6 >= len(ts) || ts[i+1].Type != 6 {
				return nil, fmt.Errorf("invalid native raid reward row")
			}
			for j := 2; j <= 6; j++ {
				if ts[i+j].Type != 0 {
					return nil, fmt.Errorf("invalid native reward scalar")
				}
			}
			if ts[i+3].Value <= 0 || ts[i+4].Value == 0 || ts[i+4].Value < -1 {
				return nil, fmt.Errorf("invalid reward weight/template")
			}
			out = append(out, BakalRewardEntry{ts[i+1].Text, ts[i+2].Value, uint32(ts[i+3].Value), ts[i+4].Value, ts[i+5].Value, ts[i+6].Value})
			i += 6
		}
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("native normal raid rewards missing for %d", dungeon)
	}
	return out, nil
}
