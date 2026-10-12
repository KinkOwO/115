package extradrop

import (
	"testing"

	"dfolan/internal/catalog/pvf"
)

// syntheticTable 手搭一张与 `AdditionalDropRewardEvent.ctp` **同形**的表
// （记录索引/引用关系照实机 dump 抄）。用内存表而不是读归档：解析器要能在没有
// 760 MB PVF 的环境里被钉住。
//
// 刻意保留最容易解析错的两处：**`[reward table]` 是混排三元组**（文本 + 两个数）、
// **`[rule]` 只有第三组有**（可选引用）。
func syntheticTable() *pvf.CTPTable {
	name := func(s string) pvf.CTPCell { return pvf.CTPCell{Kind: "name", Name: s} }
	num := func(f float64) pvf.CTPCell { return pvf.CTPCell{Kind: "float", Float: f} }
	ref := func(n string, v ...uint64) pvf.CTPRef { return pvf.CTPRef{Name: n, Values: v} }

	recs := []pvf.CTPRecord{
		{Index: 0, Name: "[event period]", Cells: []pvf.CTPCell{name("2026-07-07 09:00:00"), name("2026-10-06 09:00:00")}},
		{Index: 1, Name: "[target dungeon type]", Cells: []pvf.CTPCell{name("ENDKEEPER OF ORDER")}},
		{Index: 2, Name: "[max amount]", Refs: []pvf.CTPRef{ref("[type]", 3), ref("[amount]", 4)}},
		{Index: 3, Name: "[type]", Cells: []pvf.CTPCell{name("drop item"), name("party drop item")}},
		{Index: 4, Name: "[amount]", Cells: []pvf.CTPCell{num(1000)}},
		{Index: 5, Name: "[group table]", Refs: []pvf.CTPRef{ref("[group]", 6, 12, 18)}},
		{Index: 6, Name: "[group]", Refs: []pvf.CTPRef{ref("[type]", 7), ref("[item level]", 8), ref("[item type]", 9), ref("[reward table]", 10)}},
		{Index: 7, Name: "[type]", Cells: []pvf.CTPCell{name("get item")}},
		{Index: 8, Name: "[item level]", Cells: []pvf.CTPCell{num(115)}},
		{Index: 9, Name: "[item type]", Cells: []pvf.CTPCell{name("[primer]"), name("[oath]"), name("[weapon]")}},
		{Index: 10, Name: "[reward table]", Cells: []pvf.CTPCell{
			name("rare"), num(590015808), num(1),
			name("unique"), num(590015808), num(2),
			name("primeval"), num(590015808), num(5),
		}},
		{Index: 12, Name: "[group]", Refs: []pvf.CTPRef{ref("[type]", 13), ref("[item level]", 14), ref("[item type]", 15), ref("[reward table]", 16)}},
		{Index: 13, Name: "[type]", Cells: []pvf.CTPCell{name("drop item")}},
		{Index: 14, Name: "[item level]", Cells: []pvf.CTPCell{num(115)}},
		{Index: 15, Name: "[item type]", Cells: []pvf.CTPCell{name("[primer]"), name("[oath]")}},
		{Index: 16, Name: "[reward table]", Cells: []pvf.CTPCell{name("unique"), num(590015808), num(5)}},
		{Index: 18, Name: "[group]", Refs: []pvf.CTPRef{ref("[type]", 19), ref("[item level]", 20), ref("[item type]", 21), ref("[rule]", 22), ref("[reward table]", 23)}},
		{Index: 19, Name: "[type]", Cells: []pvf.CTPCell{name("party drop item")}},
		{Index: 20, Name: "[item level]", Cells: []pvf.CTPCell{num(115)}},
		{Index: 21, Name: "[item type]", Cells: []pvf.CTPCell{name("[primer]")}},
		{Index: 22, Name: "[rule]", Cells: []pvf.CTPCell{name("check only higher rarity")}},
		{Index: 23, Name: "[reward table]", Cells: []pvf.CTPCell{name("legendary"), num(590015808), num(10)}},
	}
	return &pvf.CTPTable{
		Path:    "live/event/kor/2026/0326_oracleresearch/additionaldroprewardevent.ctp",
		Version: 1, RecordCount: uint32(len(recs)), Records: recs,
		Columns: []pvf.CTPColumn{
			{Name: "[event period]", Records: []uint64{0}},
			{Name: "[group table]", Records: []uint64{5}},
			{Name: "[max amount]", Records: []uint64{2}},
			{Name: "[target dungeon type]", Records: []uint64{1}},
		},
	}
}

func TestParseSyntheticShape(t *testing.T) {
	c, err := Parse(syntheticTable())
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if c.TargetDungeonType != "ENDKEEPER OF ORDER" {
		t.Fatalf("target dungeon type = %q", c.TargetDungeonType)
	}
	if c.PeriodStart != "2026-07-07 09:00:00" || c.PeriodEnd != "2026-10-06 09:00:00" {
		t.Fatalf("period = %q..%q", c.PeriodStart, c.PeriodEnd)
	}
	if c.MaxAmount != 1000 {
		t.Fatalf("max amount = %d, want 1000", c.MaxAmount)
	}
	if len(c.MaxTypes) != 2 || c.MaxTypes[0] != "drop item" {
		t.Fatalf("max types = %v", c.MaxTypes)
	}
	if len(c.Groups) != 3 {
		t.Fatalf("groups = %d, want 3", len(c.Groups))
	}
	g, ok := c.GroupByType("get item")
	if !ok {
		t.Fatal("缺 get item 组")
	}
	if g.ItemLevel != 115 || len(g.ItemTypes) != 3 {
		t.Fatalf("get item 组: level=%d types=%v", g.ItemLevel, g.ItemTypes)
	}
	if len(g.Tiers) != 3 {
		t.Fatalf("get item tiers = %d, want 3", len(g.Tiers))
	}
	want := []RewardTier{{"rare", 590015808, 1}, {"unique", 590015808, 2}, {"primeval", 590015808, 5}}
	for i := range want {
		if g.Tiers[i] != want[i] {
			t.Fatalf("tier[%d] = %+v, want %+v", i, g.Tiers[i], want[i])
		}
	}
	p, ok := c.GroupByType("party drop item")
	if !ok || p.Rule != "check only higher rarity" {
		t.Fatalf("party drop item: ok=%v rule=%q", ok, p.Rule)
	}
	if d, _ := c.GroupByType("drop item"); d.Rule != "" {
		t.Fatalf("drop item 不该有 rule，得到 %q", d.Rule)
	}
}

// TestParseRejectsBadShapes 形状不符必须拒绝（列错位时发错物品比不发更糟）。
func TestParseRejectsBadShapes(t *testing.T) {
	cases := []struct {
		name   string
		break_ func(*pvf.CTPTable)
	}{
		{"缺 event period", func(t *pvf.CTPTable) { t.Columns = t.Columns[1:] }},
		{"event period 只有一个文本", func(t *pvf.CTPTable) { t.Records[0].Cells = t.Records[0].Cells[:1] }},
		{"target dungeon type 空串", func(t *pvf.CTPTable) { t.Records[1].Cells[0].Name = "" }},
		{"group table 没有组", func(t *pvf.CTPTable) { t.Records[5].Refs[0].Values = nil }},
		{"reward table 不是 3 的倍数", func(t *pvf.CTPTable) { t.Records[10].Cells = t.Records[10].Cells[:2] }},
		{"reward table 物品号是 0", func(t *pvf.CTPTable) { t.Records[10].Cells[1].Float = 0 }},
		{"item type 为空", func(t *pvf.CTPTable) { t.Records[9].Cells = nil }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tbl := syntheticTable()
			tc.break_(tbl)
			if _, err := Parse(tbl); err == nil {
				t.Fatalf("期望报错：%s", tc.name)
			}
		})
	}
}

func TestResolveEventPath(t *testing.T) {
	index := "89 `Live/Event/Kor/2022/0618_ThreeBuffEvent/ThreeBuffEvent.evt` " +
		"766 `Live/Event/Kor/2026/0326_DropsLuckyBox/DropsLuckyBox.evt` " +
		"1093 `Live/Event/Kor/2026/0326_OracleResearch/AdditionalDropRewardEvent.ctp` " +
		"516 `Live/Else/Univ/2026/0804_2026_AradPass_season3/NewAradPassEvent.evt` "
	got, err := ResolveEventPath(index, EventID)
	if err != nil {
		t.Fatalf("ResolveEventPath: %v", err)
	}
	if got != "live/event/kor/2026/0326_oracleresearch/additionaldroprewardevent.ctp" {
		t.Fatalf("path = %q", got)
	}
	for _, bad := range []string{
		"313 `a/b.evt` ",                 // 没有 1093
		"1093 `a/b.ctp` 1093 `c/d.ctp` ", // 重复绑定
		"1093 `a/b.txt` ",                // 后缀不认
		"1093 `/abs/b.ctp` ",             // 绝对路径
		"1093 `../b.ctp` ",               // 含 ..
		"1093 `a/b.ctp` 331 ",            // token 不成对
	} {
		if _, err := ResolveEventPath(bad, EventID); err == nil {
			t.Fatalf("期望报错：%q", bad)
		}
	}
}
