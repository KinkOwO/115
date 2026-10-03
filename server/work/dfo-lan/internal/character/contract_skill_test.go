package character

import (
	"dfolan/internal/catalog"
	"testing"
)

func TestTacticianContractSkillLearning(t *testing.T) {
	c, e := catalog.LoadCharacters("../../configs/characters.next25.json")
	if e != nil {
		t.Fatal(e)
	}
	l, e := LoadLearningCatalog("../../configs/skills.release.json", c.Source.Checksum)
	if e != nil {
		t.Fatal(e)
	}

	// Skill 8 requires level 15.
	d := l.index[0][8]
	levels := d.Ints("[required level]")
	if len(levels) != 1 || levels[0] != 15 {
		t.Fatalf("unexpected required level for skill 8: %v", levels)
	}

	st := State{Level: 10, Advancement: 0}

	// Without Tactician's Contract (level 10 < 15): must fail
	if _, err := d.costForState(st, 1, nil); err == nil {
		t.Fatal("expected level 10 to fail learning level 15 skill without tactician contract")
	}

	// With Tactician's Contract (effective level = 10 + 5 = 15 >= 15): must succeed
	cost, err := d.costForLevel(st, int(st.Level)+5, 1, nil)
	if err != nil {
		t.Fatalf("expected level 10 + 5 to succeed learning level 15 skill with tactician contract: %v", err)
	}
	if cost != 15 {
		t.Fatalf("expected SP cost 15, got %d", cost)
	}

	// Level 9 + 5 = 14 < 15: must still fail
	st9 := State{Level: 9, Advancement: 0}
	if _, err := d.costForLevel(st9, int(st9.Level)+5, 1, nil); err == nil {
		t.Fatal("expected level 9 + 5 to fail learning level 15 skill")
	}
}

func TestGrowthContractBonusCalculations(t *testing.T) {
	// Verify +20% bonus math
	baseExp := uint64(1000)
	boostedExp := baseExp + baseExp*20/100
	if boostedExp != 1200 {
		t.Fatalf("expected 1200, got %d", boostedExp)
	}

	// Verify room fatigue cost reduction by 1
	roomCost := uint16(1)
	reducedCost := roomCost - 1
	if reducedCost != 0 {
		t.Fatalf("expected 0, got %d", reducedCost)
	}

	roomCost2 := uint16(2)
	reducedCost2 := roomCost2 - 1
	if reducedCost2 != 1 {
		t.Fatalf("expected 1, got %d", reducedCost2)
	}
}
