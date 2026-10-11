package catalog

import (
	"dfolan/internal/catalog/pvf"
	"math"
	"math/rand"
	"reflect"
	"testing"
)

func TestBoosterSourceEmptyFaceRetainsProbabilityAndTuplePositions(t *testing.T) {
	tokens := []pvf.Token{{Type: 3, Text: "[booster info]"}, {Type: 3, Text: "[etc]"}}
	for _, v := range []int32{1, -1, 700000, 1, 17, 270000, 1, 18, 30000, 1} {
		tokens = append(tokens, pvf.Token{Type: 0, Value: v})
	}
	tokens = append(tokens, pvf.Token{Type: 3, Text: "[/etc]"}, pvf.Token{Type: 3, Text: "[/booster info]"})
	got := parseBoosterInfo(tokens)
	want := []BoosterRewardPool{{DrawCount: 1, Candidates: []BoosterRewardCandidate{{Template: math.MaxUint32, Weight: 700000, Count: 1}, {Template: 17, Weight: 270000, Count: 1}, {Template: 18, Weight: 30000, Count: 1}}}}
	if !reflect.DeepEqual(got, want) {
		t.Fatal(got)
	}
	counts := map[uint32]int{}
	rng := rand.New(rand.NewSource(7))
	for i := 0; i < 10000; i++ {
		counts[got[0].Pick(rng)[0].Template]++
	}
	if counts[math.MaxUint32] < 6800 || counts[math.MaxUint32] > 7200 || counts[18] < 200 || counts[18] > 400 {
		t.Fatal("empty face no longer carries source weight", counts)
	}
	for _, n := range []uint32{1, 3, 7} {
		p := BoosterRewardPool{DrawCount: n, Candidates: []BoosterRewardCandidate{{Template: 17, Count: 2}}}
		if len(p.Pick(rng)) != int(n) {
			t.Fatal("single candidate draw count lost", n)
		}
	}
}
