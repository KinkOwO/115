package oracleresearch

// 组装（2413）用到的两件事：**槽位号 → 模块号** 与 **配方查找**。
//
// 表都是导出字段 ⇒ 这里不碰 PVF，纯结构体测；真源解析另由 Parse 的既有测试覆盖。

import "testing"

func TestModuleAtSlotUsesSourceTable(t *testing.T) {
	// 故意**打乱顺序**并让槽位号 != 模块号-偏移：证明实现读的是表，不是那条巧合的等差。
	c := &Catalog{Slots: []ModuleSlot{
		{Module: 590015769, Slot: 15}, // [shoulder]
		{Module: 590015767, Slot: 13}, // [coat]
		{Module: 590015754, Slot: 0},
		{Module: 590015777, Slot: 23},
	}}
	cases := []struct {
		slot uint32
		want uint32
		ok   bool
	}{
		{0, 590015754, true},
		{13, 590015767, true},
		{15, 590015769, true},
		{23, 590015777, true},
		{5, 0, false},  // 表里没有
		{99, 0, false}, // 越界
	}
	for _, tc := range cases {
		got, ok := c.ModuleAtSlot(tc.slot)
		if ok != tc.ok || got != tc.want {
			t.Errorf("ModuleAtSlot(%d) = (%d,%v), want (%d,%v)", tc.slot, got, ok, tc.want, tc.ok)
		}
	}
}

func TestModuleAtSlotOnNilCatalog(t *testing.T) {
	var c *Catalog
	if _, ok := c.ModuleAtSlot(0); ok {
		t.Fatal("nil 目录不该命中")
	}
	if _, ok := c.RecipeFor(1, 2); ok {
		t.Fatal("nil 目录不该命中")
	}
}

func TestRecipeForMatchesPair(t *testing.T) {
	c := &Catalog{Recipes: []Recipe{
		{SetModule: 590015762, SetCount: 1, PartModule: 590015767, PartCount: 1, ResultItem: 100051282},
		{SetModule: 590015762, SetCount: 1, PartModule: 590015768, PartCount: 1, ResultItem: 100101165},
		{SetModule: 590015764, SetCount: 1, PartModule: 590015769, PartCount: 1, ResultItem: 100151114},
	}}
	// 真机样本 2：套装槽 10 → 590015764、部位槽 15 → 590015769。
	r, ok := c.RecipeFor(590015764, 590015769)
	if !ok || r.ResultItem != 100151114 {
		t.Fatalf("RecipeFor(590015764,590015769) = (%+v,%v), want ResultItem 100151114", r, ok)
	}
	// 顺序不能反（套装 + 部位是有方向的）。
	if _, ok := c.RecipeFor(590015769, 590015764); ok {
		t.Fatal("配方查找不该反向命中")
	}
	if _, ok := c.RecipeFor(590015762, 590015769); ok {
		t.Fatal("不存在的组合不该命中")
	}
}
