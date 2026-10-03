package loot

import (
	"dfolan/internal/catalog"
	"fmt"
	"math"
	"strconv"
	"strings"
)

const WorldDropDenominator uint32 = 100000

// User-selected S4 compatibility policy (2026-10-02). 100 means one times
// the source weights /100000; this is not a claim about the official 115 rate.
func ParseWorldDropPercent(value string) (uint32, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return 100, nil
	}
	p, err := strconv.ParseUint(value, 10, 32)
	if err != nil || p > 10000 {
		return 0, fmt.Errorf("DFO_ORDINARY_WORLD_DROP_PERCENT must be an integer in 0..10000 (100 = 1x)")
	}
	return uint32(p), nil
}

func rollWorldItems(c catalog.LootCatalog, seed uint32, level byte) (Outcome, error) {
	out := Outcome{NextSeed: seed}
	if c.OrdinaryWorldDropPercent > 10000 {
		return out, fmt.Errorf("invalid ordinary world drop multiplier")
	}
	if c.WorldDrop == nil || c.OrdinaryWorldDropPercent == 0 || level == 0 || level > 199 {
		return out, nil
	}
	row, ok := c.WorldDrop.Levels[uint32(level)]
	if !ok {
		return out, nil
	}
	if row.Column2 != 0 {
		return out, fmt.Errorf("world drop level %d has unsupported second column %d", level, row.Column2)
	}
	var total uint64
	for _, p := range row.Items {
		if p.Template > 0 && p.Value > 0 {
			total += uint64(p.Value)
		}
	}
	if total > math.MaxUint32 {
		return out, fmt.Errorf("world drop weight overflow")
	}
	if total == 0 {
		return out, nil
	}
	threshold := total * uint64(c.OrdinaryWorldDropPercent) / 100
	if threshold == 0 {
		return out, nil
	}
	if threshold > uint64(WorldDropDenominator) {
		threshold = uint64(WorldDropDenominator)
	}
	rng := RNG{seed}
	if uint64(rng.Next(WorldDropDenominator)) < threshold {
		ticket := uint64(rng.Next(uint32(total)))
		for _, p := range row.Items {
			if p.Template <= 0 || p.Value <= 0 {
				continue
			}
			if ticket >= uint64(p.Value) {
				ticket -= uint64(p.Value)
				continue
			}
			id := uint32(p.Template)
			// Do not reweight/reroll when a selected source item cannot be
			// granted. Preserve the other entries' declared probabilities.
			it, known := c.Items[id]
			if !known || it.Kind != "stackable" || it.StackableType == "[quest]" || c.MonsterItemExclusions[id] {
				out.SkippedKinds = append(out.SkippedKinds, fmt.Sprintf("world_drop_unavailable:%d", id))
				break
			}
			if c.HasRuntimeDetails() {
				if _, err := c.ItemScript(id); err != nil {
					out.SkippedKinds = append(out.SkippedKinds, fmt.Sprintf("world_drop_unreadable:%d:%v", id, err))
					break
				}
			}
			out.Awards = []Award{{Template: id, Amount: 1}}
			break
		}
	}
	out.NextSeed = rng.Seed
	return out, nil
}
