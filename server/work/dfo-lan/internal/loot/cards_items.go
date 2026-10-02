package loot

import (
	"fmt"
	"math"

	"dfolan/internal/dungeon"
	"dfolan/internal/inventory"
)

const ordinaryFreeCardModel = "pvf-ordinary-free-item-compat-v1"

// The appearance formula is the named reference compatibility formula, using
// the current four-column PVF profile, signed grade offsets and source weights.
// It uses the already verified ordinary NOTI35 channel. Other item types remain
// empty rather than being misfiled into its main-equipment list.
func (s *Service) ordinaryFreeCard(d *dungeon.Session, seed uint32, level byte) (Award, error) {
	if s.Equipment == nil {
		return Award{}, fmt.Errorf("ordinary card equipment source unavailable")
	}
	t := s.Catalog.ClearReward
	if t == nil {
		return Award{}, fmt.Errorf("ordinary clear-reward table/difficulty unavailable")
	}
	difficulty, err := ordinaryCardDifficultyIndex(d.Difficulty)
	if err != nil {
		return Award{}, err
	}
	profile, ok := t.Profile("default")
	row, covered := profile.Row(int32(level))
	if !ok || !covered || row.Auxiliary != 0 {
		return Award{}, fmt.Errorf("ordinary default card profile unsupported at level %d", level)
	}
	total, visited := len(d.Maze.Rooms), len(d.Visited)
	if total == 0 || visited == 0 {
		return Award{}, fmt.Errorf("ordinary card exploration unavailable")
	}
	if visited > total {
		visited = total
	}
	mapRate := float64(visited) / float64(total)
	for _, pair := range t.MapCountRates {
		if pair[0] == int32(total) {
			mapRate *= float64(pair[1]) / 10000
			break
		}
	}
	threshold := math.Min(10000, math.Max(0, float64(row.Probability)*mapRate+float64(t.DifficultyBonus[difficulty])))
	rng := RNG{seed}
	// Gold variance consumes the first draw in CardGold. Continue that stream
	// for the item instead of restarting it at the correlated first value.
	for i := 0; i+2 < len(s.Tables.Gold); i += 3 {
		if s.Tables.Gold[i] == float64(level) && s.Tables.Gold[i+2] > 0 {
			rng.Next(uint32(s.Tables.Gold[i+2])*2 + 1)
			break
		}
	}
	if float64(rng.Next(10000)) >= threshold {
		return Award{}, nil
	}
	var weight uint64
	for _, n := range t.ItemTypeProbability {
		if n < 0 {
			return Award{}, fmt.Errorf("negative card item-type weight")
		}
		weight += uint64(n)
	}
	if weight == 0 || weight > math.MaxUint32 {
		return Award{}, fmt.Errorf("invalid card item-type weights")
	}
	pick, itemType := rng.Next(uint32(weight)), 0
	for i, n := range t.ItemTypeProbability {
		if pick < uint32(n) {
			itemType = i + 1
			break
		}
		pick -= uint32(n)
	}
	roll, rarity := int32(rng.Next(1000000)+1), int32(-1)
	for i, n := range t.Rarity {
		if roll <= n {
			rarity = int32(i)
			break
		}
	}
	if itemType != 2 || rarity < 0 {
		return Award{}, nil
	}
	var grade []float64
	for _, rangeRow := range t.GradeReference {
		if rangeRow[0] == int32(level) {
			grade = []float64{float64(level), float64(rangeRow[1]), float64(rangeRow[2])}
			break
		}
	}
	if grade == nil {
		return Award{}, fmt.Errorf("ordinary card grade row absent at %d", level)
	}
	candidates := equipmentCandidates(s.Equipment.OrdinaryPool, rarity, level, grade)
	if len(candidates) == 0 {
		return Award{}, nil
	}
	item, err := weightedEquipment(&rng, candidates)
	if err != nil {
		return Award{}, err
	}
	if _, err = s.Equipment.Reward(item.ID); err != nil {
		return Award{}, fmt.Errorf("ordinary card candidate %d: %w", item.ID, err)
	}
	return Award{Template: item.ID, Amount: 1}, nil
}

func weightedEquipment(rng *RNG, candidates []inventory.EquipmentDrop) (inventory.EquipmentDrop, error) {
	var total uint64
	for _, item := range candidates {
		total += uint64(item.Weight)
	}
	if total == 0 || total > math.MaxUint32 {
		return inventory.EquipmentDrop{}, fmt.Errorf("invalid ordinary equipment weights")
	}
	pick := rng.Next(uint32(total))
	for _, item := range candidates {
		if pick < item.Weight {
			return item, nil
		}
		pick -= item.Weight
	}
	return inventory.EquipmentDrop{}, fmt.Errorf("ordinary equipment selection exhausted")
}
