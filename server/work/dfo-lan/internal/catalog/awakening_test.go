package catalog

import (
	"dfolan/internal/catalog/pvf"
	"reflect"
	"testing"
)

func TestAwakeningGrantBoundaries(t *testing.T) {
	tags := func(s string) pvf.Token { return pvf.Token{Type: 3, Text: s} }
	ints := func(n int32) pvf.Token { return pvf.Token{Type: 0, Value: n} }
	p := []pvf.Token{tags("[growtype 2]"), tags("[skill]"), ints(999), tags("[awakening 1]"), tags("[awakening skill]"), ints(86), ints(1), tags("[/awakening skill]"), tags("[pvp awakening skill]"), ints(99), ints(1), tags("[awakening 2]"), tags("[awakening skill]"), ints(245), ints(1), tags("[growtype 3]"), tags("[awakening 1]"), tags("[awakening skill]"), ints(87), ints(1)}
	want := map[byte]map[byte][]int32{1: {1: {86, 1}, 2: {245, 1}}, 2: {1: {87, 1}}}
	if got := AwakeningSkillGrants(p); !reflect.DeepEqual(got, want) {
		t.Fatal(got)
	}
}

// [growtype 1] 是 growtype 0（未转职基座）。无转职分支职业（[max grow count] 1，
// 只有 [growtype 1] 一段）把 [awakening 1..3] 授权写在该段内 —— 例如
// character/mage/creatormage.chr 与 character/swordman/dsswordman.chr —— 必须保留。
func TestAwakeningGrantKeepsUnadvancedGrowtype(t *testing.T) {
	tags := func(s string) pvf.Token { return pvf.Token{Type: 3, Text: s} }
	ints := func(n int32) pvf.Token { return pvf.Token{Type: 0, Value: n} }
	p := []pvf.Token{
		tags("[growtype 1]"),
		tags("[awakening 1]"), tags("[awakening skill]"), ints(273), ints(1), tags("[/awakening skill]"),
		tags("[awakening 3]"), tags("[awakening skill]"), ints(407), ints(1), tags("[/awakening skill]"),
	}
	want := map[byte]map[byte][]int32{0: {1: {273, 1}, 3: {407, 1}}}
	if got := AwakeningSkillGrants(p); !reflect.DeepEqual(got, want) {
		t.Fatal(got)
	}
	// Any [awakening skill] block seen before the first [growtype N] tag still
	// has grow == 0 and must stay refused, so a preamble can never be read as
	// growtype 0 grants.
	before := []pvf.Token{tags("[awakening skill]"), ints(9), ints(1)}
	if got := AwakeningSkillGrants(before); len(got) != 0 {
		t.Fatal(got)
	}
}
