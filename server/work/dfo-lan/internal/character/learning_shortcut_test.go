package character

import (
	"dfolan/internal/catalog/pvf"
	"testing"
)

// 历史夹具：已退休 skills.release 导出（3224 行）。只有 12 行的 [type] 不是
// [active]/[passive]：8 行是导出器解析不出的常量（导出成 "default"，如 job8
// atmage 138 spiralpress / 139 violentstorm），4 行根本没有 [type]（job4
// priest 133/134/250/253）。它们全是觉醒或 variation-point 行。
//
// 2026-09-23 实机：atmage 的 138/139 学得会、却注册不了快捷键 —— CMD28 被
// Active() 判成被动。ShortcutCapable 只认 [passive]。
func TestShortcutCapableFollowsPassiveMarkerOnly(t *testing.T) {
	variant := []pvf.Token{{Type: 0, Value: 1}}
	awakened := []pvf.Token{{Type: 0, Value: 40}}
	cases := []struct {
		name   string
		fields map[string][]pvf.Token
		want   bool
	}{
		{"active", map[string][]pvf.Token{"[type]": {{Type: 6, Text: "[active]"}}}, true},
		{"passive", map[string][]pvf.Token{"[type]": {{Type: 6, Text: "[passive]"}}}, false},
		{"atmage 138 spiralpress: unresolved constant", map[string][]pvf.Token{
			"[type]": {{Type: 6, Text: "default"}}, "[variation point]": variant, "[awakening maximum level]": awakened}, true},
		{"atmage 139 violentstorm: unresolved constant", map[string][]pvf.Token{
			"[type]": {{Type: 6, Text: "default"}}, "[variation point]": variant, "[awakening maximum level]": awakened}, true},
		{"priest 253 doomcrush: no [type] at all", map[string][]pvf.Token{"[variation point]": variant}, true},
	}
	for _, c := range cases {
		d := LearningDefinition{Fields: c.fields}
		if got := d.ShortcutCapable(); got != c.want {
			t.Fatalf("%s: ShortcutCapable = %v, want %v", c.name, got, c.want)
		}
		if d.Passive() == c.want {
			t.Fatalf("%s: Passive = %v, want %v", c.name, d.Passive(), !c.want)
		}
	}
}

// Active() keeps its narrow meaning on purpose: it decides automatic shortcut
// placement, so it must not start claiming rows whose [type] never resolved.
func TestActiveStillDemandsExplicitActive(t *testing.T) {
	if (LearningDefinition{Fields: map[string][]pvf.Token{"[type]": {{Type: 6, Text: "default"}}}}).Active() {
		t.Fatal("unresolved [type] must not count as [active]")
	}
	if !(LearningDefinition{Fields: map[string][]pvf.Token{"[type]": {{Type: 6, Text: "[active]"}}}}).Active() {
		t.Fatal("[active] must stay active")
	}
}
