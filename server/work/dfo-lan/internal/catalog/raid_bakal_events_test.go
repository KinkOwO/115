package catalog

import (
	"dfolan/internal/catalog/pvf"
	"testing"
)

func TestBakalEnterProjectionRejectsPartialAndHardModeEvents(t *testing.T) {
	tag := func(s string) pvf.Token { return pvf.Token{Type: 3, Text: s} }
	num := func(v int32) pvf.Token { return pvf.Token{Type: 0, Value: v} }
	pure := []pvf.Token{tag("[TRIGGER]"), tag("[ON ENTER DUNGEON]"), num(10), tag("[IF]"), tag("[HP]"), tag("[>]"), num(0), tag("[/TRIGGER]"), tag("[BEHAVIOR]"), tag("[SET]"), tag("[AWAKE]"), num(1), tag("[/BEHAVIOR]")}
	mixed := append([]pvf.Token(nil), pure[:len(pure)-1]...)
	mixed = append(mixed, tag("[AWAKE RANDOM DRAGON]"), num(900), tag("[/BEHAVIOR]"))
	ts := append(mixed, pure...)
	ts = append(ts, tag("[RAID PHASE OF HARDMODE]"))
	ts = append(ts, pure...)
	rules := parseBakalEnterSymbolRules(ts)
	if len(rules) != 1 || rules[0].Dungeon != 10 || len(rules[0].Conditions) != 1 || rules[0].Conditions[0].Operator != "[>]" || len(rules[0].Assignments) != 1 {
		t.Fatalf("partial/mixed or hard-mode rules escaped boundary: %+v", rules)
	}
}
