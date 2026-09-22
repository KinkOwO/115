package inventory

import (
	"encoding/json"
	"testing"

	"dfolan/internal/catalog"
)

// 2026-09-22 玩家报告：admin 发不出「非怪物掉落」类物品
// （`bin/admin.exe -item 10418036x1000` → `item 10418036: invalid equipment award`）。
//
// 奥德赛银币 10418036 / 金币 10418035 按设计不在 loot.next25.json（那是怪物掉落
// 目录），只存在于 items.index.json；网关用 SupplementStackables 把它们补进目录，
// cmd/admin 从来没做这一步 ⇒ Awarder.Grant 里 Catalog.Items[id].Kind 读到零值 ""，
// 于是落进装备分支、AddEquipment 失败。
//
// 这条用例走真实 Awarder：补挂后必须落到消耗品槽 65..120，1000 一叠（stack_limit
// 缺省取 missing_stack_limit=1000），再发 1000 另起一叠而不是被"库存已满"拒掉。
func TestOdysseyCoinGrantReachesConsumableSlots(t *testing.T) {
	const (
		coin uint32 = 10418036 // stackable/10418001/10418036.stk  [unlimited waste]
		gold uint32 = 10418035
	)
	c, err := catalog.LoadLoot("../../configs/loot.next25.json")
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := c.Items[coin]; ok {
		t.Fatal("fixture drift: the Odyssey coin is now a monster drop")
	}
	if err := c.SupplementStackables("../../configs/items.index.json"); err != nil {
		t.Fatal(err)
	}
	rules, err := LoadBagRules("../../configs/inventory.next29.json")
	if err != nil {
		t.Fatal(err)
	}
	gear, err := LoadEquipmentCatalog("../../configs/equipment.current35.json", c.Source.Checksum)
	if err != nil {
		t.Fatal(err)
	}
	a := &Awarder{Catalog: c, Rules: rules, Equipment: gear}

	raw := json.RawMessage(`{}`)
	slots := make([]uint16, 0, 4)
	for _, id := range []uint32{coin, gold} {
		if a.Catalog.Items[id].Kind != "stackable" {
			t.Fatalf("template %d not reachable after supplementing: %+v", id, a.Catalog.Items[id])
		}
		for pass := 0; pass < 2; pass++ {
			out, receipt, err := a.Grant(raw, id, 1000)
			if err != nil {
				t.Fatalf("template %d pass %d: %v", id, pass, err)
			}
			if len(receipt.Slots) != 1 || receipt.Slots[0] < 65 || receipt.Slots[0] > 120 {
				t.Fatalf("template %d pass %d landed in %v, want one consumable slot 65..120", id, pass, receipt.Slots)
			}
			slots = append(slots, receipt.Slots[0])
			raw = out
		}
	}
	if slots[0] == slots[1] || slots[2] == slots[3] {
		t.Fatalf("the second 1000 went into the already full stack: %v", slots)
	}

	b, err := ReadBag(raw)
	if err != nil {
		t.Fatal(err)
	}
	counts := map[uint32]uint32{}
	for _, it := range b.Items {
		counts[it.Template] += it.Amount
	}
	if counts[coin] != 2000 || counts[gold] != 2000 {
		t.Fatalf("bag holds %v, want 2000 of each coin", counts)
	}
}
