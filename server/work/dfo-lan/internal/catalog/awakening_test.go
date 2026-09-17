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
