package inventory

import (
	"testing"

	"dfolan/internal/catalog"
)

// 2026-09-23 玩家报告：分解奥德赛装备只出了灵魂/小晶块，**没给奥德赛硬币**。
//
// 根因不在"算错了"，而在**整段规则没做**：客户端的 `etc/disjoint.etc` 里有一段
// `[event result]` —— 按**装备模板 id** 直接查的额外产出平表（全表 769 条三元组
// (模板, 产出物, 数量)），其中奥德赛金币 10418035 共 159 条、银币 10418036 共 114 条。
// 服务端 `CalculateDisjointRewards` 只实现了「无色小晶块 / 附加产物 / 灵魂」三类，
// 这段表**一条都没实现**，而且分解产物一律塞进材料栏。
//
// 注意别把"没实现"误读成"硬币从没被用过"：这两个模板在工程内的既有用途是
// **商城付款**（internal/inventory/shop.go、itemshopimport）与**副本掉落**
// （internal/loot/odyssey_currency.go），与分解路径完全不同，所以本文件补的是
// 分解侧，而不是在已有实现上改行为。
//
// 下面这条走真实的 Bag.Disjoint：分解 100051394（客户端表里写的是给 30 个金币），
// 必须同时满足三件事 —— ① 产出 10418035 × 30；② 落**消耗品栏 65..120**
// （它是 `[unlimited waste]`，客户端钉在这一栏）；③ 常规产物（无色小晶块）仍在，
// 硬币是**追加**而不是替换。
func TestBagDisjointAwardsOdysseyCoinsIntoConsumableSlots(t *testing.T) {
	const (
		jacket uint32 = 100051394 // etc/disjoint.etc [event result]: 100051394 10418035 30
		gold   uint32 = 10418035
	)
	c, err := catalog.LoadLoot("../../configs/loot.next25.json")
	if err != nil {
		t.Fatal(err)
	}
	// 硬币不在怪物掉落目录里，靠 items.index.json 补进目录后才能解析出它的
	// [stackable type]（`[unlimited waste]`）—— 与网关启动时的做法一致。
	if err := c.SupplementStackables("../../configs/items.index.json"); err != nil {
		t.Fatal(err)
	}
	rules, err := LoadBagRules("../../configs/inventory.next29.json")
	if err != nil {
		t.Fatal(err)
	}

	b := Bag{Equipment: []BagEquipment{{Slot: 11, Template: jacket, Durability: 60}}}
	updated, res, err := b.Disjoint(c, rules, nil, []uint16{11}, 0xFFFF)
	if err != nil {
		t.Fatalf("Disjoint failed: %v", err)
	}
	if len(updated.Equipment) != 0 {
		t.Fatalf("expected the jacket to be removed, got %+v", updated.Equipment)
	}

	var coins, cube uint32
	var coinSlot uint16
	for _, rw := range res.Rewards {
		switch rw.Template {
		case gold:
			coins += rw.Count
			coinSlot = rw.Slot
		case ClearCubeFragmentID:
			cube += rw.Count
		}
	}
	if coins != 30 {
		t.Fatalf("expected 30 Odyssey gold coins (client [event result] says 30), got %d in %+v", coins, res.Rewards)
	}
	if coinSlot < 65 || coinSlot > 120 {
		t.Fatalf("the coin is [unlimited waste]: it must land in the consumable range 65..120, got slot %d", coinSlot)
	}
	if cube == 0 {
		t.Fatalf("the ordinary clear-cube reward must still be awarded alongside the coin, got %+v", res.Rewards)
	}
}

// 表里没有的模板一分不给 —— 防止"给所有装备都发硬币"这类越界。
func TestBagDisjointCoinTableDoesNotLeakToOtherTemplates(t *testing.T) {
	c, err := catalog.LoadLoot("../../configs/loot.next25.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := c.SupplementStackables("../../configs/items.index.json"); err != nil {
		t.Fatal(err)
	}
	rules, err := LoadBagRules("../../configs/inventory.next29.json")
	if err != nil {
		t.Fatal(err)
	}
	b := Bag{Equipment: []BagEquipment{{Slot: 11, Template: 27054, Durability: 35}}}
	_, res, err := b.Disjoint(c, rules, nil, []uint16{11}, 0xFFFF)
	if err != nil {
		t.Fatalf("Disjoint failed: %v", err)
	}
	for _, rw := range res.Rewards {
		if rw.Template == 10418035 || rw.Template == 10418036 {
			t.Fatalf("template 27054 is not in [event result]; it must not award coins: %+v", res.Rewards)
		}
	}
}

// 现有行为不能变：`[material]` 类产物（小晶块 / 元素结晶 / 灵魂）仍落材料栏 121..176。
func TestBagDisjointMaterialRewardsKeepTheirRange(t *testing.T) {
	c, err := catalog.LoadLoot("../../configs/loot.next25.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := c.SupplementStackables("../../configs/items.index.json"); err != nil {
		t.Fatal(err)
	}
	rules, err := LoadBagRules("../../configs/inventory.next29.json")
	if err != nil {
		t.Fatal(err)
	}
	b := Bag{Equipment: []BagEquipment{{Slot: 11, Template: 100051394, Durability: 60}}}
	_, res, err := b.Disjoint(c, rules, nil, []uint16{11}, 0xFFFF)
	if err != nil {
		t.Fatalf("Disjoint failed: %v", err)
	}
	// 逐个看非硬币的产物：既可能是账号材料（先入材料栏，随后被 sweep 走），
	// 也可能是普通材料，改动后都应仍在 121..176。
	for _, rw := range res.Rewards {
		if rw.Template == 10418035 || rw.Template == 10418036 {
			continue
		}
		if rw.Slot < 121 || rw.Slot > 176 {
			t.Fatalf("material reward %d landed in slot %d, want the material range 121..176",
				rw.Template, rw.Slot)
		}
	}
}

// 表本身的形状检查：数量、币种、取值都必须与 etc/disjoint.etc 对得上。
// 这几条数字是**从客户端原文数出来的**，改动本文件时若不符就是转写错了。
func TestDisjointOdysseyCoinTableShape(t *testing.T) {
	gold, silver := 0, 0
	for tpl, rw := range disjointOdysseyCoins {
		switch rw.Template {
		case 10418035:
			gold++
		case 10418036:
			silver++
		default:
			t.Fatalf("template %d maps to unexpected reward %d", tpl, rw.Template)
		}
		if rw.Count != 1 && rw.Count != 15 && rw.Count != 30 {
			t.Fatalf("template %d has unexpected count %d (client only uses 1/15/30)", tpl, rw.Count)
		}
	}
	if len(disjointOdysseyCoins) != 273 || gold != 159 || silver != 114 {
		t.Fatalf("table drifted from etc/disjoint.etc: %d rows (%d gold + %d silver), want 273 (159 + 114)",
			len(disjointOdysseyCoins), gold, silver)
	}
	// 玩家报的那件：客户端原样给 30 个金币。
	if rw := disjointOdysseyCoins[100051394]; rw.Template != 10418035 || rw.Count != 30 {
		t.Fatalf("100051394 should award 30 x 10418035, got %+v", rw)
	}
	// 银币那侧同样抽查一条（客户端：100051397 → 10418036 × 30）。
	if rw := disjointOdysseyCoins[100051397]; rw.Template != 10418036 || rw.Count != 30 {
		t.Fatalf("100051397 should award 30 x 10418036, got %+v", rw)
	}
}
