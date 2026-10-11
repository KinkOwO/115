package catalog

import "testing"

func TestIspinsCardsFollowSourceWeightsAndRanges(t *testing.T) {
	defs := map[uint16]IspinsOperationDefinition{}
	for pool := 0; pool < 3; pool++ {
		defs[uint16(10+pool)] = IspinsOperationDefinition{Type: 8, MemberLimit: "party", Difficulty: "normal", SelectPool: uint32(pool), Probability: 1, RandomRanges: []LegionOperationRange{{"party", "normal", 15, 25, 5}}}
	}
	last := func(bound int64) (int64, error) { return bound - 1, nil }
	cards, err := ChooseIspinsCards(defs, "party", "normal", last)
	if err != nil {
		t.Fatal(err)
	}
	for pool, card := range cards {
		if card.Index != uint16(10+pool) || card.Value != 25 || card.Grade != uint32(pool+1) {
			t.Fatal(cards)
		}
	}
	defs[44] = IspinsOperationDefinition{Type: 6, FixedValue: 7, MemberLimit: "party", Difficulty: "normal", SelectPool: 0, Probability: 99}
	cards, err = ChooseIspinsCards(defs, "party", "normal", last)
	if err != nil || cards[0].Index != 44 || cards[0].Value != 7 {
		t.Fatal(cards, err)
	}
	zero := defs[44]
	zero.Probability = 0
	defs[44] = zero
	cards, err = ChooseIspinsCards(defs, "party", "normal", last)
	if err != nil || cards[0].Index != 10 {
		t.Fatal(cards, err)
	}
	if _, err = ChooseIspinsCards(defs, "solo", "normal", last); err == nil {
		t.Fatal("wrong dimension selected")
	}
}
