package inventory

import (
	"dfolan/internal/catalog"
	"dfolan/internal/catalog/pvf"
	"testing"
)

// 银币/金币（`[unlimited waste]`）在源里不写 `[stackable limit]`，靠类型名声明"无限"。
// 服务端过去把"没写上限"兜成 missing_stack_limit（1000），于是手里的 23 个银币与 GM 发
// 的 1000 个分成两叠（实机 2026-09-23 观察）。
func TestUnlimitedStackablesIgnoreMissingStackLimit(t *testing.T) {
	const coin, capped = uint32(10418036), uint32(30001)
	rules := BagRules{
		Source:            "test",
		Slots:             map[string][2]uint16{"[throw]": {65, 120}},
		MissingStackLimit: 1000,
	}
	cat := catalog.LootCatalog{
		Source: pvf.ArchiveSnapshot{Checksum: "test"},
		Items: map[uint32]catalog.LootItem{
			coin:   {ID: coin, Kind: "stackable", StackableType: "[unlimited waste]"},
			capped: {ID: capped, Kind: "stackable", StackableType: "[waste]", StackLimit: 10},
		},
	}
	bag := Bag{
		Version: "ordinary-bag-v1",
		Items:   []BagItem{{Slot: 65, Template: coin, Amount: 23}},
	}
	next, slot, err := bag.Add(cat, rules, coin, 1000)
	if err != nil {
		t.Fatal(err)
	}
	if slot != 65 {
		t.Fatalf("银币没有并进原叠：落到 slot %d", slot)
	}
	if n := bagTotal(next, coin); n != 1023 {
		t.Fatalf("银币合计 %d，期望 1023", n)
	}
	if len(next.Items) != 1 {
		t.Fatalf("仍然分成多叠：%+v", next.Items)
	}
	// 源里写了显式上限的类型不受影响。
	if _, _, err := bag.Add(cat, rules, capped, 11); err == nil {
		t.Fatal("显式上限 10 的物品被放开到 11")
	}
	if _, _, err := bag.Add(cat, rules, capped, 10); err != nil {
		t.Fatalf("上限内的数量被拒: %v", err)
	}
}

func TestMissingExplicitStackLimitUsesSignedClientMaximum(t *testing.T) {
	rules := BagRules{Source: "test", MissingStackLimit: 1000}
	cat := catalog.LootCatalog{Source: pvf.ArchiveSnapshot{Checksum: "test"}, Items: map[uint32]catalog.LootItem{
		42: {ID: 42, Kind: "stackable", StackableType: "[waste]"},
	}}
	if got := StackLimitForTemplate(cat, rules, 42); got != 2147483647 {
		t.Fatalf("known template limit = %d", got)
	}
	if got := StackLimitForTemplate(cat, rules, 99); got != 1000 {
		t.Fatalf("unknown template limit = %d", got)
	}
	bag := Bag{Version: "ordinary-bag-v1", Items: []BagItem{{Slot: 65, Template: 42, Amount: 1000}}}
	bag, _, err := bag.Add(cat, rules, 42, 1500)
	if err != nil || len(bag.Items) != 1 || bag.Items[0].Amount != 2500 {
		t.Fatalf("bag stack = %+v, %v", bag, err)
	}
}
