package loot

import (
	"dfolan/internal/catalog"
	"dfolan/internal/inventory"
	"testing"
)

func TestOrdinarySourceCategoryDifficultyAndEmptyPoolBudget(t *testing.T) {
	c := catalog.LootCatalog{MaximumGrade: 1}
	tables := Tables{Probability: []float64{1, 200, 10000, 0, 1000, 0, 0}, Gold: []float64{10, 100, 0}, Grade: []float64{10, 0, 1}, Rank: make([]float64, 20), Rarity: make([]float64, 36), Difficulty: make([]float64, 25)}
	for i := range tables.Rank {
		tables.Rank[i] = 1
	}
	for i := range tables.Rarity {
		tables.Rarity[i] = 1000000
	}
	for i := range tables.Difficulty {
		tables.Difficulty[i] = 1
	}
	tables.Difficulty[4], tables.Difficulty[2*5+4] = .5, 2
	rules := Rules{Denominator: 10000, DifficultyBonus: []float64{99, 99, 99, 99, 99}, SupportedKinds: []string{"gold", "equipment"}}
	pool := []inventory.EquipmentDrop{{ID: 1, Grade: 10, Rarity: 0, Weight: 1}, {ID: 2, Grade: 10, Rarity: 0, Weight: 9}}
	gear, second, emptyHits, goldHits := 0, 0, 0, 0
	const samples = 10000
	for i := uint32(1); i <= samples; i++ {
		out, err := RollOrdinary(c, tables, rules, pool, i*2654435761, 10, 0, 4, 0)
		if err != nil {
			t.Fatal(err)
		}
		for _, award := range out.Awards {
			if award.Template == 0 {
				goldHits++
				if award.Amount != 50 {
					t.Fatalf("gold used item/JSON difficulty: %+v", out)
				}
			} else {
				gear++
				if award.Template == 2 {
					second++
				}
			}
		}
		empty, err := RollOrdinary(c, tables, rules, nil, i*2654435761, 10, 0, 4, 0)
		if err != nil || len(empty.Awards) > 1 || len(empty.Awards) == 1 && empty.Awards[0].Template != 0 {
			t.Fatalf("empty generic pool manufactured gear: %+v %v", empty, err)
		}
		emptyHits += empty.ItemBudget
	}
	if gear < 1800 || gear > 2200 || second < gear*85/100 || second > gear*95/100 || emptyHits != gear || goldHits < 4700 || goldHits > 5300 {
		t.Fatalf("rates/weights/budget mismatch: gear=%d second=%d empty=%d gold=%d", gear, second, emptyHits, goldHits)
	}
}
