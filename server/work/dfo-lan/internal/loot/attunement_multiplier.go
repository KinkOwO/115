package loot

import "fmt"

// SetBorderMultipliers applies server-owner numerical policy over validated PVF
// content. Only the three Border dungeons are affected, not the endkeeper,
// omen coupons, oath rewards, or hidden tables.
func (a *AttunementRewards) SetBorderMultipliers(quantity, rarity int) error {
	if quantity < 1 || quantity > 100 || rarity < 1 || rarity > 100 {
		return fmt.Errorf("attunement multipliers must be in 1..100")
	}
	for _, t := range a.Tables {
		if t.Dungeon < 100005066 || t.Dungeon > 100005068 {
			continue
		}
		for _, b := range t.Additional {
			if uint64(b.DropCount)*uint64(quantity) > 10000 {
				return fmt.Errorf("attunement boosted draw count exceeds 10000")
			}
		}
	}
	a.quantityMultiplier, a.rarityWeightMultiplier = uint32(quantity), uint32(rarity)
	return nil
}

func (a *AttunementRewards) rewardMultipliers(dungeon uint32) (uint32, uint32) {
	if dungeon < 100005066 || dungeon > 100005068 {
		return 1, 1
	}
	q, r := a.quantityMultiplier, a.rarityWeightMultiplier
	if q == 0 {
		q = 1
	}
	if r == 0 {
		r = 1
	}
	return q, r
}

// Weight scaling uses the increased total as denominator: multiplying two
// mutually exclusive probabilities without normalization could exceed 100%.
// Source weights stay untouched, including their million-space invariant.
func pickAttunementBoosted(rng *RNG, list []attunementEntry, multiplier uint32) (attunementEntry, error) {
	if multiplier == 1 {
		return pickAttunement(rng, list)
	}
	weight := func(e attunementEntry) uint64 {
		w := uint64(e.Weight)
		if e.Tier == "epic" || e.Tier == "primeval" {
			w *= uint64(multiplier)
		}
		return w
	}
	var total uint64
	for _, e := range list {
		total += weight(e)
	}
	if total == 0 || total > 4294967295 {
		return attunementEntry{}, fmt.Errorf("invalid boosted attunement weight total")
	}
	roll := uint64(rng.Next(uint32(total)))
	for _, e := range list {
		w := weight(e)
		if roll < w {
			return e, nil
		}
		roll -= w
	}
	return attunementEntry{}, fmt.Errorf("boosted attunement selection exhausted")
}

// Additional effects include equipment and material branches. Boost the branch
// by its epic/primeval share as well, so a pure epic branch gains 5x relative
// weight against a material branch. Integer resolution matches source weights.
func pickAttunementBranchBoosted(rng *RNG, list []attunementAdditional, multiplier uint32) (attunementAdditional, error) {
	if multiplier == 1 {
		return pickAttunementBranch(rng, list)
	}
	weights := make([]uint32, len(list))
	var total uint64
	for i, b := range list {
		var high uint64
		for _, e := range b.Entries {
			if e.Tier == "epic" || e.Tier == "primeval" {
				high += uint64(e.Weight)
			}
		}
		w := uint64(b.SelectProb) * (attunementWeightSpace + uint64(multiplier-1)*high) / attunementWeightSpace
		if w == 0 || w > 4294967295 {
			return attunementAdditional{}, fmt.Errorf("invalid boosted branch weight")
		}
		weights[i] = uint32(w)
		total += w
	}
	if total == 0 || total > 4294967295 {
		return attunementAdditional{}, fmt.Errorf("invalid boosted branch total")
	}
	roll := uint64(rng.Next(uint32(total)))
	for i, w := range weights {
		if roll < uint64(w) {
			return list[i], nil
		}
		roll -= uint64(w)
	}
	return attunementAdditional{}, fmt.Errorf("boosted branch selection exhausted")
}
