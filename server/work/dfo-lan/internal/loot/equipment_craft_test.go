package loot

import (
	"dfolan/internal/catalog"
	"dfolan/internal/inventory"
	"encoding/json"
	"testing"
)

// 「装备生成」的成本挑选：**玩家在窗口里点的是哪一支，就扣哪一支**。
//
// ★ 分仓：三档登记证 `10361512`~`10361515` 是**账号共享材料（灵魂仓库 / 容器 35）**，
// 必须从 `AccountMaterials` 扣、而不是从背包 —— 实机 `have 0` 之谜就是没做这件事。
// ★ 不回退：`payOption` 来自请求头 `[13]`。实机 2026-09-29 13:52 那笔（`[13]=2`，
// 玩家选的是**巡礼之印**）被旧实现"挑第一支付得起的"错扣成了 35,000 金币。
func TestPickCraftCost(t *testing.T) {
	// 组 2 的真实两支（源自 configs/equipment-create-cost.generated.json）：
	//   1 = 登记证 10361514×1 + 金币 35000；2 = 登记证 10361514×1 + 巡礼之印 10401346×7
	group := catalog.CreateCostGroup{
		Index: 2,
		Items: []uint32{100391056},
		Costs: []catalog.CreateCostOption{
			{Number: 1, Pairs: []catalog.CreateCostItem{
				{Template: 10361514, Amount: 1},
				{Template: 0, Amount: 35000},
			}},
			{Number: 2, Pairs: []catalog.CreateCostItem{
				{Template: 10361514, Amount: 1},
				{Template: 10401346, Amount: 7},
			}},
		},
	}
	empty := inventory.NewAccountMaterials()
	acct := empty
	var e error
	if acct, _, e = acct.Add(10361514, 5); e != nil {
		t.Fatal(e)
	}

	// 1) 选 cost 1（登记证 + 金币）：登记证在**账号仓库**、金币也够 ⇒ 材料标记 FromAccount。
	rich := inventory.Bag{Gold: 40000}
	opt, gold, bagMats, acctMats, e := pickCraftCost(rich, acct, group, 1)
	if e != nil {
		t.Fatal(e)
	}
	if opt != 1 || gold != 35000 {
		t.Fatalf("rich: option=%d gold=%d, want 1/35000", opt, gold)
	}
	if len(bagMats) != 0 {
		t.Fatalf("rich: bag materials should be empty, got %+v", bagMats)
	}
	if len(acctMats) != 1 || acctMats[0].Template != 10361514 || acctMats[0].Count != 1 {
		t.Fatalf("rich: account materials = %+v", acctMats)
	}

	// 2) ★ 选 cost 2（登记证 + 巡礼之印）：必须扣背包里的 `10401346`×7，**不碰金币**。
	seals := inventory.Bag{Gold: 40000, Items: []inventory.BagItem{
		{Slot: 127, Template: 10401346, Amount: 120},
	}}
	opt, gold, bagMats, acctMats, e = pickCraftCost(seals, acct, group, 2)
	if e != nil {
		t.Fatal(e)
	}
	if opt != 2 || gold != 0 {
		t.Fatalf("seals: option=%d gold=%d, want 2/0", opt, gold)
	}
	if len(bagMats) != 1 || bagMats[0].Template != 10401346 || bagMats[0].Count != 7 {
		t.Fatalf("seals: bag materials = %+v", bagMats)
	}
	if len(acctMats) != 1 || acctMats[0].Template != 10361514 {
		t.Fatalf("seals: account materials = %+v", acctMats)
	}

	// 3) ★ 选了 cost 2、金币管够但巡礼之印不够 ⇒ **必须拒绝，不许回退 cost 1**。
	poorSeals := inventory.Bag{Gold: 999999, Items: []inventory.BagItem{
		{Slot: 127, Template: 10401346, Amount: 3},
	}}
	if _, _, _, _, e = pickCraftCost(poorSeals, acct, group, 2); e == nil {
		t.Fatalf("cost 2 with only 3 seals must be refused, not silently downgraded to cost 1")
	}

	// 4) ★ 选了 cost 1、巡礼之印管够但金币不够 ⇒ 也必须拒绝（不许自动换 cost 2）。
	//    这正是实机 13:52 的错扣形态反过来：不许用"另一支付得起"来顶替玩家的选择。
	poorGold := inventory.Bag{Gold: 100, Items: []inventory.BagItem{
		{Slot: 127, Template: 10401346, Amount: 120},
	}}
	if _, _, _, _, e = pickCraftCost(poorGold, acct, group, 1); e == nil {
		t.Fatalf("cost 1 with 100 gold must be refused, not silently downgraded to cost 2")
	}

	// 5) ★ 登记证**只在背包里**（旧实机的情形）：必须拒绝 —— 背包里那份其实属于账号仓库，
	//    客户端不会拿背包那份去付（`SweepAccountMaterials` 迟早把它搬走）。
	bagOnly := inventory.Bag{Gold: 40000, Items: []inventory.BagItem{
		{Slot: 124, Template: 10361514, Amount: 5},
	}}
	if _, _, _, _, e := pickCraftCost(bagOnly, empty, group, 1); e == nil {
		t.Fatalf("bag-only ticket must be refused (regression: 实机 have 0)")
	}

	// 6) 付法序号不存在 ⇒ 拒绝，并点名可用的序号（不许猜一支扣下去）。
	if _, _, _, _, e = pickCraftCost(rich, acct, group, 3); e == nil {
		t.Fatalf("unknown pay option must be refused")
	} else if !contains(e.Error(), "no cost option 3") {
		t.Fatalf("refusal should name the requested option, got %v", e)
	}
}

// 金币行必须被识别成金币（模板 0），别当成物品 0 去扣。
func TestCreateCostGoldRow(t *testing.T) {
	if !(catalog.CreateCostItem{Template: 0, Amount: 1}).Gold() {
		t.Fatalf("template 0 must be gold")
	}
	if (catalog.CreateCostItem{Template: 10361513, Amount: 1}).Gold() {
		t.Fatalf("material must not be gold")
	}
}

// 三档登记证必须被认成账号共享材料 —— 这是分仓的判据本身。
func TestCraftTicketsAreAccountMaterials(t *testing.T) {
	for _, tpl := range []uint32{10361512, 10361513, 10361514, 10361515, 10361516} {
		if _, ok := inventory.AccountMaterialSlot(tpl); !ok {
			t.Fatalf("template %d must be an account-shared material", tpl)
		}
	}
	// 反过来：10401346 不是（它留在背包里扣）。
	if _, ok := inventory.AccountMaterialSlot(10401346); ok {
		t.Fatalf("10401346 must stay in the ordinary bag")
	}
}

func contains(hay, needle string) bool {
	return indexOf(hay, needle) >= 0
}

func indexOf(hay, needle string) int {
	for i := 0; i+len(needle) <= len(hay); i++ {
		if hay[i:i+len(needle)] == needle {
			return i
		}
	}
	return -1
}

// ★ 幂等键必须在**每次成功之后都变** —— 否则同物二次生成会被当成重放静默吞掉
// （实机 2026-09-29 14:21：两笔全被吞，日志却是 DONE + 全零回执）。
func TestCraftEventKeyChangesPerAttempt(t *testing.T) {
	before := json.RawMessage(`{"bag":{"gold":100}}`)
	after := json.RawMessage(`{"bag":{"gold":65}}`)

	if craftEventKey(100391056, 25, 2, before) == craftEventKey(100391056, 25, 2, after) {
		t.Fatalf("key must differ when the character state differs, else the 2nd craft is swallowed")
	}
	// 同状态同请求 ⇒ 同键：重复帧仍然是幂等的（不会扣两次）。
	if craftEventKey(100391056, 25, 2, before) != craftEventKey(100391056, 25, 2, before) {
		t.Fatalf("key must be stable for the same state, else a duplicate frame double-charges")
	}
	// 不同物/槽/档必须分开。
	if craftEventKey(100391056, 25, 2, before) == craftEventKey(100354178, 23, 2, before) {
		t.Fatalf("key must include template and slot")
	}
}
