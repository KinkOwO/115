package loot

import "testing"

func TestBorderFivefoldDrawsAndScope(t *testing.T) {
	a, err := LoadAttunementRewards(attunementConfig)
	if err != nil {
		t.Fatal(err)
	}
	if err = a.SetBorderMultipliers(5, 5); err != nil {
		t.Fatal(err)
	}
	for _, id := range []uint32{100005066, 100005067, 100005068} {
		tab := a.byDungeon[id]
		for seed := uint32(1); seed < 200; seed++ {
			rng := RNG{seed}
			for i := 0; i < 5; i++ {
				if _, err := pickAttunementBoosted(&rng, tab.Fixed[0].Entries, 5); err != nil {
					t.Fatal(err)
				}
			}
			branch, err := pickAttunementBranchBoosted(&rng, tab.Additional, 5)
			if err != nil {
				t.Fatal(err)
			}
			got, _, err := a.Roll(seed, id, 0)
			if err != nil {
				t.Fatal(err)
			}
			if len(got) != 5+int(branch.DropCount)*5 {
				t.Fatalf("%d seed %d: %d rewards", id, seed, len(got))
			}
		}
	}
	base, err := LoadAttunementRewards(attunementConfig)
	if err != nil {
		t.Fatal(err)
	}
	for seed := uint32(1); seed < 100; seed++ {
		x, xs, err := a.Roll(seed, 100005014, 0)
		if err != nil {
			t.Fatal(err)
		}
		y, ys, err := base.Roll(seed, 100005014, 0)
		if err != nil {
			t.Fatal(err)
		}
		if xs != ys || len(x) != len(y) {
			t.Fatal("endkeeper changed")
		}
		for i := range x {
			if x[i] != y[i] {
				t.Fatal("endkeeper award changed")
			}
		}
	}
	for _, tab := range a.Tables {
		for _, fixed := range tab.Fixed {
			if err := checkAttunementEntries(fixed.Entries, "unchanged source"); err != nil {
				t.Fatal(err)
			}
		}
	}
}

func TestBorderEpicPrimevalWeightedDistribution(t *testing.T) {
	entries := []attunementEntry{{Tier: "rare", Weight: 900000, Item: 1}, {Tier: "epic", Weight: 90000, Item: 2}, {Tier: "primeval", Weight: 10000, Item: 3}}
	rng := RNG{123456}
	counts := map[uint32]int{}
	for i := 0; i < 100000; i++ {
		e, err := pickAttunementBoosted(&rng, entries, 5)
		if err != nil {
			t.Fatal(err)
		}
		counts[e.Item]++
	}
	// 900k : 450k : 50k => 64.286%, 32.143%, 3.571%.
	for id, want := range map[uint32]int{1: 64286, 2: 32143, 3: 3571} {
		got := counts[id]
		if got < want-700 || got > want+700 {
			t.Fatalf("item %d count %d want near %d", id, got, want)
		}
	}
	a, err := LoadAttunementRewards(attunementConfig)
	if err != nil {
		t.Fatal(err)
	}
	if a.SetBorderMultipliers(0, 5) == nil || a.SetBorderMultipliers(5, 101) == nil {
		t.Fatal("invalid policy accepted")
	}
}

func TestBorderAdditionalEquipmentBranchesGainWeight(t *testing.T) {
	branches := []attunementAdditional{
		{EffectIndex: 1, SelectProb: 900000, DropCount: 1, Entries: []attunementEntry{{Tier: "material", Weight: 1000000, Item: 1}}},
		{EffectIndex: 2, SelectProb: 100000, DropCount: 1, Entries: []attunementEntry{{Tier: "epic", Weight: 1000000, Item: 2}}},
	}
	rng := RNG{123}
	hits := 0
	for i := 0; i < 100000; i++ {
		b, err := pickAttunementBranchBoosted(&rng, branches, 5)
		if err != nil {
			t.Fatal(err)
		}
		if b.EffectIndex == 2 {
			hits++
		}
	}
	if hits < 35000 || hits > 36500 {
		t.Fatalf("equipment branch hits %d expected 35.714%%", hits)
	}
}
