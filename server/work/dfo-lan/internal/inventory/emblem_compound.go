package inventory

import (
	"dfolan/internal/catalog"
	"dfolan/internal/game/protocol"
	"fmt"
)

func (b Bag) CompoundEmblems(c catalog.LootCatalog, bagRules BagRules, rules *EmblemCompoundRules, r protocol.CompoundEmblemRequest, draw func(uint32) (uint32, error)) (Bag, []protocol.CompoundEmblemReward, error) {
	fail := func(e error) (Bag, []protocol.CompoundEmblemReward, error) { return b, nil, e }
	if rules == nil || draw == nil || rules.Source != c.Source.Checksum || bagRules.Source != c.Source.Checksum {
		return fail(fmt.Errorf("emblem compound source unavailable or mismatched"))
	}
	// Directed selection modes need their own source groups and native contract.
	if r.Mode != 0 {
		return fail(fmt.Errorf("directed emblem compound mode unsupported"))
	}
	if len(r.Inputs) < 2 || len(r.Inputs) > 5 {
		return fail(fmt.Errorf("invalid emblem compound count"))
	}
	seen := map[uint16]bool{}
	var grade int32
	for _, in := range r.Inputs {
		if seen[in.Slot] {
			return fail(fmt.Errorf("duplicate emblem compound slot"))
		}
		seen[in.Slot] = true
		g, ok := rules.Grades[in.Template]
		item, present := c.Items[in.Template]
		if !ok || !present || item.Kind != "stackable" || item.StackableType != "[avatar emblem]" {
			return fail(fmt.Errorf("unsupported source emblem compound input"))
		}
		if grade != 0 && grade != g {
			return fail(fmt.Errorf("mixed emblem compound grades"))
		}
		grade = g
		found := false
		for _, row := range b.Items {
			if row.Slot == in.Slot {
				if row.Template != in.Template || row.Amount == 0 || row.ExpireTime != 0 {
					return fail(fmt.Errorf("stale or limited-period emblem compound input"))
				}
				found = true
				break
			}
		}
		if !found {
			return fail(fmt.Errorf("emblem compound input missing"))
		}
	}
	prob, ok := rules.Rolls[EmblemCompoundKey{grade, len(r.Inputs)}]
	if !ok {
		return fail(fmt.Errorf("unsupported source emblem compound combination"))
	}
	n, e := draw(100)
	if e != nil {
		return fail(e)
	}
	if n >= 100 {
		return fail(fmt.Errorf("invalid emblem compound random draw"))
	}
	col := 0
	for col < 4 && n >= prob[col] {
		n -= prob[col]
		col++
	}
	if col == 4 {
		return fail(fmt.Errorf("invalid emblem compound probability row"))
	}
	pool := rules.Pools[int32(col+1)]
	if len(pool) == 0 {
		return fail(fmt.Errorf("empty source emblem compound pool"))
	}
	n, e = draw(uint32(len(pool)))
	if e != nil {
		return fail(e)
	}
	if n >= uint32(len(pool)) {
		return fail(fmt.Errorf("invalid emblem compound pool draw"))
	}
	id := pool[n]
	if rules.Grades[id] != int32(col+1) {
		return fail(fmt.Errorf("invalid source emblem compound reward"))
	}
	next := b
	next.Items = make([]BagItem, 0, len(b.Items))
	for _, row := range b.Items {
		if seen[row.Slot] {
			row.Amount--
		}
		if row.Amount > 0 {
			next.Items = append(next.Items, row)
		}
	}
	next, _, e = next.Add(c, bagRules, id, 1)
	if e != nil {
		return fail(e)
	}
	return next, []protocol.CompoundEmblemReward{{Template: id, Count: 1}}, nil
}
