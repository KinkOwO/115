package inventory

import (
	"dfolan/internal/catalog"
	"dfolan/internal/catalog/pvf"
	"math"
	"strings"
	"testing"
)

func TestSymbolAttributesNativeFloatCritical(t *testing.T) {
	// Exact native tag-2 cell from 590004578; 3.5 is already a percent.
	s := catalog.ScriptRecord{Cells: []pvf.Token{{Type: 3, Text: "[enchant]"}, {Type: 3, Text: "[physical critical hit]"}, {Type: 2, Value: 1080033280}, {Type: 4, Value: 1080033280}, {Type: 2, Value: int32(math.Float32bits(float32(math.NaN())))}, {Type: 3, Text: "[/enchant]"}}}
	a := SymbolAttributes{}
	a.Add(s, false)
	if a["[physical critical hit]"] != 3.5 || a.Text() != "物理暴击率 +3.5%" {
		t.Fatalf("native float projection: %#v / %s", a, a.Text())
	}
}

func TestSymbolAttributesExcludeOutsideEnchantAndKeepBuffSkillsSeparate(t *testing.T) {
	tag := func(s string) pvf.Token { return pvf.Token{Type: 3, Text: s} }
	num := func(v int32) pvf.Token { return pvf.Token{Type: 0, Value: v} }
	source := catalog.ScriptRecord{Cells: []pvf.Token{tag("[grade]"), num(3), tag("[physical attack]"), num(999), tag("[enchant]"), tag("[physical attack]"), num(30), tag("[move speed]"), num(17), tag("[skill levelup]"), {Type: 6, Text: "[archer]"}, num(84), num(1), tag("[/enchant]"), tag("[fame value]"), num(40)}}
	normal, buff := SymbolAttributes{}, SymbolAttributes{}
	normal.Add(source, false)
	normal.Add(source, false)
	buff.Add(source, true)
	if normal["[physical attack]"] != 60 || normal["[move speed]"] != 34 || normal["skill:[archer]:84"] != 2 || len(normal) != 3 {
		t.Fatalf("wrong projection: %#v", normal)
	}
	if len(buff) != 1 || buff["skill:[archer]:84"] != 1 {
		t.Fatalf("buff included ordinary stats: %#v", buff)
	}
	if s := normal.Text(); !strings.Contains(s, "力量 +60") || !strings.Contains(s, "移动速度 +3.4%") || strings.Contains(s, "999") {
		t.Fatal(s)
	}
	if (SymbolAttributes{}).Text() != "无属性加成" {
		t.Fatal("empty state")
	}
}
