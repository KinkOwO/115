package catalog

import (
	"dfolan/internal/catalog/pvf"
	"testing"
)

func selTag(text string) pvf.Token { return pvf.Token{Type: 3, Text: text} }
func selNum(v int32) pvf.Token     { return pvf.Token{Type: 0, Value: v} }

func itemTemplates(cat SelectionCategory) []uint32 {
	out := make([]uint32, 0, len(cat.Items))
	for _, it := range cat.Items {
		out = append(out, it.Template)
	}
	return out
}

// 10344930/10358468：源脚本没有写 [/booster select category]，脚本结尾就是块的
// 边界；解析器必须接受，而不是把整个盒子当未建模。
// ★ 2026-10-11：「Royal Pass 每日登录礼包」590722589 的**真实脚本**。分类块里写
// 的是 `[stackable]`（不是 [equipment]），三选一与件数全在这里：
//
//	[booster select category] 0 0
//	[stackable] 10327284 90 10327285 45 10362432 50 [/stackable]
//
// 不建模这一段 ⇒ Resolve 的件数表为空 ⇒ 发放回落到礼盒自身数量（1），
// 实机表现就是「选了 x45 只到 1 个」。
func TestParseSelectionStackablePairs(t *testing.T) {
	cells := []pvf.Token{
		selTag("[stackable type]"), selTag("[booster selection]"),
		selTag("[booster category num]"), selNum(0),
		selTag("[booster selection num]"), selNum(1),
		selTag("[booster select category]"), selNum(0), selNum(0),
		selTag("[stackable]"),
		selNum(10327284), selNum(90), selNum(10327285), selNum(45), selNum(10362432), selNum(50),
		selTag("[/stackable]"),
		selTag("[/booster select category]"),
	}
	cats, fixed := ParseSelectionCells(cells)
	if fixed || len(cats) != 1 {
		t.Fatalf("fixed=%v cats=%+v", fixed, cats)
	}
	cat := cats[0]
	if cat.Category != [2]byte{0, 0} {
		t.Fatalf("category = %v", cat.Category)
	}
	want := []SelectionItem{
		{Template: 10327284, Count: 90},
		{Template: 10327285, Count: 45},
		{Template: 10362432, Count: 50},
	}
	if len(cat.Items) != len(want) {
		t.Fatalf("items = %+v", cat.Items)
	}
	for i, w := range want {
		if cat.Items[i] != w {
			t.Fatalf("item %d = %+v，应为 %+v", i, cat.Items[i], w)
		}
	}
	if len(cat.Sections) != 1 || cat.Sections[0] != "[stackable]" {
		t.Fatalf("sections = %v", cat.Sections)
	}
	// 建模之后必须算「可比较」，否则重复类别会从去重退回拒绝。
	if hasUnmodelledSection(cat) {
		t.Fatal("已建模的 [stackable] 段被判成未建模")
	}
}

// 源里 33 个 `[stackable]` 块是 5 个数字一条的记录（10404663 的
// `10404771 1 2 115 108 …`，共 5 条 = 25 个数字）。形状未定 ⇒ 整块不建模，
// 并且要在 Sections 里留下「未解析」的记号，保持旧的未建模语义。
func TestParseSelectionStackableRecordShapeStaysUnmodelled(t *testing.T) {
	cells := []pvf.Token{
		selTag("[booster select category]"), selNum(0), selNum(0),
		selTag("[stackable]"),
		selNum(10404771), selNum(1), selNum(2), selNum(115), selNum(108),
		selNum(10404772), selNum(1), selNum(2), selNum(116), selNum(109),
		selNum(10404773), selNum(1), selNum(2), selNum(117), selNum(110),
		selNum(10404774), selNum(1), selNum(2), selNum(118), selNum(111),
		selNum(10404775), selNum(1), selNum(2), selNum(119), selNum(112),
		selTag("[/stackable]"),
		selTag("[/booster select category]"),
	}
	cats, _ := ParseSelectionCells(cells)
	if len(cats) != 1 {
		t.Fatalf("cats = %+v", cats)
	}
	if len(cats[0].Items) != 0 {
		t.Fatalf("5 数字一条的块不该被当成 (模板,数量) 对：%+v", cats[0].Items)
	}
	if len(cats[0].Sections) != 1 || cats[0].Sections[0] != selectionSectionStackableUnparsed {
		t.Fatalf("sections = %v", cats[0].Sections)
	}
	if !hasUnmodelledSection(cats[0]) {
		t.Fatal("未解析的 [stackable] 必须仍算未建模")
	}
}

func TestParseSelectionImplicitCloseAtEndOfScript(t *testing.T) {
	cells := []pvf.Token{
		selTag("[stackable type]"), selTag("[booster selection]"),
		selTag("[booster select category]"), selNum(0), selNum(0),
		selTag("[equipment]"), selNum(101590700), selNum(1), selTag("[/equipment]"),
	}
	cats, fixed := ParseSelectionCells(cells)
	if fixed || len(cats) != 1 || cats[0].Category != [2]byte{0, 0} {
		t.Fatalf("implicit end close: fixed=%v cats=%+v", fixed, cats)
	}
	if got := itemTemplates(cats[0]); len(got) != 1 || got[0] != 101590700 {
		t.Fatalf("items = %v", got)
	}
}

// 10345171 一类：块缺尾标签，下一个 [booster select category] 就是边界；不能把
// 下一个块的头消费掉，也不能把它的小类合并进当前块。
func TestParseSelectionImplicitCloseKeepsNextCategory(t *testing.T) {
	cells := []pvf.Token{
		selTag("[booster select category]"), selNum(0), selNum(0),
		selTag("[equipment]"), selNum(11), selNum(1), selTag("[/equipment]"),
		selTag("[booster select category]"), selNum(1), selNum(0),
		selTag("[equipment]"), selNum(22), selNum(1), selTag("[/equipment]"), selTag("[/booster select category]"),
	}
	cats, _ := ParseSelectionCells(cells)
	if len(cats) != 2 {
		t.Fatalf("got %d categories: %+v", len(cats), cats)
	}
	if cats[0].Category != [2]byte{0, 0} || cats[1].Category != [2]byte{1, 0} {
		t.Fatalf("pairs = %v %v", cats[0].Category, cats[1].Category)
	}
	if got := itemTemplates(cats[0]); len(got) != 1 || got[0] != 11 {
		t.Fatalf("first items = %v", got)
	}
	if got := itemTemplates(cats[1]); len(got) != 1 || got[0] != 22 {
		t.Fatalf("second items = %v", got)
	}
}

// 50006227：源把 [/equipment] 写成了 [equipment]；装备段必须在类别边界收口，
// 绝不能把后续小类的 (job,growtype) 和条目吞成装备条目。
func TestParseSelectionEquipmentStopsAtCategoryBoundary(t *testing.T) {
	cells := []pvf.Token{
		selTag("[booster select category]"), selNum(7), selNum(2),
		selTag("[equipment]"), selNum(100), selNum(1), selNum(200), selNum(1),
		selTag("[equipment]"), selTag("[/booster select category]"),
		selTag("[booster select category]"), selNum(8), selNum(0),
		selTag("[equipment]"), selNum(400), selNum(1), selNum(500), selNum(1), selTag("[/equipment]"), selTag("[/booster select category]"),
	}
	cats, _ := ParseSelectionCells(cells)
	if len(cats) != 2 {
		t.Fatalf("got %d categories: %+v", len(cats), cats)
	}
	if got := itemTemplates(cats[0]); len(got) != 2 || got[0] != 100 || got[1] != 200 {
		t.Fatalf("bounded items = %v", got)
	}
	if got := itemTemplates(cats[1]); len(got) != 2 || got[0] != 400 || got[1] != 500 {
		t.Fatalf("next category items = %v", got)
	}
}
