package catalog

import (
	"os"
	"path/filepath"
	"testing"
)

// 两份积分源表都是**文本**（UTF-16 → pvf.Archive.ReadText 自动解码），
// 这里用最小样本钉住行格式与查表口径。
const setPointInfoSample = `
[set point]
 [table]
  [info]
   [group] 55
   [awakening] 0
   [part set index] -1
   [value] 65
  [/info]
  [info]
   [group] 55
   [awakening] 1
   [part set index] -1
   [value] 75
  [/info]
  [info]
   [group] 51
   [awakening] 0
   [part set index] -1
   [value] 115
  [/info]
  [info]
   [group] 51
   [awakening] 0
   [part set index] 16201
   [value] 130
  [/info]
 [/table]
[/set point]
`

const oathPointInfoSample = `
[oath point]
 [table]
  100313750 0 -1 265
  100610095 0 -1 355
  100313752 0 16201 265
  100610042 0 16201 355
  100610043 1 16201 455
 [/table]
[/oath point]
`

func TestParsePointRulesSample(t *testing.T) {
	setTable, e := ParseSetPointInfo(setPointInfoSample)
	if e != nil {
		t.Fatalf("parse set point: %v", e)
	}
	if len(setTable.Rules) != 4 {
		t.Fatalf("set point rows = %d, want 4 (%+v)", len(setTable.Rules), setTable.Rules)
	}
	oathTable, e := ParseOathPointInfo(oathPointInfoSample)
	if e != nil {
		t.Fatalf("parse oath point: %v", e)
	}
	if len(oathTable.Rules) != 5 {
		t.Fatalf("oath point rows = %d, want 5 (%+v)", len(oathTable.Rules), oathTable.Rules)
	}

	p := PointRules{Set: setTable, Oath: oathTable}
	// 未调适（awakening 0）也有分：这是"0 分不是设计使然"的直接证据。
	if v, ok := p.SetPointFor(55, 0, -1); !ok || v != 65 {
		t.Fatalf("set point 55/0/-1 = %d ok=%v, want 65", v, ok)
	}
	if v, ok := p.SetPointFor(55, 1, -1); !ok || v != 75 {
		t.Fatalf("set point 55/1/-1 = %d ok=%v, want 75", v, ok)
	}
	// 同一个 group 在"有套装号"与"通用（-1）"两行是不同的值，必须精确匹配。
	if v, ok := p.SetPointFor(51, 0, 16201); !ok || v != 130 {
		t.Fatalf("set point 51/0/16201 = %d ok=%v, want 130", v, ok)
	}
	if v, ok := p.SetPointFor(51, 0, -1); !ok || v != 115 {
		t.Fatalf("set point 51/0/-1 = %d ok=%v, want 115", v, ok)
	}
	if _, ok := p.SetPointFor(51, 0, 999); ok {
		t.Fatal("unknown part set index must not match")
	}
	if v, ok := p.OathPointFor(100610042, 0, 16201); !ok || v != 355 {
		t.Fatalf("oath point 100610042/0/16201 = %d ok=%v, want 355", v, ok)
	}
	if v, ok := p.OathPointFor(100610043, 1, 16201); !ok || v != 455 {
		t.Fatalf("oath point 100610043/1/16201 = %d ok=%v, want 455", v, ok)
	}
	if _, ok := p.OathPointFor(100610042, 1, 16201); ok {
		t.Fatal("awakening must be part of the key")
	}
}

// 档位阶梯：`[grade list]` 是"积分 → 档位/稀有度"的换算表（截图里的「稀有 I 达成750」）。
func TestParseSetPointGradeLadder(t *testing.T) {
	const sample = `
[set point]
 [table]
  [info]
   [group] 55
   [awakening] 0
   [part set index] -1
   [value] 65
  [/info]
 [/table]
[/set point]
[grade list]
1 750 <37::set_grade_01> <37::set_grade_number_01> ` + "`rare`" + ` ` + "`purple_light`" + `
2 1200 <37::set_grade_06> <37::set_grade_number_01> ` + "`unique`" + ` ` + "`amplify_pink`" + `
5 2550 <37::set_grade_21> <37::set_grade_number_00> ` + "`primeval`" + ` ` + "`primeval`" + `
[/grade list]
[add parameters]
 [part set index]
  16201
 [/part set index]
[/add parameters]
`
	table, e := ParseSetPointInfo(sample)
	if e != nil {
		t.Fatalf("parse: %v", e)
	}
	// 关键：`[add parameters]` 里的裸 `[part set index]` 与数字列表不得污染规则/档位。
	if len(table.Rules) != 1 {
		t.Fatalf("rules = %+v, want exactly the one [set point] row", table.Rules)
	}
	if len(table.Grades) != 3 {
		t.Fatalf("grades = %+v, want 3", table.Grades)
	}
	if table.Grades[0].Threshold != 750 || table.Grades[0].Rarity != "rare" {
		t.Fatalf("grade[0] = %+v", table.Grades[0])
	}
	if table.Grades[2].Grade != 5 || table.Grades[2].Threshold != 2550 || table.Grades[2].Rarity != "primeval" {
		t.Fatalf("grade[2] = %+v", table.Grades[2])
	}
	p := PointRules{Set: table}
	if g, ok := p.GradeFor(1000); !ok || g.Threshold != 750 {
		t.Fatalf("GradeFor(1000) = %+v ok=%v, want the 750 row", g, ok)
	}
	if g, ok := p.GradeFor(2550); !ok || g.Grade != 5 {
		t.Fatalf("GradeFor(2550) = %+v ok=%v, want grade 5", g, ok)
	}
	if _, ok := p.GradeFor(749); ok {
		t.Fatal("below the lowest threshold must not map to a grade")
	}
}

func TestParsePointRulesRejectEmpty(t *testing.T) {
	if _, e := ParseSetPointInfo("[set point]\n [table]\n [/table]\n[/set point]\n"); e == nil {
		t.Fatal("empty set point table must be refused")
	}
	if _, e := ParseOathPointInfo("[oath point]\n [table]\n [/table]\n[/oath point]\n"); e == nil {
		t.Fatal("empty oath point table must be refused")
	}
}

// 聚合口径：按模板精确查、套装号行不存在时回退 -1 通用行、查不到的件不计分。
func TestOathPointsAggregation(t *testing.T) {
	table, e := ParseOathPointInfo(oathPointInfoSample)
	if e != nil {
		t.Fatalf("parse: %v", e)
	}
	p := PointRules{Oath: table}
	// 100610095 → 355（-1 行）、100610042 → 355（16201 行）、未知模板不计分。
	total, hits := p.OathPoints([]PointItem{
		{Template: 100610095, PartSetIndex: -1},
		{Template: 100610042, PartSetIndex: 16201},
		{Template: 999999, PartSetIndex: -1},
	})
	if total != 355+355 || hits != 2 {
		t.Fatalf("oath points = %d hits=%d, want %d/2", total, hits, 355+355)
	}
	// 套装号行不存在 ⇒ 回退到同一模板的 -1 行（这里 100610042 没有 -1 行，
	// 但该 (模板, 档位) 只有一行 16201 ⇒ 属"唯一行"确定命中）。
	if total, hits := p.OathPoints([]PointItem{{Template: 100610042, PartSetIndex: 16212}}); total != 355 || hits != 1 {
		t.Fatalf("single-row fallback = %d/%d, want 355/1", total, hits)
	}
	// awakening 参与键：1 档的 100610043 有分。
	if total, hits := p.OathPoints([]PointItem{{Template: 100610043, Awakening: 1, PartSetIndex: 16201}}); total != 455 || hits != 1 {
		t.Fatalf("awakening row = %d/%d, want 455/1", total, hits)
	}
	// 源里没有的档位（老件/未知档位）回退 0 档基础分，而不是把整件丢掉。
	if total, hits := p.OathPoints([]PointItem{{Template: 100610095, Awakening: 9, PartSetIndex: -1}}); total != 355 || hits != 1 {
		t.Fatalf("unknown awakening must fall back to 0; got %d/%d, want 355/1", total, hits)
	}
	// SetPoints 不再是缺口：两层映射（模板 →能力组→ (group,awakening) 行）现在可算。
	// 见 TestSetPointsAggregation。
}

// SetPoints 的两层映射与归属规则：
//
//	模板 --(AbilityGroups)--> 能力组号 --(+档位)--> setpointinfo.cos 的行
//
// 三种行的处理各钉一条：`[part set index] = -1` 用本件套装号补；补不出（本件没有套装号）
// **不累加**；档位只精确匹配（不回退 0 档，否则会把 0 分算成有分）。
func TestSetPointsAggregation(t *testing.T) {
	table, e := ParseSetPointInfo(setPointInfoSample)
	if e != nil {
		t.Fatalf("parse: %v", e)
	}
	p := PointRules{
		Set: table,
		AbilityGroups: map[uint32][]uint32{
			1001: {55},      // 只命中 -1 行 → 靠本件套装号补
			1002: {55},      // 本件没有套装号 → 命不中
			1003: {51},      // 同组两行：-1 行 + 16201 行
			1004: {999},     // 组不在表里
			1005: {55, 999}, // 多组，只有一组有行
		},
	}
	cases := []struct {
		name      string
		item      PointItem
		wantTotal uint32
		wantHits  int
	}{
		{"-1 行用本件套装号补，命中", PointItem{Template: 1001, PartSetIndex: 16201}, 65, 1},
		{"没有套装号可归属 ⇒ 不累加", PointItem{Template: 1002, PartSetIndex: -1}, 0, 0},
		{"两行都用本件套装号 ⇒ 累加", PointItem{Template: 1003, PartSetIndex: 16201}, 115 + 130, 1},
		{"能力组不在表里 ⇒ 0", PointItem{Template: 1004, PartSetIndex: 16201}, 0, 0},
		{"多组里只有一组有行", PointItem{Template: 1005, PartSetIndex: 16201}, 65, 1},
		{"档位只精确匹配，不回退 0 档", PointItem{Template: 1003, Awakening: 3, PartSetIndex: 16201}, 0, 0},
		{"表里没有的档位", PointItem{Template: 1001, Awakening: 2, PartSetIndex: 16201}, 0, 0},
		{"未知模板不猜", PointItem{Template: 999999, PartSetIndex: 16201}, 0, 0},
	}
	for _, c := range cases {
		total, hits := p.SetPoints([]PointItem{c.item})
		if total != c.wantTotal || hits != c.wantHits {
			t.Errorf("%s: got %d/%d, want %d/%d", c.name, total, hits, c.wantTotal, c.wantHits)
		}
	}
	// 合计与命中件数按件累加。
	if total, hits := p.SetPoints([]PointItem{
		{Template: 1001, PartSetIndex: 16201},
		{Template: 1003, PartSetIndex: 16201},
		{Template: 1002, PartSetIndex: -1},
	}); total != 65+115+130 || hits != 2 {
		t.Errorf("aggregate = %d/%d, want %d/2", total, hits, 65+115+130)
	}
	// 未装载能力组 ⇒ 返回"算不出"(0,0)，调用方不得当"角色积分为 0"。
	if total, hits := (PointRules{Set: table}).SetPoints([]PointItem{{Template: 1001, PartSetIndex: 16201}}); total != 0 || hits != 0 {
		t.Errorf("missing ability groups must report uncomputable, got %d/%d", total, hits)
	}
}

// 实机验收锚点：角色 1 的穿戴（2026-10-04 存档只读查询）
// 晶体 100401633 / 100401592 / 100401632 + 誓约核心 100610065。
//
// 真实源表下的分数：125（`0 16210 125`）+ 45（`0 -1 45`）+ 85（`0 16210 85`）
// + 455（`0 16210 455`）= **710** —— 这就是"誓约积分为 0"修好后实机应看到的数字
// （第一档阈值 750）。改回退口径前先看这里为什么是这些数。
func TestOathPointsRealCharacterOne(t *testing.T) {
	path := filepath.Join("..", "..", "runtime", "pvfinspect-primer", "lv1", "11-oathpointinfo.cos.txt")
	text, e := os.ReadFile(path)
	if e != nil {
		t.Skipf("真实源不在（先跑 pvfinspect 导出）: %v", e)
	}
	oath, e := ParseOathPointInfo(string(text))
	if e != nil {
		t.Fatalf("parse real oath point: %v", e)
	}
	p := PointRules{Oath: oath}
	items := []PointItem{
		{Template: 100401633, PartSetIndex: -1}, // 源里只有 `0 16210 125`
		{Template: 100401592, PartSetIndex: -1}, // `0 -1 45`
		{Template: 100401632, PartSetIndex: -1}, // `0 16210 85`
		{Template: 100610065, PartSetIndex: -1}, // `0 16210 455`
	}
	total, hits := p.OathPoints(items)
	if total != 710 || hits != 4 {
		t.Fatalf("character 1 oath points = %d (hits %d), want 710 (hits 4)", total, hits)
	}
}

// 它钉住"未调适也有分"和"档位阶梯到 2550 = primeval"这两条事实 ——
// 前者是用户"誓约积分为 0"必须被修好的依据，后者对应截图里的「稀有 I 达成750 / 激活需 2550」。
func TestParsePointRulesRealSource(t *testing.T) {
	dir := filepath.Join("..", "..", "runtime", "pvfinspect-primer", "lv1")
	setText, e := os.ReadFile(filepath.Join(dir, "10-setpointinfo.cos.txt"))
	if e != nil {
		t.Skipf("真实源不在（先跑 pvfinspect 导出）: %v", e)
	}
	table, e := ParseSetPointInfo(string(setText))
	if e != nil {
		t.Fatalf("parse real set point: %v", e)
	}
	if len(table.Rules) < 50 {
		t.Fatalf("real set point rules = %d, want >=50", len(table.Rules))
	}
	var found65, found115 bool
	for _, r := range table.Rules {
		if r.Group == 55 && r.Awakening == 0 && r.PartSetIndex == -1 && r.Value == 65 {
			found65 = true
		}
		if r.Group == 51 && r.Awakening == 0 && r.PartSetIndex == -1 && r.Value == 115 {
			found115 = true
		}
	}
	if !found65 || !found115 {
		t.Fatalf("real source lost the awakening-0 rows (55→65 found=%v, 51→115 found=%v)", found65, found115)
	}
	// 档位阶梯：最低 750/rare、最高 2550/primeval。
	if len(table.Grades) < 5 {
		t.Fatalf("real grade rows = %d, want >=5 (%+v)", len(table.Grades), table.Grades)
	}
	top := table.Grades[len(table.Grades)-1]
	if top.Grade != 5 || top.Threshold != 2550 || top.Rarity != "primeval" {
		t.Fatalf("top grade = %+v, want grade 5 / 2550 / primeval", top)
	}
	if table.Grades[0].Threshold != 750 || table.Grades[0].Rarity != "rare" {
		t.Fatalf("first grade = %+v, want 750 / rare", table.Grades[0])
	}

	oathText, e := os.ReadFile(filepath.Join(dir, "11-oathpointinfo.cos.txt"))
	if e != nil {
		t.Skipf("真实源不在（先跑 pvfinspect 导出）: %v", e)
	}
	oath, e := ParseOathPointInfo(string(oathText))
	if e != nil {
		t.Fatalf("parse real oath point: %v", e)
	}
	if len(oath.Rules) < 100 {
		t.Fatalf("real oath point rules = %d, want >=100", len(oath.Rules))
	}
	if oath.MinOathPoint != 1200 {
		t.Fatalf("min oath point = %d, want 1200", oath.MinOathPoint)
	}
	var foundCore, foundCrystal bool
	for _, r := range oath.Rules {
		if r.Template == 100610095 && r.Awakening == 0 && r.PartSetIndex == -1 && r.Value == 355 {
			foundCore = true
		}
		// 晶体同样有分（awakening 0 → 45，1 → 55）—— 用户身上的晶体就是这一类。
		if r.Template == 100401592 && r.Awakening == 0 && r.PartSetIndex == -1 && r.Value == 45 {
			foundCrystal = true
		}
	}
	if !foundCore || !foundCrystal {
		t.Fatalf("real source lost awakening-0 oath rows (100610095→355 found=%v, 100401592→45 found=%v)",
			foundCore, foundCrystal)
	}
}
