package catalog

import (
	"dfolan/internal/catalog/pvf"
	"testing"
)

func TestBakalBiddingRejectsMissingNormalAndInvalidWeights(t *testing.T) {
	tag := func(v string) pvf.Token { return pvf.Token{Type: 3, Text: v} }
	n := func(v int32) pvf.Token { return pvf.Token{Type: 0, Value: v} }
	ts := []pvf.Token{tag("[BIDDING REWARD COUNT]"), tag("[HARD]"), n(1), tag("[RATE LIST]"), n(9), n(100), tag("[/RATE LIST]"), tag("[/HARD]"), tag("[/BIDDING REWARD COUNT]")}
	if _, err := bakalNormalBiddingCount(ts, "[BIDDING REWARD COUNT]"); err == nil {
		t.Fatal("hard-mode-only count accepted as normal")
	}
	for _, values := range [][]uint32{{1}, {1, 0}, {1, 2, 1, 3}, {1, ^uint32(0), 2, 1}} {
		if _, err := bakalWeightedNumbers(values); err == nil {
			t.Fatal("invalid probability distribution accepted", values)
		}
	}
}
