package inventory

import (
	"dfolan/internal/catalog"
	"dfolan/internal/game/protocol"
	"testing"
)

// 复现实机 2026-09-29 13:29:52 的 bagmove：
//
//	{"to": 124, "from": 127, "template": 10401346}
//
// 当时的包（本地按 10:33 发放 + 13:24 补发后的样子重建）：
//
//	124 = 10361513 x30, 125 = 10361514 x30, 126 = 10361515 x30, 127 = 10401346 x120
//
// 这条用例只回答一件事：**一次两格移动会不会吞掉第三件东西**。
// 若这个用例通过，就说明"三档登记证消失"不是 MoveStackRequest 干的，得另找。
func TestMoveStackOverwriteDoesNotEatNeighbours(t *testing.T) {
	bag := Bag{
		Gold: 261017,
		Items: []BagItem{
			{Slot: 121, Template: 10362432, Amount: 482},
			{Slot: 122, Template: 3166, Amount: 14},
			{Slot: 124, Template: 10361513, Amount: 30},
			{Slot: 125, Template: 10361514, Amount: 30},
			{Slot: 126, Template: 10361515, Amount: 30},
			{Slot: 127, Template: 10401346, Amount: 120},
		},
	}
	rules := BagRules{
		Slots: map[string][2]uint16{
			"[material]": {121, 176},
			"[waste]":    {65, 120},
		},
		MissingStackLimit: 9999,
	}
	// Source 留空 = 与 rules.Source 一致，跳过代次校验；这里只测移动本身。
	var cat catalog.LootCatalog

	// 客户端约定：正在拖的东西写在 Destination*，目标写在 Source*。
	// 这里复现"把 127 的 10401346 往 124 挪"，且客户端认为 124 是空的（SourceItem = 0）。
	req := protocol.ItemMoveRequest{
		SourceSlot:      124,
		SourceItem:      0, // 客户端以为 124 空着
		DestinationSlot: 127,
		DestinationItem: 10401346,
		Count:           0,
		Selection:       0xffffffff,
	}
	// ★ 结论：**必须拒绝**。客户端说"124 空着"，但服务端在 124 有东西 ——
	// 这一层 `matches(a, r.SourceItem)` 守卫正是防止"用陈旧视图覆盖掉别的物品"。
	// （实机的三档登记证消失**不是**移动吃掉的，而是被 `SweepAccountMaterials`
	//   搬进了账号共享仓库 —— 见 `accountMaterialSlotByTemplate` 里的 376..378。）
	if _, e := bag.MoveStackRequest(cat, rules, req); e == nil {
		t.Fatalf("must refuse: client thinks slot 124 is empty but the server holds 10361513 there")
	}
	// 守卫也必须挡住"用陈旧身份覆盖"：目标格在服务端不是客户端说的那件时同样拒绝。
	stale := req
	stale.DestinationItem = 999
	if _, e := bag.MoveStackRequest(cat, rules, stale); e == nil {
		t.Fatalf("must refuse a stale destination identity")
	}
}

// 反向：如果客户端**知道** 124 有东西（SourceItem = 10361513），那就是一次交换，两件都该在。
func TestMoveStackSwapKeepsBoth(t *testing.T) {
	bag := Bag{Items: []BagItem{
		{Slot: 124, Template: 10361513, Amount: 30},
		{Slot: 127, Template: 10401346, Amount: 120},
	}}
	rules := BagRules{
		Slots:             map[string][2]uint16{"[material]": {121, 176}},
		MissingStackLimit: 9999,
	}
	// Source 留空 = 与 rules.Source 一致，跳过代次校验；这里只测移动本身。
	var cat catalog.LootCatalog
	req := protocol.ItemMoveRequest{
		SourceSlot:      124,
		SourceItem:      10361513,
		DestinationSlot: 127,
		DestinationItem: 10401346,
		Selection:       0xffffffff,
	}
	out, e := bag.MoveStackRequest(cat, rules, req)
	if e != nil {
		t.Fatalf("swap refused: %v", e)
	}
	got := map[uint32]uint32{}
	for _, it := range out.Items {
		got[it.Template] += it.Amount
	}
	if got[10361513] != 30 || got[10401346] != 120 {
		t.Fatalf("swap lost an item: %v", got)
	}
}
