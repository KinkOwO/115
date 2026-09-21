package catalog

import (
	"dfolan/internal/catalog/pvf"
	"strings"
	"testing"
)

func areaCells(middle ...pvf.Token) []pvf.Token {
	out := []pvf.Token{{Type: 3, Text: "[area]"}, {Type: 0, Value: 43}, {Type: 6, Text: "stormpass.map"}}
	return append(out, append(middle, pvf.Token{Type: 3, Text: "[normal]"}, pvf.Token{Type: 3, Text: "[/area]"})...)
}

func TestParseWorldAreasOdysseyEnterLevel(t *testing.T) {
	// 单数值 cell：[need level] 与 [odyssey enter level] 并存（风云径 43/* 实源形态）。
	rows, e := parseWorldAreas(43, areaCells(
		pvf.Token{Type: 3, Text: "[permission]"},
		pvf.Token{Type: 3, Text: "[need level]"}, pvf.Token{Type: 0, Value: 50},
		pvf.Token{Type: 3, Text: "[odyssey enter level]"}, pvf.Token{Type: 0, Value: 45},
		pvf.Token{Type: 3, Text: "[/permission]"},
	))
	if e != nil || len(rows) != 1 {
		t.Fatalf("single-value parse: %v %v", rows, e)
	}
	a := rows[0]
	if a.MinimumLevel != 50 || a.OdysseyEnterLevel != 45 {
		t.Fatalf("parsed need=%d odyssey=%d", a.MinimumLevel, a.OdysseyEnterLevel)
	}
	if a.RequiredLevel(true) != 45 || a.RequiredLevel(false) != 50 {
		t.Fatalf("required levels: odyssey=%d normal=%d", a.RequiredLevel(true), a.RequiredLevel(false))
	}
	if len(a.Pending) != 0 {
		t.Fatalf("unexpected pending: %v", a.Pending)
	}

	// 条件式（多 cell）：不能解释，落 Pending 且不产生 odyssey 门槛。
	rows, e = parseWorldAreas(43, areaCells(
		pvf.Token{Type: 3, Text: "[permission]"},
		pvf.Token{Type: 3, Text: "[odyssey enter level]"}, pvf.Token{Type: 0, Value: 45}, pvf.Token{Type: 0, Value: 114},
		pvf.Token{Type: 3, Text: "[/permission]"},
	))
	if e != nil || len(rows) != 1 {
		t.Fatalf("conditional parse: %v %v", rows, e)
	}
	a = rows[0]
	if a.OdysseyEnterLevel != 0 || !strings.Contains(a.Pending[0], "conditional odyssey level rule") {
		t.Fatalf("conditional expected pending, got odyssey=%d pending=%v", a.OdysseyEnterLevel, a.Pending)
	}
}

// TestWorldCatalogOdysseyEnterLevel 锁住修补后的运行时目录事实：
// 78 个双标签区、风云径 43/* need 50 / odyssey 45、area 总数与
// source.checksum 保持不变（checksum 变更会触发 world/character 版本门禁）。
func TestWorldCatalogOdysseyEnterLevel(t *testing.T) {
	cat, e := LoadWorld("../../configs/world.generated.json")
	if e != nil {
		t.Fatal(e)
	}
	if cat.Source.Checksum != "7ef2db59331f7e5b18b2f250b8b907526bf2c94b17a7312036cf599644d88e80" {
		t.Fatal("source checksum changed:", cat.Source.Checksum)
	}
	if len(cat.Areas) != 694 {
		t.Fatalf("area count = %d, want 694", len(cat.Areas))
	}
	dual := 0
	for key, a := range cat.Areas {
		if a.OdysseyEnterLevel == 0 {
			continue
		}
		dual++
		if a.MinimumLevel == 0 {
			t.Fatalf("%s has odyssey tag without need level", key)
		}
	}
	if dual != 78 {
		t.Fatalf("odyssey-tagged areas = %d, want 78", dual)
	}
	for _, key := range []string{"43/0", "43/1", "43/2"} {
		a, ok := cat.Areas[key]
		if !ok {
			t.Fatal("missing area", key)
		}
		if a.MinimumLevel != 50 || a.OdysseyEnterLevel != 45 {
			t.Fatalf("%s need=%d odyssey=%d, want 50/45", key, a.MinimumLevel, a.OdysseyEnterLevel)
		}
		if a.RequiredLevel(true) != 45 || a.RequiredLevel(false) != 50 {
			t.Fatalf("%s required levels wrong", key)
		}
	}
}
