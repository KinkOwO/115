package loot

import (
	"testing"
)

func TestCardGold(t *testing.T) {
	tables := Tables{
		Gold: []float64{18, 250, 20},
	}
	rules := CardRules{
		GoldNumerator:   175,
		GoldDenominator: 1000,
		Difficulty:      []float64{1.0, 1.2, 1.5},
	}
	gold, err := CardGold(tables, rules, 12345, 18, 0)
	if err != nil {
		t.Fatalf("CardGold failed: %v", err)
	}
	if gold == 0 {
		t.Fatalf("expected non-zero gold, got 0")
	}
}
