package main

import (
	"dfolan/internal/catalog"
	"testing"

	"dfolan/internal/loot"
)

// The endkeeper's own table pays its bonus through two smart drop carriers
// (10362480 / 10362481, named by the fixed box 10416107's third pool). They are
// typed [virtual] and still declare [booster info] with [instantly open], so the
// old type-string container test called them ordinary items: they reached the
// ground and nobody could open them.
//
// The catalog is now built from the [booster info] section itself, and each
// carrier's reserved placeholder row (490000001) is replaced by the group its own
// [smart drop group id] names - a reading with 226/226 support in the shipped
// data (every item whose body pays a reserved id declares a smart group, and no
// item pays a reserved id without one).
func TestSmartDropCarriersOpenThroughTheirGroup(t *testing.T) {
	cat, err := catalog.LoadBoosterCatalog("../../configs/booster-catalog.json", "../../configs/items.index.json")
	if err != nil {
		t.Fatal(err)
	}
	src := boosterBoxSource{catalog: cat}

	for _, tc := range []struct {
		carrier uint32
		group   uint32
		rows    int
		first   uint32
		weight  uint32
	}{
		{carrier: 10362480, group: 21250, rows: 42, first: 101001149, weight: 30},
		{carrier: 10362481, group: 21251, rows: 11, first: 100051304, weight: 10},
	} {
		box, ok := src.RewardBox(tc.carrier)
		if !ok {
			t.Fatalf("carrier %d is not openable: its [booster info] section was not read", tc.carrier)
		}
		if len(box.Pools) != 1 {
			t.Fatalf("carrier %d pools: got %d want 1", tc.carrier, len(box.Pools))
		}
		p := box.Pools[0]
		if p.Draws != 1 || len(p.Candidates) != tc.rows {
			t.Fatalf("carrier %d: got %d draws / %d candidates, want 1 / %d",
				tc.carrier, p.Draws, len(p.Candidates), tc.rows)
		}
		if p.Candidates[0].Template != tc.first || p.Candidates[0].Weight != tc.weight {
			t.Fatalf("carrier %d first row: got %+v want template %d weight %d",
				tc.carrier, p.Candidates[0], tc.first, tc.weight)
		}
		for _, c := range p.Candidates {
			// Paying the placeholder would award nothing at all, which is the
			// quiet version of the defect this test exists for.
			if c.Template == 490000001 || c.Template == 0 {
				t.Fatalf("carrier %d still pays the reserved placeholder", tc.carrier)
			}
		}
		if !src.Container(tc.carrier) {
			t.Fatalf("carrier %d is not recognised as a container", tc.carrier)
		}
	}

	// A wrapper that pays a carrier must never hand the carrier over: it is a box
	// too, so the walk has to open it and pay what is inside.
	hits := map[uint32]int{}
	for seed := uint32(0); seed < 400; seed++ {
		paid, _, _ := loot.OpenRewardBoxes(seed, src, []loot.Award{{Template: 10416107, Amount: 1}})
		for _, a := range paid {
			hits[a.Template]++
			if a.Template == 10362480 || a.Template == 10362481 {
				t.Fatalf("seed %d: the clear handed over carrier %d, which cannot be opened", seed, a.Template)
			}
		}
	}
	if len(hits) == 0 {
		t.Fatal("400 opens of the endkeeper box paid nothing at all")
	}
	t.Logf("400 opens paid %d distinct template(s)", len(hits))
}
