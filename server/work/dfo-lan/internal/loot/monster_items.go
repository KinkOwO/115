package loot

import (
	"fmt"
	"math"
	"strconv"
	"strings"

	"dfolan/internal/catalog"
)

const MonsterItemDropDenominator uint32 = 10000

func monsterItemsEnabled(r Rules) bool {
	for _, kind := range r.SupportedKinds {
		if kind == "stackable" {
			return true
		}
	}
	return false
}

// User-selected S4 compatibility policy (2026-10-02), pending official 115
// trigger evidence. Source pair values select items; they are not this rate.
func ParseMonsterItemDropPercent(value string) (uint32, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return 1000, nil
	}
	percent, err := strconv.ParseUint(value, 10, 32)
	if err != nil || percent > 100 {
		return 0, fmt.Errorf("DFO_ORDINARY_MONSTER_ITEM_DROP_PERCENT must be an integer in 0..100")
	}
	return uint32(percent) * 100, nil
}

func rollMonsterItems(c catalog.LootCatalog, table catalog.MonsterItemTable, seed uint32) (Outcome, error) {
	out := Outcome{NextSeed: seed}
	if c.OrdinaryMonsterItemRate > MonsterItemDropDenominator {
		return out, fmt.Errorf("invalid ordinary MOB item rate")
	}
	// mod 倍率在这里现读（不是构造目录时固化）：没有 mod 设置时与目录值逐位相同。
	rate := effectiveMonsterItemRate(c)
	if rate == 0 || !table.Declared {
		return out, nil
	}
	var candidates []catalog.MonsterItemPair
	var total uint64
	for _, p := range table.Items {
		if p.Template <= 0 || p.Value <= 0 || c.MonsterItemExclusions[uint32(p.Template)] {
			continue
		}
		// Creation-rate zero is allowed only by this explicit MOB declaration.
		if item, known := c.Items[uint32(p.Template)]; !known || item.Kind != "stackable" || item.StackableType == "[quest]" {
			out.SkippedKinds = append(out.SkippedKinds, fmt.Sprintf("monster_item_unavailable:%d", p.Template))
			continue
		}
		if c.HasRuntimeDetails() {
			if _, err := c.ItemScript(uint32(p.Template)); err != nil {
				out.SkippedKinds = append(out.SkippedKinds, fmt.Sprintf("monster_item_unreadable:%d:%v", p.Template, err))
				continue
			}
		}
		candidates = append(candidates, p)
		total += uint64(p.Value)
		if total > math.MaxUint32 {
			return out, fmt.Errorf("MOB item weight overflow")
		}
	}
	if total == 0 {
		return out, nil
	}
	rng := RNG{seed}
	if rng.Next(MonsterItemDropDenominator) < rate {
		ticket := uint64(rng.Next(uint32(total)))
		for _, p := range candidates {
			if ticket < uint64(p.Value) {
				out.Awards = []Award{{uint32(p.Template), 1}}
				break
			}
			ticket -= uint64(p.Value)
		}
	}
	out.NextSeed = rng.Seed
	return out, nil
}
