package boostup

import (
	"dfolan/internal/catalog/pvf"
	"encoding/json"
	"errors"
	"testing"
)

func TestBoostBeadRestrictionFollowsSourceAndCurrentStep(t *testing.T) {
	c := &Catalog{Steps: []Step{{Number: 1}, {Number: 2, BlockEnchantBead: true}, {Number: 3}}}
	for _, test := range []struct {
		active, finished bool
		step, phase      byte
		blocked          bool
	}{
		{false, false, 2, 2, false}, {true, false, 1, 2, false},
		{true, false, 2, 0, true}, {true, false, 2, 1, true}, {true, false, 2, 2, true},
		{true, false, 3, 0, false}, {true, true, 4, 0, false},
	} {
		raw, e := WriteState(json.RawMessage(`{"unrelated":"preserve"}`), State{Version: 1, Activated: test.active, Training: Training{Step: test.step, Phase: test.phase, Finished: test.finished}})
		if e != nil {
			t.Fatal(e)
		}
		err := c.CheckEnchantBead(raw)
		if errors.Is(err, ErrEnchantBeadBlocked) != test.blocked || err != nil && !test.blocked {
			t.Fatalf("%+v: %v", test, err)
		}
	}
	var off *Catalog
	if e := off.CheckEnchantBead(json.RawMessage(`{}`)); e != nil {
		t.Fatal(e)
	}
	bad, _ := WriteState(json.RawMessage(`{}`), State{Version: 1, Activated: true, Training: Training{Step: 9}})
	if e := c.CheckEnchantBead(bad); e == nil {
		t.Fatal("corrupt active step bypassed guard")
	}
	const flag = "[block equipment enchanted bead]"
	for _, v := range []int32{0, 1, 2, -1} {
		got, e := sourceStepFlag([]pvf.Token{{Type: 3, Text: flag}, {Type: 0, Value: v}}, flag)
		if e != nil || got != (v == 1) {
			t.Fatal(v, got, e)
		}
	}
	if _, e := sourceStepFlag([]pvf.Token{{Type: 3, Text: flag}}, flag); e == nil {
		t.Fatal("missing flag value accepted")
	}
}
