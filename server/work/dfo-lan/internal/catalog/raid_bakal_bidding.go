package catalog

import (
	"dfolan/internal/catalog/pvf"
	"fmt"
	"math"
)

type BakalWeightedNumber struct{ Value, Weight uint32 }
type BakalBiddingItem struct {
	Template, Weight uint32
	Attributes       [5]int32
}

// Timings and pools are read from the same archive as the encounter. These
// definitions do not authorize an item grant without a successful gold bid.
type BakalBiddingRules struct {
	Count, WeeklyCount, WeeklyGroups                   []BakalWeightedNumber
	WeeklyItems                                        map[uint32][]BakalBiddingItem
	StartDelay, BreakTime, Time, WaitTime, WarningTime uint32
	DecreaseAfterBids, LowestTime                      uint32
	DecreasePerBid                                     float32
	RewardTimes                                        []uint32
	GoldCards                                          []uint32
}

func bakalIntegerBlock(ts []pvf.Token, start int, end string) ([]uint32, int, error) {
	var out []uint32
	for i := start; i < len(ts); i++ {
		if ts[i].Text == end {
			return out, i, nil
		}
		if ts[i].Type != 0 || ts[i].Value < 0 {
			return nil, i, fmt.Errorf("invalid native bidding integer before %s", end)
		}
		out = append(out, uint32(ts[i].Value))
	}
	return nil, start, fmt.Errorf("unclosed native bidding block %s", end)
}

func bakalWeightedNumbers(values []uint32) ([]BakalWeightedNumber, error) {
	if len(values) == 0 || len(values)%2 != 0 {
		return nil, fmt.Errorf("invalid bidding rate pairs")
	}
	out := make([]BakalWeightedNumber, 0, len(values)/2)
	seen := map[uint32]bool{}
	var total uint64
	for i := 0; i < len(values); i += 2 {
		if seen[values[i]] {
			return nil, fmt.Errorf("duplicate bidding rate value")
		}
		seen[values[i]] = true
		total += uint64(values[i+1])
		out = append(out, BakalWeightedNumber{values[i], values[i+1]})
	}
	if total == 0 || total > math.MaxUint32 {
		return nil, fmt.Errorf("invalid bidding total weight")
	}
	return out, nil
}

// A normal difficulty count lives inside [HARD] 0. Parsing both difficulties
// into one distribution would double the probability mass.
func bakalNormalBiddingCount(ts []pvf.Token, section string) ([]BakalWeightedNumber, error) {
	in, hard := false, int32(-1)
	for i := 0; i < len(ts); i++ {
		switch ts[i].Text {
		case section:
			in = true
		case "[/" + section[1:]:
			in = false
		case "[HARD]":
			if !in {
				continue
			}
			if i+1 >= len(ts) || ts[i+1].Type != 0 {
				return nil, fmt.Errorf("invalid bidding difficulty")
			}
			hard = ts[i+1].Value
		case "[/HARD]":
			hard = -1
		case "[RATE LIST]":
			if !in || hard != 0 {
				continue
			}
			values, _, err := bakalIntegerBlock(ts, i+1, "[/RATE LIST]")
			if err != nil {
				return nil, err
			}
			return bakalWeightedNumbers(values)
		}
	}
	return nil, fmt.Errorf("normal bidding distribution missing: %s", section)
}

func loadBakalBidding(a *pvf.Archive, rules []pvf.Token) (BakalBiddingRules, error) {
	b := BakalBiddingRules{WeeklyItems: map[uint32][]BakalBiddingItem{}}
	var err error
	b.Count, err = bakalNormalBiddingCount(rules, "[BIDDING REWARD COUNT]")
	if err != nil {
		return b, err
	}
	scalars := map[string]*uint32{
		"[BIDDING START DELAY TIME]": &b.StartDelay, "[BIDDING BREAK TIME]": &b.BreakTime,
		"[BIDDING TIME]": &b.Time, "[BIDDING WAIT TIME]": &b.WaitTime,
		"[BIDDING WARNING TIME]": &b.WarningTime, "[DECREASE BIDDING TIME BID COUNT]": &b.DecreaseAfterBids,
		"[LOWEST BIDDING TIME]": &b.LowestTime,
	}
	seen := map[string]bool{}
	for i, t := range rules {
		if t.Text == "[PHASE]" {
			break
		}
		if ptr, ok := scalars[t.Text]; ok {
			if seen[t.Text] || i+1 >= len(rules) || rules[i+1].Type != 0 || rules[i+1].Value < 0 {
				return b, fmt.Errorf("invalid bidding scalar %s", t.Text)
			}
			*ptr, seen[t.Text] = uint32(rules[i+1].Value), true
		}
		if t.Text == "[DECREASE BIDDING TIME PER BID]" {
			if seen[t.Text] || i+1 >= len(rules) || rules[i+1].Type != 2 {
				return b, fmt.Errorf("invalid bidding decrease")
			}
			b.DecreasePerBid = rules[i+1].Number
			seen[t.Text] = true
		}
	}
	if len(seen) != len(scalars)+1 || b.Time == 0 || b.LowestTime > b.Time || b.DecreasePerBid < 0 || math.IsNaN(float64(b.DecreasePerBid)) || math.IsInf(float64(b.DecreasePerBid), 0) {
		return b, fmt.Errorf("incomplete bidding timings")
	}
	ts, err := a.Tokens("contents/2022/bakalraid/etc/bakalweeklybidding.etc")
	if err != nil {
		return b, err
	}
	b.WeeklyCount, err = bakalNormalBiddingCount(ts, "[WEEKLY BIDDING REWARD COUNT]")
	if err != nil {
		return b, err
	}
	var group uint32
	inItems := false
	for i := 0; i < len(ts); i++ {
		switch ts[i].Text {
		case "[WEEKLY BIDDING RATE]":
			values, last, e := bakalIntegerBlock(ts, i+1, "[/WEEKLY BIDDING RATE]")
			if e != nil {
				return b, e
			}
			b.WeeklyGroups, err = bakalWeightedNumbers(values)
			if err != nil {
				return b, err
			}
			i = last
		case "[WEEKLY BIDDING ITEM]":
			inItems = true
		case "[/WEEKLY BIDDING ITEM]":
			inItems = false
		case "[GROUP]":
			if !inItems {
				continue
			}
			if i+1 >= len(ts) || ts[i+1].Type != 0 || ts[i+1].Value <= 0 {
				return b, fmt.Errorf("invalid weekly bidding group")
			}
			group = uint32(ts[i+1].Value)
			if _, exists := b.WeeklyItems[group]; exists {
				return b, fmt.Errorf("duplicate weekly bidding group")
			}
		case "[ITEM]":
			if !inItems || group == 0 {
				return b, fmt.Errorf("bidding item outside group")
			}
			values, last, e := bakalIntegerBlock(ts, i+1, "[/ITEM]")
			if e != nil {
				return b, e
			}
			if len(values) == 0 || len(values)%7 != 0 {
				return b, fmt.Errorf("invalid weekly bidding item row")
			}
			for j := 0; j < len(values); j += 7 {
				if values[j] == 0 || values[j+1] == 0 {
					return b, fmt.Errorf("invalid weekly bidding item weight/template")
				}
				item := BakalBiddingItem{Template: values[j], Weight: values[j+1]}
				for k := range item.Attributes {
					item.Attributes[k] = int32(values[j+2+k])
				}
				b.WeeklyItems[group] = append(b.WeeklyItems[group], item)
			}
			i = last
		case "[/GROUP]":
			group = 0
		}
	}
	if len(b.WeeklyGroups) == 0 {
		return b, fmt.Errorf("weekly bidding group rates missing")
	}
	for _, g := range b.WeeklyGroups {
		if len(b.WeeklyItems[g.Value]) == 0 {
			return b, fmt.Errorf("weekly bidding rate refers to missing group %d", g.Value)
		}
	}
	ts, err = readBakalCOSTokens(a, "contents/2022/bakalraid/etc/bakalreward.cos")
	if err != nil {
		return b, err
	}
	for i, t := range ts {
		switch t.Text {
		case "[reward time info]":
			b.RewardTimes, _, err = bakalIntegerBlock(ts, i+1, "[/reward time info]")
		case "[bidding gold card list]":
			b.GoldCards, _, err = bakalIntegerBlock(ts, i+1, "[/bidding gold card list]")
		}
		if err != nil {
			return b, err
		}
	}
	if len(b.RewardTimes) == 0 || len(b.GoldCards) == 0 {
		return b, fmt.Errorf("native bidding presentation missing")
	}
	return b, nil
}
