package catalog

import (
	"dfolan/internal/catalog/pvf"
	"fmt"
)

const BakalWeeklyBiddingSource = "contents/2022/bakalraid/etc/bakalweeklybidding.etc"

func loadBakalWeeklyBidding(a *pvf.Archive, r *BakalRaidRules) error {
	s, e := ResolveScript(a, BakalWeeklyBiddingSource)
	if e != nil {
		return e
	}
	return parseBakalWeeklyBidding(s.Cells, &r.Bidding)
}

func parseBakalWeeklyBidding(ts []pvf.Token, out *BakalBiddingRules) error {
	count, e := bakalBlock(ts, "[weekly bidding reward count]", "[/weekly bidding reward count]")
	if e != nil {
		return e
	}
	hards, e := bakalBlocks(count, "[hard]", "[/hard]")
	if e != nil {
		return e
	}
	for _, b := range hards {
		rows := bakalRows(b)
		if len(rows) == 0 || len(bakalInts(rows[0].Args)) != 1 {
			return fmt.Errorf("weekly bidding difficulty missing")
		}
		rates, e := bakalBlock(b, "[rate list]", "[/rate list]")
		if e != nil {
			return e
		}
		v := bakalInts(rates)
		if len(v) == 0 || len(v)%2 != 0 {
			return fmt.Errorf("invalid weekly count weights")
		}
		h := BakalBiddingHard{Hard: bakalInts(rows[0].Args)[0]}
		for i := 0; i < len(v); i += 2 {
			if v[i] < 0 || v[i+1] < 0 {
				return fmt.Errorf("negative weekly count")
			}
			h.Rates = append(h.Rates, BakalBiddingRate{Grade: v[i], Weight: v[i+1]})
		}
		out.WeeklyCounts = append(out.WeeklyCounts, h)
	}
	rates, e := bakalBlock(ts, "[weekly bidding rate]", "[/weekly bidding rate]")
	if e != nil {
		return e
	}
	v := bakalInts(rates)
	if len(v) == 0 || len(v)%2 != 0 {
		return fmt.Errorf("invalid weekly group weights")
	}
	for i := 0; i < len(v); i += 2 {
		if v[i] <= 0 || v[i+1] < 0 {
			return fmt.Errorf("invalid weekly group")
		}
		out.WeeklyRates = append(out.WeeklyRates, BakalBiddingRate{Grade: v[i], Weight: v[i+1]})
	}
	pool, e := bakalBlock(ts, "[weekly bidding item]", "[/weekly bidding item]")
	if e != nil {
		return e
	}
	groups, e := bakalBlocks(pool, "[group]", "[/group]")
	if e != nil {
		return e
	}
	out.WeeklyGroups = map[int][]BakalBiddingItem{}
	for _, g := range groups {
		rows := bakalRows(g)
		if len(rows) == 0 || len(bakalInts(rows[0].Args)) != 1 {
			return fmt.Errorf("invalid weekly group header")
		}
		id := bakalInts(rows[0].Args)[0]
		if _, dup := out.WeeklyGroups[id]; dup {
			return fmt.Errorf("duplicate weekly group")
		}
		items, e := bakalBlock(g, "[item]", "[/item]")
		if e != nil {
			return e
		}
		values := bakalInts(items)
		if len(values) == 0 || len(values)%7 != 0 {
			return fmt.Errorf("invalid weekly item tuple")
		}
		for i := 0; i < len(values); i += 7 {
			if values[i] <= 0 || values[i+1] < 0 {
				return fmt.Errorf("invalid weekly template/weight")
			}
			item := BakalBiddingItem{Template: uint32(values[i]), Weight: values[i+1]}
			copy(item.Values[:], values[i+2:i+7])
			out.WeeklyGroups[id] = append(out.WeeklyGroups[id], item)
		}
	}
	for _, rate := range out.WeeklyRates {
		if len(out.WeeklyGroups[rate.Grade]) == 0 {
			return fmt.Errorf("weekly group not bound")
		}
	}
	return nil
}
