package dungeon

import (
	"dfolan/internal/catalog"
	"testing"
)

func TestElvenmereRequiresSourceFloor(t *testing.T) {
	for _, floor := range []byte{0, 1, 101} {
		if _, err := ElvenmereFloorReward(nil, floor); err == nil {
			t.Fatal("missing source accepted")
		}
	}
	rules := &catalog.ElvenmereRules{MaxFloor: 2, Floors: map[byte]catalog.ElvenmereFloorRule{1: {Experience: 7}}}
	if _, err := ElvenmereFloorReward(rules, 2); err == nil {
		t.Fatal("missing row accepted")
	}
	if r, err := ElvenmereFloorReward(rules, 1); err != nil || r.Experience != 7 {
		t.Fatal(r, err)
	}
	rules.Floors[1] = catalog.ElvenmereFloorRule{Experience: 9}
	if r, _ := ElvenmereFloorReward(rules, 1); r.Experience != 9 {
		t.Fatal("reward ignores source")
	}
}
