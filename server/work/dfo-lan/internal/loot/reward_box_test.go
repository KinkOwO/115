package loot

import (
	"reflect"
	"strings"
	"testing"
)

// fakeBoxes is a hand-built source. The unit tests are about the unwrap rule,
// not about the shipped tables: a case built from the real catalog would only
// prove the catalog.
type fakeBoxes struct {
	boxes      map[uint32]RewardBox
	items      map[uint32]bool
	containers map[uint32]bool
}

func (f fakeBoxes) RewardBox(t uint32) (RewardBox, bool) { b, ok := f.boxes[t]; return b, ok }
func (f fakeBoxes) Item(t uint32) bool                   { return f.items[t] }
func (f fakeBoxes) Container(t uint32) bool              { return f.containers[t] }

// TestOpenRewardBoxesPaysTheContentsAndNotTheWrapper is the whole point of the
// step: the table pays wrappers, the player is owed what is inside them.
func TestOpenRewardBoxesPaysTheContentsAndNotTheWrapper(t *testing.T) {
	src := fakeBoxes{
		boxes: map[uint32]RewardBox{
			900: {Pools: []RewardBoxPool{
				{Draws: 1, Candidates: []RewardBoxCandidate{{Template: 11, Weight: 600000, Count: 70}}},
				{Draws: 1, Candidates: []RewardBoxCandidate{{Template: 12, Weight: 400000, Count: 1}}},
			}},
		},
		items:      map[uint32]bool{11: true, 12: true},
		containers: map[uint32]bool{900: true},
	}
	seen := map[uint32]int{}
	for seed := uint32(1); seed <= 400; seed++ {
		got, _, _ := OpenRewardBoxes(seed, src, []Award{{Template: 900, Amount: 1}})
		if len(got) != 2 {
			t.Fatalf("seed %d: %d rows, want one per pool", seed, len(got))
		}
		for _, a := range got {
			if a.Template == 900 {
				t.Fatalf("seed %d: the wrapper itself reached the ground", seed)
			}
			seen[a.Template]++
		}
	}
	if seen[11] == 0 || seen[12] == 0 {
		t.Fatalf("only one face of the pool was ever drawn: %v", seen)
	}
	// The amount is the pool's, not the wrapper's: one open of a 70-material
	// entry pays seventy, not one.
	first, _, _ := OpenRewardBoxes(1, src, []Award{{Template: 900, Amount: 1}})
	for _, a := range first {
		if a.Template == 11 && a.Amount != 70 {
			t.Fatalf("material amount = %d, want the pool's 70", a.Amount)
		}
	}
}

// TestOpenRewardBoxesHonoursDrawCount pins [draw count]: one open is not
// necessarily one prize, and collapsing it would quietly pay a tenth of a clear.
func TestOpenRewardBoxesHonoursDrawCount(t *testing.T) {
	src := fakeBoxes{
		boxes: map[uint32]RewardBox{
			900: {Pools: []RewardBoxPool{
				{Draws: 3, Candidates: []RewardBoxCandidate{{Template: 11, Count: 5}}},
			}},
		},
		items:      map[uint32]bool{11: true},
		containers: map[uint32]bool{900: true},
	}
	got, _, _ := OpenRewardBoxes(7, src, []Award{{Template: 900, Amount: 1}})
	if len(got) != 3 {
		t.Fatalf("%d rows, want the declared 3 draws", len(got))
	}
	for _, a := range got {
		if a.Template != 11 || a.Amount != 5 {
			t.Fatalf("row %+v, want three of template 11 amount 5", a)
		}
	}
}

// TestOpenRewardBoxesPaysOneWhenADrawCountIsAbsent keeps the absent [draw count]
// meaning once, which is what the open path does with it.
func TestOpenRewardBoxesPaysOneWhenADrawCountIsAbsent(t *testing.T) {
	src := fakeBoxes{
		boxes: map[uint32]RewardBox{
			900: {Pools: []RewardBoxPool{{Candidates: []RewardBoxCandidate{{Template: 11, Count: 1}}}}},
		},
		items:      map[uint32]bool{11: true},
		containers: map[uint32]bool{900: true},
	}
	got, _, _ := OpenRewardBoxes(7, src, []Award{{Template: 900, Amount: 1}})
	if len(got) != 1 || got[0].Template != 11 {
		t.Fatalf("got %+v, want a single draw", got)
	}
}

// TestOpenRewardBoxesDropsTheEmptyFace pins the reading of the reserved slot: an
// entry the item catalog does not know is the source saying "no prize", so it is
// paid as nothing rather than invented into an item.
func TestOpenRewardBoxesDropsTheEmptyFace(t *testing.T) {
	src := fakeBoxes{
		boxes: map[uint32]RewardBox{
			900: {Pools: []RewardBoxPool{{Draws: 1, Candidates: []RewardBoxCandidate{{Template: 12, Weight: 1}}}}},
		},
		items:      map[uint32]bool{},
		containers: map[uint32]bool{900: true},
	}
	got, _, skipped := OpenRewardBoxes(3, src, []Award{{Template: 900, Amount: 1}})
	if len(got) != 0 {
		t.Fatalf("the empty face paid %+v", got)
	}
	if len(skipped) != 1 || skipped[0] != "attunement_empty_prize_12" {
		t.Fatalf("skipped = %v, want the empty face recorded", skipped)
	}
}

// TestOpenRewardBoxesUnwrapsToTheProducts is the semantic guard on depth. The
// source wraps a prize more than once - equipment sits inside a box inside a
// box - and a wrapper that reaches the bag cannot be used at all, so the walk
// has to keep going until it holds something the player can actually hold. What
// the source itself says about these prizes is "dispensed in the opened state".
func TestOpenRewardBoxesUnwrapsToTheProducts(t *testing.T) {
	src := fakeBoxes{
		boxes: map[uint32]RewardBox{
			900: {Pools: []RewardBoxPool{{Draws: 1, Candidates: []RewardBoxCandidate{{Template: 901, Weight: 1}}}}},
			901: {Pools: []RewardBoxPool{{Draws: 1, Candidates: []RewardBoxCandidate{{Template: 902, Weight: 1}}}}},
			902: {Pools: []RewardBoxPool{{Draws: 1, Candidates: []RewardBoxCandidate{{Template: 11, Weight: 1}}}}},
		},
		items:      map[uint32]bool{11: true},
		containers: map[uint32]bool{900: true, 901: true, 902: true},
	}
	got, _, _ := OpenRewardBoxes(5, src, []Award{{Template: 900, Amount: 1}})
	if len(got) != 1 || got[0].Template != 11 {
		t.Fatalf("got %+v, want the product under three wrappers", got)
	}
}

// TestOpenRewardBoxesRefusesAnUnopenableBox pins the second half of the rule: a
// box this build cannot take apart - a selection box that needs the player to
// choose - must not be paid either, because the player cannot use it. It is
// reported instead, so the gap stays visible rather than silently costing a
// reward branch.
func TestOpenRewardBoxesRefusesAnUnopenableBox(t *testing.T) {
	src := fakeBoxes{
		boxes: map[uint32]RewardBox{
			900: {Pools: []RewardBoxPool{{Draws: 1, Candidates: []RewardBoxCandidate{{Template: 903, Weight: 1}}}}},
		},
		items:      map[uint32]bool{903: true},
		containers: map[uint32]bool{900: true, 903: true},
	}
	got, _, skipped := OpenRewardBoxes(5, src, []Award{{Template: 900, Amount: 1}})
	if len(got) != 0 {
		t.Fatalf("an unopenable box was paid: %+v", got)
	}
	if len(skipped) != 1 || !strings.Contains(skipped[0], "attunement_unopenable_box_903") {
		t.Fatalf("skipped = %v, want the unopenable box recorded", skipped)
	}
}

// TestOpenRewardBoxesStopsAtADepthCap keeps a source that nests boxes cyclically
// from spinning forever.
func TestOpenRewardBoxesStopsAtADepthCap(t *testing.T) {
	src := fakeBoxes{
		boxes: map[uint32]RewardBox{
			900: {Pools: []RewardBoxPool{{Draws: 1, Candidates: []RewardBoxCandidate{{Template: 901, Weight: 1}}}}},
			901: {Pools: []RewardBoxPool{{Draws: 1, Candidates: []RewardBoxCandidate{{Template: 900, Weight: 1}}}}},
		},
		items:      map[uint32]bool{},
		containers: map[uint32]bool{900: true, 901: true},
	}
	got, _, skipped := OpenRewardBoxes(5, src, []Award{{Template: 900, Amount: 1}})
	if len(got) != 0 {
		t.Fatalf("a cycle paid %+v", got)
	}
	if len(skipped) != 1 || skipped[0] != "attunement_reward_nesting_too_deep" {
		t.Fatalf("skipped = %v, want the depth cap recorded", skipped)
	}
}

// TestOpenRewardBoxesLeavesWhatItCannotOpenAlone covers the inert cases: a
// template that is not a wrapper is a plain prize, and no source at all must not
// disturb the awards or the seed.
func TestOpenRewardBoxesLeavesWhatItCannotOpenAlone(t *testing.T) {
	src := fakeBoxes{boxes: map[uint32]RewardBox{}, items: map[uint32]bool{11: true}}
	in := []Award{{Template: 11, Amount: 3}}

	got, seed, skipped := OpenRewardBoxes(9, src, in)
	if seed != 9 {
		t.Fatalf("plain award consumed the seed: %d", seed)
	}
	if !reflect.DeepEqual(got, in) || len(skipped) != 0 {
		t.Fatalf("plain award was rewritten: %+v / %v", got, skipped)
	}

	got, seed, _ = OpenRewardBoxes(9, nil, in)
	if seed != 9 || !reflect.DeepEqual(got, in) {
		t.Fatalf("nil source changed %+v / seed %d", got, seed)
	}

	if got, seed, _ := OpenRewardBoxes(9, src, nil); len(got) != 0 || seed != 9 {
		t.Fatalf("no awards produced %+v / seed %d", got, seed)
	}
}

// TestOpenRewardBoxesIsDeterministic pins that the unwrap draws from the run's
// own sequence: the same seed must pay the same ground, or a replay of a
// captured session could not be explained.
func TestOpenRewardBoxesIsDeterministic(t *testing.T) {
	src := fakeBoxes{
		boxes: map[uint32]RewardBox{
			900: {Pools: []RewardBoxPool{{Draws: 4, Candidates: []RewardBoxCandidate{
				{Template: 11, Weight: 500000, Count: 1},
				{Template: 12, Weight: 500000, Count: 2},
			}}}},
		},
		items:      map[uint32]bool{11: true, 12: true},
		containers: map[uint32]bool{900: true},
	}
	in := []Award{{Template: 900, Amount: 1}}
	a1, s1, _ := OpenRewardBoxes(42, src, in)
	a2, s2, _ := OpenRewardBoxes(42, src, in)
	if !reflect.DeepEqual(a1, a2) || s1 != s2 {
		t.Fatalf("same seed diverged: %+v/%d vs %+v/%d", a1, s1, a2, s2)
	}
	if s1 == 42 {
		t.Fatal("a multi-candidate pool did not advance the seed")
	}
	if _, s3, _ := OpenRewardBoxes(s1, src, in); s3 == s1 {
		t.Fatal("the second unwrap reused the seed")
	}
}
