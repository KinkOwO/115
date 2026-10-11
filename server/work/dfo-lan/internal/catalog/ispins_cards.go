package catalog

import (
	"fmt"
	"math"
	"sort"
)

// Native N2255 has three records of {index i16, parameter i16, grade u32}.
// The source's select pools correspond to the captured grades pool+1.
type IspinsCard struct {
	Index, Value uint16
	Grade        uint32
}

// Draw receives a positive exclusive bound. The gateway supplies secure
// entropy; tests can change the source and supply a deterministic draw.
func ChooseIspinsCards(defs map[uint16]IspinsOperationDefinition, party, difficulty string, draw func(int64) (int64, error)) ([3]IspinsCard, error) {
	var cards [3]IspinsCard
	for pool := range cards {
		var indices []int
		var total int64
		for index, def := range defs {
			if def.SelectPool == uint32(pool) && def.Difficulty == difficulty && def.MemberLimit == party && def.Probability > 0 {
				indices = append(indices, int(index))
				total += int64(def.Probability)
			}
		}
		if total == 0 || draw == nil {
			return cards, fmt.Errorf("Ispins source pool%d empty", pool)
		}
		sort.Ints(indices)
		n, err := draw(total)
		if err != nil {
			return cards, err
		}
		if n < 0 || n >= total {
			return cards, fmt.Errorf("Ispins draw outside source pool")
		}
		for _, index := range indices {
			def := defs[uint16(index)]
			if n >= int64(def.Probability) {
				n -= int64(def.Probability)
				continue
			}
			value, err := def.DrawParameter(party, difficulty, draw)
			if err != nil {
				return cards, fmt.Errorf("operation%d: %w", index, err)
			}
			if index > math.MaxInt16 {
				return cards, fmt.Errorf("Ispins index exceeds native signed short")
			}
			cards[pool] = IspinsCard{Index: uint16(index), Value: value, Grade: uint32(pool + 1)}
			break
		}
	}
	return cards, nil
}

func (def IspinsOperationDefinition) DrawParameter(party, difficulty string, draw func(int64) (int64, error)) (uint16, error) {
	// The native time operation takes minutes from [type fixed value].
	if def.Type == 6 {
		if def.FixedValue == 0 || def.FixedValue > math.MaxInt16 {
			return 0, fmt.Errorf("invalid source time parameter")
		}
		return uint16(def.FixedValue), nil
	}
	for _, r := range def.RandomRanges {
		if r.Party != party || r.Difficulty != difficulty {
			continue
		}
		if r.Minimum < 0 || r.Maximum > math.MaxInt16 || r.Maximum < r.Minimum || (r.Minimum != r.Maximum && r.Step <= 0) {
			return 0, fmt.Errorf("invalid native card range")
		}
		value := int64(r.Minimum)
		if r.Minimum != r.Maximum {
			count := (int64(r.Maximum)-value)/int64(r.Step) + 1
			n, err := draw(count)
			if err != nil {
				return 0, err
			}
			if n < 0 || n >= count {
				return 0, fmt.Errorf("Ispins draw outside source range")
			}
			value += n * int64(r.Step)
		}
		return uint16(value), nil
	}
	return 0, fmt.Errorf("Ispins source parameter dimension missing")
}
