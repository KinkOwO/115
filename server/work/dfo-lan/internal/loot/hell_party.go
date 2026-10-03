package loot

import (
	"dfolan/internal/catalog"
	"dfolan/internal/dungeon"
	"dfolan/internal/inventory"
	"fmt"
	"strconv"
	"strings"
)

// Independent numerical policy; 100 = 1x. User approved S4 compatibility
// on 2026-10-03, with current PVF probability/rarity/equipment source values.
func ParseHellPartyDropPercent(value string) (uint32, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return 100, nil
	}
	p, err := strconv.ParseUint(value, 10, 32)
	if err != nil || p > 10000 {
		return 0, fmt.Errorf("DFO_HELL_PARTY_DROP_PERCENT must be an integer in 0..10000 (100 = 1x)")
	}
	return uint32(p), nil
}

func rollHellParty(c catalog.LootCatalog, pool []inventory.EquipmentDrop, seed uint32, d *dungeon.Session, actor dungeon.HellPartyRunActor) (Outcome, error) {
	out := Outcome{NextSeed: seed}
	if actor.RewardRolls == 0 || c.HellPartyDropPercent == 0 {
		return out, nil
	}
	t := c.HellPartyDrop
	if t == nil || d == nil || d.HellParty == nil || len(t.Rarity) != 2 || c.HellPartyDropPercent > 10000 || actor.RewardRolls > 255 {
		return out, fmt.Errorf("Hell Party drop source unavailable")
	}
	index := 0
	if d.Difficulty > 0 {
		index = int(d.Difficulty) - 1
	}
	if index > 4 {
		return out, fmt.Errorf("Hell Party dungeon difficulty outside source")
	}
	var rate uint32
	found := false
	for _, row := range t.Probability {
		if d.Definition.BasisLevel >= row[0] && d.Definition.BasisLevel <= row[1] {
			rate = row[index+2]
			found = true
			break
		}
	}
	if !found {
		return out, fmt.Errorf("%w: Hell basis level %d", ErrOutOfDropRange, d.Definition.BasisLevel)
	}
	if rate == 0 {
		return out, nil
	}
	chance := uint64(rate) * uint64(c.HellPartyDropPercent) / 100
	rarityIndex := 0
	if d.HellParty.Mode == 2 {
		rarityIndex = 1
	} else if d.HellParty.Mode != 1 {
		return out, fmt.Errorf("unsupported Hell reward mode")
	}
	// S4's equipment atlas compatibility window, not a guessed 115 formula.
	minimum, basis := d.Definition.MinimumLevel, d.Definition.BasisLevel
	if minimum == 0 {
		minimum = basis
	}
	if basis == 0 {
		basis = minimum
	}
	if minimum > basis {
		minimum, basis = basis, minimum
	}
	maximum := basis + 7
	if maximum > 200 {
		maximum = 200
	}
	collect := func(rarity int32) []inventory.EquipmentDrop {
		var rows []inventory.EquipmentDrop
		for _, row := range pool {
			if row.Rarity == rarity && row.Grade >= int32(minimum) && row.Grade <= int32(maximum) && row.Weight > 0 {
				rows = append(rows, row)
			}
		}
		return rows
	}
	rng := RNG{seed}
	for i := uint32(0); i < actor.RewardRolls; i++ {
		if uint64(rng.Next(1001)) > chance {
			continue
		}
		ticket := rng.Next(1000000) + 1
		rarity := int32(-1)
		for j, threshold := range t.Rarity[rarityIndex] {
			if ticket <= threshold {
				rarity = int32(j)
				break
			}
		}
		if rarity < 0 {
			return out, fmt.Errorf("Hell rarity uncovered")
		}
		rows := collect(rarity)
		if len(rows) == 0 && rarity > 0 {
			rows = collect(0)
		} // S4's rarity-0 fallback only
		if len(rows) == 0 {
			out.SkippedKinds = append(out.SkippedKinds, fmt.Sprintf("hell_equipment_pool_empty:rarity=%d,grade=%d..%d", rarity, minimum, maximum))
			continue
		}
		item, err := weightedEquipment(&rng, rows)
		if err != nil {
			return out, err
		}
		out.Awards = append(out.Awards, Award{Template: item.ID, Amount: 1})
	}
	out.NextSeed = rng.Seed
	return out, nil
}
