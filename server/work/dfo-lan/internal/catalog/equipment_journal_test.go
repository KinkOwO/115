package catalog

import (
	"os"
	"path/filepath"
	"testing"
)

// 合成用例：把源里用到的每一种段形状都覆盖一遍（不依赖 PVF，也不依赖导入产物）。
const journalSample = `[disjoint guide]
 [setpoint] 3000
 [title] 101036479
[/disjoint guide]
[max equipment count] 99
[max equipment count by equipment type]
` + "`[oath]`, 2, 1\n" + "`[oath]`, 8, 1\n" + `[/max equipment count by equipment type]
[max awakening count] 9
[part set index]
 16201
 16202
[/part set index]
[common primer item index]
 100401592
[/common primer item index]
[weapon group]
 [info]
  [index] 1
  [list]
   101001149
   101001150
  [/list]
 [/info]
 [info]
  [index] 2
  [list]
   101011315
  [/list]
 [/info]
[/weapon group]
[new peculiar group]
 [group]
  [index] 16223
  [list]
   1 2 3
  [/list]
  [name] <20::equipment_journal_38>
  [set mark] 0
  [button type] ` + "`create`" + `
 [/group]
 [group]
  [index] 16226
  [list]
   27 28
  [/list]
  [name] <20::equipment_journal_41>
  [set mark] 3
  [button type] ` + "`equipment transform`" + `
 [/group]
[/new peculiar group]
[oath group]
 307
 308
[/oath group]
`

func TestParseEquipmentJournalRulesSample(t *testing.T) {
	r, e := ParseEquipmentJournalRules(journalSample)
	if e != nil {
		t.Fatalf("parse: %v", e)
	}
	if r.Maximum != 99 {
		t.Fatalf("maximum = %d, want 99", r.Maximum)
	}
	if r.MaxAwakening != 9 {
		t.Fatalf("max awakening = %d, want 9", r.MaxAwakening)
	}
	if len(r.MaximumByType) != 2 {
		t.Fatalf("maximum_by_type = %d entries, want 2", len(r.MaximumByType))
	}
	if l := r.MaximumByType[0]; l.Kind != "[oath]" || l.Rarity != 2 || l.Maximum != 1 {
		t.Fatalf("first limit = %+v, want {[oath] 2 1}", l)
	}
	if got := r.PartSetIndex; len(got) != 2 || got[0] != 16201 || got[1] != 16202 {
		t.Fatalf("part set index = %v", got)
	}
	if got := r.CommonPrimerItem; len(got) != 1 || got[0] != 100401592 {
		t.Fatalf("common primer = %v", got)
	}
	if got := r.OathGroup; len(got) != 2 || got[0] != 307 || got[1] != 308 {
		t.Fatalf("oath group = %v", got)
	}
	if len(r.WeaponGroups) != 2 {
		t.Fatalf("weapon groups = %d, want 2", len(r.WeaponGroups))
	}
	if g := r.WeaponGroups[0]; g.Index != 1 || len(g.Members) != 2 || g.Members[1] != 101001150 {
		t.Fatalf("weapon group[0] = %+v", g)
	}
	if len(r.Categories) != 2 {
		t.Fatalf("categories = %d, want 2", len(r.Categories))
	}
	c, ok := r.Category(0)
	if !ok || c.Index != 16223 || c.Button != "create" || len(c.Members) != 3 || c.Members[2] != 3 {
		t.Fatalf("category 0 = %+v ok=%v", c, ok)
	}
	if _, ok := r.Category(2); ok {
		t.Fatalf("category 2 should be absent in this sample")
	}
	if r.DisjointGuide["setpoint"] != 3000 {
		t.Fatalf("disjoint guide setpoint = %d, want 3000", r.DisjointGuide["setpoint"])
	}
}

func TestParseEquipmentJournalRulesRejectsBroken(t *testing.T) {
	// 容器判定是"该 name 在这个文本里出现过 [/name]"。所以"未闭合"要构造一个
	// 同名容器（先出现一次收尾）才会被识别成容器。
	if _, e := ParseEquipmentJournalRules("[max equipment count] 99\n[list]\n[/list]\n[list]\n"); e == nil {
		t.Fatalf("unclosed container should fail")
	}
	if _, e := ParseEquipmentJournalRules("[max awakening count] 9\n"); e == nil {
		t.Fatalf("missing [max equipment count] should fail")
	}
	// 收尾名配不上
	if _, e := ParseEquipmentJournalRules("[max equipment count] 99\n[list]\n[/weapon group]\n"); e == nil {
		t.Fatalf("mismatched close should fail")
	}
	// 段头不是数字
	if _, e := ParseEquipmentJournalRules("[max equipment count] many\n"); e == nil {
		t.Fatalf("non-numeric [max equipment count] should fail")
	}
}

// 收录资格与上限：这是"分解时要不要登记装备库"的判据。
func TestEquipmentJournalLimitFor(t *testing.T) {
	r, e := ParseEquipmentJournalRules(journalSample)
	if e != nil {
		t.Fatalf("parse: %v", e)
	}
	cases := []struct {
		kind   string
		rarity uint32
		want   uint32
		ok     bool
	}{
		{"", 2, 99, true},        // 普通、稀有度在集合内
		{"", 8, 99, true},        // 普通、最高档
		{"", 5, 0, false},        // 稀有度不在集合内（5 = 史诗？源里没有）
		{"", 0, 0, false},        // 白装
		{"[oath]", 2, 1, true},   // 誓约、源里声明了 2
		{"[oath]", 8, 1, true},   // 誓约、源里声明了 8
		{"[oath]", 4, 99, true},  // 源里只声明了 2/8 ⇒ 回落普通上限（不是拒绝）
		{"[other]", 2, 99, true}, // 源里没为这个类型声明条目 ⇒ 回落普通上限
	}
	for _, c := range cases {
		got, ok := r.LimitFor(c.kind, c.rarity)
		if ok != c.ok || got != c.want {
			t.Fatalf("LimitFor(%q,%d) = (%d,%v), want (%d,%v)", c.kind, c.rarity, got, ok, c.want, c.ok)
		}
	}
}

// 真实源用例：数据来自 runtime/pvf-journal/ 的导出（gitignore，缺失就跳过）。
// 它把"我们量过的结构"钉住 —— 结构一变就红，而不是等实机才发现。
func TestParseEquipmentJournalRulesRealSource(t *testing.T) {
	path := filepath.Join("..", "..", "runtime", "pvf-journal", "00-equipmentsetjournal.cos.txt")
	b, e := os.ReadFile(path)
	if e != nil {
		t.Skipf("真实源不在（先跑 pvfinspect 导出）: %v", e)
	}
	r, e := ParseEquipmentJournalRules(string(b))
	if e != nil {
		t.Fatalf("parse real source: %v", e)
	}
	if r.Maximum != 99 {
		t.Fatalf("maximum = %d, want 99", r.Maximum)
	}
	if r.MaxAwakening != 9 {
		t.Fatalf("max awakening = %d, want 9", r.MaxAwakening)
	}
	// [max equipment count by equipment type]：誓约按 rarity 2/3/4/6/8 各 1
	want := map[uint32]uint32{2: 1, 3: 1, 4: 1, 6: 1, 8: 1}
	if len(r.MaximumByType) != len(want) {
		t.Fatalf("maximum_by_type = %d entries, want %d", len(r.MaximumByType), len(want))
	}
	for _, l := range r.MaximumByType {
		if l.Kind != "[oath]" {
			t.Fatalf("limit kind = %q, want [oath]", l.Kind)
		}
		if want[l.Rarity] != l.Maximum {
			t.Fatalf("oath rarity %d limit = %d, want %d", l.Rarity, l.Maximum, want[l.Rarity])
		}
	}
	if len(r.PartSetIndex) != 13 || r.PartSetIndex[0] != 16201 || r.PartSetIndex[12] != 16213 {
		t.Fatalf("part set index = %v", r.PartSetIndex)
	}
	if len(r.CommonPrimerItem) != 1 || r.CommonPrimerItem[0] != 100401592 {
		t.Fatalf("common primer = %v", r.CommonPrimerItem)
	}

	// 收藏类别：0..4，各绑一个组 ID 与 button type（UI 只有 4 个标签页，类别 2 无入口）
	if len(r.Categories) != 5 {
		t.Fatalf("categories = %d, want 5", len(r.Categories))
	}
	wantCat := map[uint32]struct {
		index  uint32
		button string
	}{
		0: {16223, "create"},
		1: {16224, "amalgamation transform"},
		2: {16225, "amalgamation transform"},
		3: {16226, "equipment transform"},
		4: {16227, "primer transform"},
	}
	for mark, exp := range wantCat {
		c, ok := r.Category(mark)
		if !ok {
			t.Fatalf("category %d absent", mark)
		}
		if c.Index != exp.index || c.Button != exp.button {
			t.Fatalf("category %d = {index:%d button:%q}, want {index:%d button:%q}",
				mark, c.Index, c.Button, exp.index, exp.button)
		}
	}

	// 两张大分组表：规模钉住（决定收录判据时不使用，仅防结构漂移）
	if len(r.WeaponGroups) != 210 {
		t.Fatalf("weapon groups = %d, want 210", len(r.WeaponGroups))
	}
	if len(r.PeculiarGroups) != 74 {
		t.Fatalf("peculiar groups = %d, want 74", len(r.PeculiarGroups))
	}
	if len(r.OathGroup) != 4 || r.OathGroup[0] != 307 || r.OathGroup[3] != 310 {
		t.Fatalf("oath group = %v", r.OathGroup)
	}
	if len(r.PrimerGroup) != 13 || r.PrimerGroup[0] != 241 || r.PrimerGroup[12] != 253 {
		t.Fatalf("primer group = %v", r.PrimerGroup)
	}

	// 判据自检：普通 99、誓约 1、非 {2,3,4,6,8} 不登记
	if n, ok := r.LimitFor("", 4); !ok || n != 99 {
		t.Fatalf("LimitFor(plain,4) = (%d,%v)", n, ok)
	}
	if n, ok := r.LimitFor("[oath]", 6); !ok || n != 1 {
		t.Fatalf("LimitFor(oath,6) = (%d,%v)", n, ok)
	}
	if _, ok := r.LimitFor("", 5); ok {
		t.Fatalf("rarity 5 must not be registrable")
	}
}

// 钉死已导出的装备库规则表：每个值都是 equipmentsetjournal.cos 的读字段，
// 是「能否入库 / 上限多少 / 收藏有几类」的唯一真源。数值来自运行时源 PVF。
func TestEquipmentJournalGeneratedCatalog(t *testing.T) {
	const source = "7ef2db59331f7e5b18b2f250b8b907526bf2c94b17a7312036cf599644d88e80"
	r, e := LoadEquipmentJournalRules("../../configs/equipment-journal.generated.json", source)
	if e != nil {
		t.Fatal(e)
	}
	if r.Path != EquipmentJournalPath || len(r.SHA256) != 64 {
		t.Fatalf("provenance %q %q", r.Path, r.SHA256)
	}
	if r.Source.Checksum != source {
		t.Fatalf("source = %s, want %s", r.Source.Checksum, source)
	}
	if r.Bytes != 122980 {
		t.Fatalf("bytes = %d, want 122980", r.Bytes)
	}
	// 代次收紧：调用方期望的代次对不上必须拒绝。
	if _, e := LoadEquipmentJournalRules("../../configs/equipment-journal.generated.json", "deadbeef"); e == nil {
		t.Fatalf("generation mismatch must be rejected")
	}
	if r.Maximum != 99 || r.MaxAwakening != 9 {
		t.Fatalf("maximum/awakening = %d/%d, want 99/9", r.Maximum, r.MaxAwakening)
	}
	if len(r.MaximumByType) != 5 {
		t.Fatalf("limits = %d, want 5", len(r.MaximumByType))
	}
	for _, l := range r.MaximumByType {
		if l.Kind != "[oath]" || l.Maximum != 1 {
			t.Fatalf("limit %+v, want [oath] -> 1", l)
		}
	}
	if len(r.PartSetIndex) != 13 || r.PartSetIndex[0] != 16201 || r.PartSetIndex[12] != 16213 {
		t.Fatalf("part set index = %v", r.PartSetIndex)
	}
	// 微光星蕴石：规格 0026 点名「必须走登记路径、不能误进普通分解」的那件。
	if len(r.CommonPrimerItem) != 1 || r.CommonPrimerItem[0] != 100401592 {
		t.Fatalf("common primer = %v", r.CommonPrimerItem)
	}
	// 五个收藏类别：UI 只摆 4 个标签页（实测覆盖 0/1/3/4），类别 2 有配置无入口。
	if len(r.Categories) != 5 {
		t.Fatalf("categories = %d, want 5", len(r.Categories))
	}
	want := map[uint32]struct {
		index  uint32
		button string
		member int
	}{
		0: {16223, "create", 11},
		1: {16224, "amalgamation transform", 15},
		2: {16225, "amalgamation transform", 36},
		3: {16226, "equipment transform", 11},
		4: {16227, "primer transform", 1},
	}
	for mark, exp := range want {
		c, ok := r.Category(mark)
		if !ok {
			t.Fatalf("category %d absent", mark)
		}
		if c.Index != exp.index || c.Button != exp.button || len(c.Members) != exp.member {
			t.Fatalf("category %d = {index:%d button:%q members:%d}, want {index:%d button:%q members:%d}",
				mark, c.Index, c.Button, len(c.Members), exp.index, exp.button, exp.member)
		}
	}
	if len(r.WeaponGroups) != 210 || len(r.PeculiarGroups) != 74 {
		t.Fatalf("groups = %d/%d, want 210/74", len(r.WeaponGroups), len(r.PeculiarGroups))
	}
	if len(r.OathGroup) != 4 || r.OathGroup[0] != 307 || r.OathGroup[3] != 310 {
		t.Fatalf("oath group = %v", r.OathGroup)
	}
	if len(r.PrimerGroup) != 13 || r.PrimerGroup[0] != 241 || r.PrimerGroup[12] != 253 {
		t.Fatalf("primer group = %v", r.PrimerGroup)
	}
	// 收录判据：普通 99 / 誓约 1 / 非 {2,3,4,6,8} 一律不登记。
	if n, ok := r.LimitFor("", 2); !ok || n != 99 {
		t.Fatalf("LimitFor(plain,2) = (%d,%v)", n, ok)
	}
	if n, ok := r.LimitFor("[oath]", 8); !ok || n != 1 {
		t.Fatalf("LimitFor(oath,8) = (%d,%v)", n, ok)
	}
	for _, rarity := range []uint32{0, 1, 5, 7, 9} {
		if _, ok := r.LimitFor("", rarity); ok {
			t.Fatalf("rarity %d must not be registrable", rarity)
		}
	}
}
