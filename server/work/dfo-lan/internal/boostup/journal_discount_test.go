package boostup

import (
	"encoding/json"
	"testing"
)

func TestBoostJournalDiscountRequiresCurrentClaimedStep(t *testing.T) {
	c := &Catalog{Steps: make([]Step, 11), JournalDiscounts: []JournalDiscount{{Step: 10, Phase: 2, Gold: 100}}}
	for _, v := range []struct {
		step, phase                     byte
		active, finished, claimed, want bool
	}{
		{9, 2, true, false, true, false}, {10, 0, true, false, false, false}, {10, 1, true, false, false, false},
		{10, 2, true, false, true, true}, {10, 2, false, false, true, false}, {12, 0, true, true, true, false},
	} {
		raw, e := WriteState(json.RawMessage(`{}`), State{Version: 1, Activated: v.active, Training: Training{Step: v.step, Phase: v.phase, Finished: v.finished, Claimed: map[byte]bool{10: v.claimed}}})
		if e != nil {
			t.Fatal(e)
		}
		g, m, e := c.JournalTransformDiscount(raw)
		if e != nil || m != 0 || (g == 100) != v.want {
			t.Fatal(v, g, m, e)
		}
	}
	raw, _ := WriteState(json.RawMessage(`{}`), State{Version: 1, Activated: true, Training: Training{Step: 10, Phase: 2}})
	if _, _, e := c.JournalTransformDiscount(raw); e == nil {
		t.Fatal("unclaimed state became discount proof")
	}
}
