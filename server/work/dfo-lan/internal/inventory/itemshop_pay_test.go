package inventory

import "testing"

func bagTotal(b Bag, template uint32) uint32 {
	var n uint32
	for _, it := range b.Items {
		if it.Template == template {
			n += it.Amount
		}
	}
	return n
}

// 实机 2026-09-23（test-jh）：从奥德赛商店买盒子，服务端按写死的金币单价扣了 1 gold，
// 而源里这条商品是 [need material] 10418036 100（100 个银币）——银币一点没动。
// PayMaterials 就是补这条路径：整笔校验、按叠扣、扣空删行。
func TestPayMaterialsChargesTheWholeBill(t *testing.T) {
	const silver, ignore = uint32(10418036), uint32(1)
	bag := Bag{Items: []BagItem{
		{Slot: 65, Template: silver, Amount: 123},
		{Slot: 66, Template: silver, Amount: 7},
		{Slot: 67, Template: ignore, Amount: 5},
	}}
	got, err := bag.PayMaterials([]MaterialCost{{Template: silver, Count: 100}}, 1)
	if err != nil {
		t.Fatal(err)
	}
	if n := bagTotal(got, silver); n != 30 {
		t.Fatalf("银币剩余 %d, 期望 30", n)
	}
	if n := bagTotal(got, ignore); n != 5 {
		t.Fatalf("无关物品被动了: %d", n)
	}
	for _, it := range got.Items {
		if it.Amount == 0 {
			t.Fatalf("扣空后留下了 0 数量的行: %+v", it)
		}
	}
	// 原背包不能被就地修改（事务回滚依赖副本语义）
	if n := bagTotal(bag, silver); n != 130 {
		t.Fatalf("原背包被改动了: %d", n)
	}
	// 数量不足：整笔拒绝
	if _, err := bag.PayMaterials([]MaterialCost{{Template: silver, Count: 200}}, 1); err == nil {
		t.Fatal("银币不足却被接受了")
	}
	// 多件材料里只要一件不足，整笔拒绝
	if _, err := bag.PayMaterials([]MaterialCost{
		{Template: silver, Count: 1},
		{Template: ignore, Count: 99},
	}, 1); err == nil {
		t.Fatal("其中一件不足却被接受了")
	}
}

// 一次购买多件时，账单按数量放大。
func TestPayMaterialsScalesWithCount(t *testing.T) {
	const silver = uint32(10418036)
	bag := Bag{Items: []BagItem{{Slot: 65, Template: silver, Amount: 250}}}
	got, err := bag.PayMaterials([]MaterialCost{{Template: silver, Count: 100}}, 2)
	if err != nil {
		t.Fatal(err)
	}
	if n := bagTotal(got, silver); n != 50 {
		t.Fatalf("买 2 件后银币剩余 %d, 期望 50", n)
	}
}

// 买盒子的完整路径：按材料付账 + 物品入包 + 金币不动。
func TestBuyWithMaterialsLeavesGoldAlone(t *testing.T) {
	const box, silver = uint32(10417798), uint32(10418036)
	rules := BagRules{
		Source:            "test",
		Slots:             map[string][2]uint16{"[throw]": {65, 120}, "[booster selection]": {65, 120}},
		MissingStackLimit: 1000,
	}
	bag := Bag{
		Version: "ordinary-bag-v1",
		Gold:    3585,
		Items:   []BagItem{{Slot: 65, Template: silver, Amount: 123}},
	}
	got, slot, err := bag.BuyWithMaterials(rules, box, 1, []MaterialCost{{Template: silver, Count: 100}}, "[booster selection]")
	if err != nil {
		t.Fatal(err)
	}
	if got.Gold != 3585 {
		t.Fatalf("金币被扣了: %d", got.Gold)
	}
	if n := bagTotal(got, silver); n != 23 {
		t.Fatalf("银币剩余 %d, 期望 23", n)
	}
	if n := bagTotal(got, box); n != 1 {
		t.Fatalf("盒子没进包: %d", n)
	}
	if slot < 65 || slot > 120 {
		t.Fatalf("盒子落在异常槽位 %d", slot)
	}
}
