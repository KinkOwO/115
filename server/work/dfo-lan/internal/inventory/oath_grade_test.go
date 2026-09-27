package inventory

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// 本文件固定「誓约/引子装备稀有度 -> noti 2838 档位」这条链。
//
// 关键点有两个，都是踩过坑的：
//   - rarity 的**数值序不是机制序**（legendary=6 在 epic=4 之前），所以必须查表，
//     任何 `40 + rarity` 的写法都会把 legendary 和 epic 换位；
//   - 没穿装备时必须落回 normal(40)，否则隐藏 BOSS（必出太初）又会场场登场。

func writeOathTable(t *testing.T, entries map[string]OathGradeEntry) string {
	t.Helper()
	b, err := json.Marshal(map[string]any{"source": map[string]any{"checksum": "test"}, "entries": entries})
	if err != nil {
		t.Fatal(err)
	}
	p := filepath.Join(t.TempDir(), "oath-grades.json")
	if err := os.WriteFile(p, b, 0o600); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestOathGradeRarityMapping(t *testing.T) {
	// 这张表就是 §10.1 的 [seasonlevel oath item] 五档装备。
	cases := []struct {
		rarity int32
		grade  uint16
		name   string
	}{
		{2, 41, "rare"},
		{3, 42, "unique"},
		{6, 43, "legendary"},
		{4, 44, "epic"},
		{8, 45, "primeval"},
	}
	for _, c := range cases {
		got, ok := oathGradeByRarity[c.rarity]
		if !ok {
			t.Fatalf("rarity %d (%s) has no tier", c.rarity, c.name)
		}
		if got != c.grade {
			t.Fatalf("rarity %d -> %d, want %d (%s)", c.rarity, got, c.grade, c.name)
		}
	}
	// 机制序：legendary(43) 必须低于 epic(44)，尽管它的 rarity 值更大。
	if oathGradeByRarity[6] >= oathGradeByRarity[4] {
		t.Fatal("legendary must rank below epic; rarity values are not monotonic")
	}
	// 只有 primeval 会召唤隐藏 BOSS，所以它必须是唯一到 45 的档。
	for r, g := range oathGradeByRarity {
		if g == 45 && r != 8 {
			t.Fatalf("rarity %d also maps to 45", r)
		}
	}
}

func TestOathGradeTableGrades(t *testing.T) {
	tab, err := LoadOathGradeTable(writeOathTable(t, map[string]OathGradeEntry{
		"100610096": {Family: "oath", Rarity: 8, Type: "[oath]"},
		"100313751": {Family: "oath", Rarity: 4, Type: "[oath]"},
		"100401592": {Family: "primer", Rarity: 2, Type: "[primer]"},
		"100401597": {Family: "primer", Rarity: 6, Type: "[primer]"},
	}))
	if err != nil {
		t.Fatal(err)
	}
	if tab.Len() != 4 {
		t.Fatalf("Len = %d, want 4", tab.Len())
	}

	// 一件都没穿：两个都是 normal，隐藏 BOSS 不会出现。
	primer, oath := tab.Grades(nil)
	if primer != OathGradeNormal || oath != OathGradeNormal {
		t.Fatalf("bare = (%d,%d), want (%d,%d)", primer, oath, OathGradeNormal, OathGradeNormal)
	}

	// 穿 primeval 誓约 + legendary 引子：只有这种情况才到 45。
	primer, oath = tab.Grades([]BagEquipment{
		{Slot: 47, Template: 100610096},
		{Slot: 45, Template: 100401597},
	})
	if oath != 45 {
		t.Fatalf("oath = %d, want 45 for a primeval oath item", oath)
	}
	if primer != 43 {
		t.Fatalf("primer = %d, want 43 for a legendary primer item", primer)
	}

	// 同一家族多件时取最高：epic(44) + primeval(45) -> 45。
	_, oath = tab.Grades([]BagEquipment{
		{Slot: 47, Template: 100313751},
		{Slot: 45, Template: 100610096},
	})
	if oath != 45 {
		t.Fatalf("oath = %d, want the best worn item (45)", oath)
	}

	// 穿的是表外装备（普通装备）：仍然 normal。
	primer, oath = tab.Grades([]BagEquipment{{Slot: 0, Template: 10000}})
	if primer != OathGradeNormal || oath != OathGradeNormal {
		t.Fatalf("non-oath gear = (%d,%d), want normal", primer, oath)
	}

	// nil 表（没配 -oath-grades-table）也必须是 normal，不能 panic。
	var nilTab *OathGradeTable
	if p, o := nilTab.Grades([]BagEquipment{{Slot: 47, Template: 100610096}}); p != OathGradeNormal || o != OathGradeNormal {
		t.Fatalf("nil table = (%d,%d), want normal", p, o)
	}
}

func TestLoadOathGradeTableRejectsBadInput(t *testing.T) {
	bad := []map[string]OathGradeEntry{
		{"1": {Family: "oath", Rarity: 5}},     // rarity 5 无档位
		{"1": {Family: "weapon", Rarity: 8}},   // 家族不认识
		{"x": {Family: "oath", Rarity: 8}},     // id 不是十进制
	}
	for i, entries := range bad {
		if _, err := LoadOathGradeTable(writeOathTable(t, entries)); err == nil {
			t.Errorf("case %d must be rejected", i)
		}
	}
	if _, err := LoadOathGradeTable(filepath.Join(t.TempDir(), "missing.json")); err == nil {
		t.Error("missing file must be rejected")
	}
}
