package loot

import (
	"reflect"
	"testing"
)

// fakeBoxes is a hand-built source. The unit tests are about the unwrap rule,
// not about the shipped tables: a case built from the real catalog would only
// prove the catalog.
type fakeBoxes struct {
	boxes map[uint32]RewardBox
	items map[uint32]bool
}

func (f fakeBoxes) RewardBox(t uint32) (RewardBox, bool) { b, ok := f.boxes[t]; return b, ok }
func (f fakeBoxes) Item(t uint32) bool                   { return f.items[t] }

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
		items: map[uint32]bool{11: true, 12: true},
	}
	seen := map[uint32]int{}
	for seed := uint32(1); seed <= 400; seed++ {
		got, _ := OpenRewardBoxes(seed, src, []Award{{Template: 900, Amount: 1}})
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
	first, _ := OpenRewardBoxes(1, src, []Award{{Template: 900, Amount: 1}})
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
		items: map[uint32]bool{11: true},
	}
	got, _ := OpenRewardBoxes(7, src, []Award{{Template: 900, Amount: 1}})
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
		items: map[uint32]bool{11: true},
	}
	got, _ := OpenRewardBoxes(7, src, []Award{{Template: 900, Amount: 1}})
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
		items: map[uint32]bool{},
	}
	got, _ := OpenRewardBoxes(3, src, []Award{{Template: 900, Amount: 1}})
	if len(got) != 0 {
		t.Fatalf("the empty face paid %+v", got)
	}
}

// TestOpenRewardBoxesUnwrapsExactlyOneLayer is the semantic guard on depth. An
// inner wrapper *is* a prize (the random book, the jar the star-stone arrives
// in), so it reaches the player as itself; unwrapping it again would dissolve
// the very items the source pays.
func TestOpenRewardBoxesUnwrapsExactlyOneLayer(t *testing.T) {
	src := fakeBoxes{
		boxes: map[uint32]RewardBox{
			900: {Pools: []RewardBoxPool{{Draws: 1, Candidates: []RewardBoxCandidate{{Template: 901, Weight: 1}}}}},
			901: {Pools: []RewardBoxPool{{Draws: 1, Candidates: []RewardBoxCandidate{{Template: 11, Weight: 1}}}}},
		},
		items: map[uint32]bool{11: true, 901: true},
	}
	got, _ := OpenRewardBoxes(5, src, []Award{{Template: 900, Amount: 1}})
	if len(got) != 1 || got[0].Template != 901 {
		t.Fatalf("got %+v, want the inner wrapper 901 itself", got)
	}
}

// TestOpenRewardBoxesLeavesWhatItCannotOpenAlone covers both inert cases: a
// template that is not a wrapper is a plain prize, and no source at all must not
// disturb the awards or the seed.
func TestOpenRewardBoxesLeavesWhatItCannotOpenAlone(t *testing.T) {
	src := fakeBoxes{boxes: map[uint32]RewardBox{}, items: map[uint32]bool{11: true}}
	in := []Award{{Template: 11, Amount: 3}}

	got, seed := OpenRewardBoxes(9, src, in)
	if seed != 9 {
		t.Fatalf("plain award consumed the seed: %d", seed)
	}
	if !reflect.DeepEqual(got, in) {
		t.Fatalf("plain award was rewritten: %+v", got)
	}

	got, seed = OpenRewardBoxes(9, nil, in)
	if seed != 9 || !reflect.DeepEqual(got, in) {
		t.Fatalf("nil source changed %+v / seed %d", got, seed)
	}

	if got, seed := OpenRewardBoxes(9, src, nil); len(got) != 0 || seed != 9 {
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
		items: map[uint32]bool{11: true, 12: true},
	}
	in := []Award{{Template: 900, Amount: 1}}
	a1, s1 := OpenRewardBoxes(42, src, in)
	a2, s2 := OpenRewardBoxes(42, src, in)
	if !reflect.DeepEqual(a1, a2) || s1 != s2 {
		t.Fatalf("same seed diverged: %+v/%d vs %+v/%d", a1, s1, a2, s2)
	}
	if s1 == 42 {
		t.Fatal("a multi-candidate pool did not advance the seed")
	}
	if _, s3 := OpenRewardBoxes(s1, src, in); s3 == s1 {
		t.Fatal("the second unwrap reused the seed")
	}
}
