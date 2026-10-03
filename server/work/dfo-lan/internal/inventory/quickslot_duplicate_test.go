package inventory

import (
	"testing"

	"dfolan/internal/catalog"
	"dfolan/internal/catalog/pvf"
)

// 2026-10-03 next79 待机区闪退：10418028（[waste] 药水）同时在快捷栏 slot 3
// （×12）与消耗品段 slot 75（×40）各一行，客户端 findItemSlot 陷入 "multiple
// slot issues" 死循环（032305 会话 79374 次告警，待机区场景永不完成）。历史
// 成因：用户把 12 个拖上 belt 后，odyssey 发放 ×30 走 Add 只认类型段，段内另
// 起 75 行。两条回归线锁住修复：清扫器把 belt 重复并回段内；发放路径认得
// belt 上的堆，不再 spawn 第二行。
func TestSweepMergesQuickSlotDuplicateIntoRange(t *testing.T) {
	rules, err := LoadBagRules("../../configs/inventory.current37.json")
	if err != nil {
		t.Fatal(err)
	}
	const potion = 10418028
	cat := catalog.LootCatalog{
		Source: pvf.ArchiveSnapshot{Checksum: "7ef2db59331f7e5b18b2f250b8b907526bf2c94b17a7312036cf599644d88e80"},
		Items:  map[uint32]catalog.LootItem{potion: {ID: potion, Kind: "stackable", StackableType: "[waste]"}},
	}
	if !rules.Quick(3) {
		t.Fatal("fixture drift: slot 3 is no longer a quick slot")
	}
	bag := Bag{Version: "ordinary-bag-v1", Items: []BagItem{
		{Slot: 3, Template: potion, Amount: 12},
		{Slot: 75, Template: potion, Amount: 40},
	}}
	fixed, moved, err := SweepStackSlots(bag, cat, rules)
	if err != nil || !moved {
		t.Fatal(moved, err)
	}
	if len(fixed.Items) != 1 {
		t.Fatalf("want a single merged row, got %+v", fixed.Items)
	}
	if fixed.Items[0].Slot != 75 || fixed.Items[0].Amount != 52 {
		t.Fatalf("merge mismatch: %+v", fixed.Items[0])
	}
	// 清扫必须幂等：再跑一遍不再动。
	if _, moved, err := SweepStackSlots(fixed, cat, rules); err != nil || moved {
		t.Fatal(moved, err)
	}
	// belt 上唯一的堆（段内无同模板行）必须原样保留。
	solo := Bag{Version: "ordinary-bag-v1", Items: []BagItem{{Slot: 3, Template: potion, Amount: 12}}}
	kept, moved, err := SweepStackSlots(solo, cat, rules)
	if err != nil || moved || len(kept.Items) != 1 || kept.Items[0].Slot != 3 {
		t.Fatal(moved, err, kept.Items)
	}
}

func TestGrantMergesIntoQuickSlotStack(t *testing.T) {
	rules, err := LoadBagRules("../../configs/inventory.current37.json")
	if err != nil {
		t.Fatal(err)
	}
	const potion = 10418028
	cat := catalog.LootCatalog{
		Source: pvf.ArchiveSnapshot{Checksum: "7ef2db59331f7e5b18b2f250b8b907526bf2c94b17a7312036cf599644d88e80"},
		Items:  map[uint32]catalog.LootItem{potion: {ID: potion, Kind: "stackable", StackableType: "[waste]"}},
	}
	// 堆整个在 belt（3）上：Add 必须并进它，而不是在消耗品段另起一叠。
	belt := Bag{Version: "ordinary-bag-v1", Items: []BagItem{{Slot: 3, Template: potion, Amount: 12}}}
	out, slot, err := belt.Add(cat, rules, potion, 30)
	if err != nil {
		t.Fatal(err)
	}
	if slot != 3 || len(out.Items) != 1 || out.Items[0].Amount != 42 {
		t.Fatalf("grant must merge into the belt stack: slot=%d items=%+v", slot, out.Items)
	}
	// 商店路径（addStackable）同规则。
	out2, slot, err := out.Buy(rules, potion, 10, 0, "[waste]")
	if err != nil {
		t.Fatal(err)
	}
	if slot != 3 || len(out2.Items) != 1 || out2.Items[0].Amount != 52 {
		t.Fatalf("buy must merge into the belt stack: slot=%d items=%+v", slot, out2.Items)
	}
	// 邮件附件路径（AddMailItem）同规则。
	mail := Bag{Version: "ordinary-bag-v1", Items: []BagItem{{Slot: 3, Template: potion, Amount: 12}}}
	out3, err := mail.AddMailItem(cat, rules, nil, MailItem{Stack: &BagItem{Template: potion, Amount: 30}})
	if err != nil {
		t.Fatal(err)
	}
	if len(out3.Items) != 1 || out3.Items[0].Slot != 3 || out3.Items[0].Amount != 42 {
		t.Fatalf("mail attachment must merge into the belt stack: %+v", out3.Items)
	}
}
