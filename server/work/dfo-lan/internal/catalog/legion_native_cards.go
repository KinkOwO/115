package catalog

import (
	"fmt"
	"math"
	"sort"
)

// NativeLegionCard freezes the source operation and its numeric operands.
// Wire conversion is separate: Dusky's floating parameters cannot silently
// become Ispins signed shorts or be rounded before native scale is established.
type NativeLegionCard struct {
	Index      uint16
	Pool       uint32
	Parameter  float64
	FixedValue *float64
	// FormulaBound means the native effect and tooltip consume operation
	// formula, not the integer magnitude in the network operation record.
	FormulaBound bool
}

// DrawCard consumes the mode's own source index list and weights. Pools are
// explicit because their presentation differs per content and per phase.
func (s *NativeLegionOperations) DrawCard(mode, party, rangeMode string, pool uint32, draw func(int64) (int64, error)) (NativeLegionCard, error) {
	return s.drawCard(mode, party, pool, draw, func(d NativeLegionOperation) (float64, bool, error) {
		v, err := d.DrawNativeParameter(party, rangeMode, draw)
		return v, false, err
	})
}

// DrawDuskyCard binds the current US type4/type7 vtables: effect +0x28 and
// tooltip +0x78 consume definition+0x110's formula vector. Their network+0x108
// value is unused. Keep the source formula intact; do not truncate floats or
// invent a sampler for the unused step0 range. Other types retain strict draws.
func (s *NativeLegionOperations) DrawDuskyCard(mode, party, rangeMode string, pool uint32, draw func(int64) (int64, error)) (NativeLegionCard, error) {
	return s.drawCard(mode, party, pool, draw, func(d NativeLegionOperation) (float64, bool, error) {
		if d.Type == 4 || d.Type == 7 {
			formula := d.Fields["[operation formula]"]
			if len(formula) == 0 || len(formula)%2 != 0 {
				return 0, false, fmt.Errorf("Dusky formula missing or incomplete")
			}
			for i := 0; i < len(formula); i += 2 {
				if formula[i].Type != 6 || formula[i].Text == "" {
					return 0, false, fmt.Errorf("Dusky formula variable missing")
				}
				if _, err := nativeOperationNumber(formula[i+1]); err != nil {
					return 0, false, err
				}
			}
			return 0, true, nil
		}
		v, err := d.DrawNativeParameter(party, rangeMode, draw)
		return v, false, err
	})
}

func (s *NativeLegionOperations) drawCard(mode, party string, pool uint32, draw func(int64) (int64, error), parameter func(NativeLegionOperation) (float64, bool, error)) (NativeLegionCard, error) {
	var card NativeLegionCard
	if s == nil || draw == nil {
		return card, fmt.Errorf("native operation source/entropy missing")
	}
	modeIndices, ok := s.Modes[mode]
	if !ok {
		return card, fmt.Errorf("native operation mode %q absent", mode)
	}
	indices := append([]uint16(nil), modeIndices...)
	sort.Slice(indices, func(i, j int) bool { return indices[i] < indices[j] })
	var total int64
	for _, index := range indices {
		d, ok := s.Definitions[index]
		if !ok {
			return card, fmt.Errorf("missing operation%d", index)
		}
		if d.SelectPool == pool && d.MemberLimit == party {
			total += int64(d.Probability)
		}
	}
	if total <= 0 {
		return card, fmt.Errorf("native operation pool%d empty for %s/%s", pool, mode, party)
	}
	n, err := draw(total)
	if err != nil {
		return card, err
	}
	if n < 0 || n >= total {
		return card, fmt.Errorf("operation draw outside source pool")
	}
	for _, index := range indices {
		d := s.Definitions[index]
		if d.SelectPool != pool || d.MemberLimit != party || d.Probability == 0 {
			continue
		}
		if n >= int64(d.Probability) {
			n -= int64(d.Probability)
			continue
		}
		value, formulaBound, err := parameter(d)
		if err != nil {
			return card, fmt.Errorf("operation%d: %w", index, err)
		}
		card = NativeLegionCard{Index: index, Pool: pool, Parameter: value, FormulaBound: formulaBound}
		if d.FixedValue != nil {
			v := *d.FixedValue
			card.FixedValue = &v
		}
		return card, nil
	}
	return card, fmt.Errorf("operation source lottery exhausted")
}

func (d NativeLegionOperation) DrawNativeParameter(party, difficulty string, draw func(int64) (int64, error)) (float64, error) {
	var found *NativeLegionRange
	for i := range d.Ranges {
		r := &d.Ranges[i]
		if r.Party == party && r.Difficulty == difficulty {
			if found != nil {
				return 0, fmt.Errorf("duplicate parameter dimension")
			}
			found = r
		}
	}
	if found == nil {
		return 0, fmt.Errorf("native parameter dimension missing")
	}
	r := *found
	if r.Minimum == r.Maximum {
		return r.Minimum, nil
	}
	// Step0 nonconstant ranges need their own native continuous sampler.
	// A failed conversion must not redirect probability to another operation.
	if r.Step <= 0 || draw == nil {
		return 0, fmt.Errorf("continuous native range has no bound sampler")
	}
	count := math.Floor((r.Maximum-r.Minimum)/r.Step) + 1
	if math.IsNaN(count) || math.IsInf(count, 0) || count < 1 || count >= math.MaxInt64 {
		return 0, fmt.Errorf("invalid native range cardinality")
	}
	n, err := draw(int64(count))
	if err != nil {
		return 0, err
	}
	if n < 0 || n >= int64(count) {
		return 0, fmt.Errorf("parameter draw outside native range")
	}
	return r.Minimum + float64(n)*r.Step, nil
}

// StagedLegionDungeons follows source COS order or an explicitly selected
// weekly source row. The native weekly epoch algorithm is a separate contract;
// no unverified date fallback changes the rotation here.
func StagedLegionDungeons(c LegionContent, weeklyRow int) ([]LegionDungeon, error) {
	if c.LastPhase != 2 || c.NativeOperations == nil {
		return nil, fmt.Errorf("native three-stage source missing")
	}
	indices := make([]uint32, int(c.LastPhase)+1)
	for i := range indices {
		indices[i] = uint32(i)
	}
	if len(c.NativeOperations.WeeklyDungeons) > 0 {
		if weeklyRow < 0 || weeklyRow >= len(c.NativeOperations.WeeklyDungeons) {
			return nil, fmt.Errorf("weekly source row unavailable")
		}
		indices = c.NativeOperations.WeeklyDungeons[weeklyRow]
	} else if weeklyRow != -1 {
		return nil, fmt.Errorf("non-weekly content has no weekly row")
	}
	if len(indices) != int(c.LastPhase)+1 {
		return nil, fmt.Errorf("source stage count mismatch")
	}
	seen := map[uint32]bool{}
	out := make([]LegionDungeon, 0, len(indices))
	for _, index := range indices {
		if int(index) >= len(c.Dungeons) || seen[index] {
			return nil, fmt.Errorf("source stage index missing/duplicate")
		}
		d := c.Dungeons[index]
		if d.Dungeon == 0 || d.Boss == 0 {
			return nil, fmt.Errorf("source stage dungeon/boss absent")
		}
		seen[index] = true
		d.Tail = append([]int64(nil), d.Tail...)
		out = append(out, d)
	}
	return out, nil
}
