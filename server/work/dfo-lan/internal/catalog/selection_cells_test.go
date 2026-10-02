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
