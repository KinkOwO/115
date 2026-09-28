package protocol

import "fmt"

// 军团（legion）奖励行与征讨（Conquest Clear reward）共用本文件的类型。
// 捐赠者主线里两条路径都在此文件；上游没有这些类型，故随补丁新增。

// Value is the item's native row value, NOT universally a quantity. Equipment
// uses an instance value (G0260 includes476296939 and999999999); a caller must
// never feed it to Bag.Add as an item count. Rows must come from frozen grants.
type ConquestRewardValue115 struct {
	Equipment bool // validation only; fresh instance value0 is valid, stack quantity0 is not
	Template  uint32
	Value     uint32
	Metadata  [21]byte
}

type ConquestClearReward115 struct {
	Present [8]bool
	Rewards [8][]ConquestRewardValue115
}

// Wire-only constructor for the sourced [disable clear reward]/special card
// path. It grants nothing. Ordinary ClearReward keeps its existing contract.
// Native1452A74C0 reads the special29B rows in the first of8 extra groups.
func ConquestClearRewardBody115(s ConquestClearReward115) ([]byte, error) {
	base := make([]byte, 281)
	base[167] = 1
	for i := 265; i < 269; i++ {
		base[i] = 255
	}
	for i := 277; i < 281; i++ {
		base[i] = 255
	}
	var first, extra []byte
	present := false
	for slot, active := range s.Present {
		if !active {
			if len(s.Rewards[slot]) != 0 {
				return nil, fmt.Errorf("conquest reward for absent seat")
			}
			first = append(first, 0)
			extra = append(extra, 0)
			continue
		}
		present = true
		if len(s.Rewards[slot]) == 0 || len(s.Rewards[slot]) > 126 {
			return nil, fmt.Errorf("conquest present seat needs bounded frozen rewards")
		}
		// Observed template0/value0 placeholders are UI rows, not gold awards.
		first = append(first, 1)
		first = append(first, make([]byte, 29)...)
		extra = append(extra, byte(1+len(s.Rewards[slot])))
		extra = append(extra, make([]byte, 29)...)
		for _, row := range s.Rewards[slot] {
			if row.Template == 0 || row.Value == 0 && !row.Equipment {
				return nil, fmt.Errorf("invalid committed conquest reward row")
			}
			extra = add32(add32(extra, row.Template), row.Value)
			extra = append(extra, row.Metadata[:]...)
		}
	}
	if !present {
		return nil, fmt.Errorf("empty conquest reward roster")
	}
	p := append([]byte(nil), base[:131]...)
	p = append(p, first...)
	p = append(p, base[139:193]...)
	p = append(p, extra...)
	return append(p, base[201:]...), nil
}
