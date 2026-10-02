package cashshop

import (
	"dfolan/internal/inventory"
	"encoding/json"
	"testing"
)

// 实机 2026-09-26：礼包就地展开开出称号（Adventurer's Will，
// equipment/character/common/title/500330719.equ），deliverAmount 的兜底把
// 它当 "[etc]" 发进了消耗品区（Use），客户端拖回装备区被拒。装备必须进
// b.Equipment（Equip 页），耐久走注入的 Catalog.Reward 规则。
func TestShopDeliveryEquipmentGoesToEquipTab(t *testing.T) {
	p := nativePilot(t, false)
	p.SupplementItemKinds(map[uint32]ItemInfo{
		500330719: {ID: 500330719, Kind: "equipment", Path: "equipment/character/common/title/500330719.equ"},
	})
	p.SetEquipmentDurability(func(uint32) (uint16, error) { return 45, nil })

	out, e := p.deliverAmount(json.RawMessage(`{}`), 500330719, 1)
	if e != nil {
		t.Fatal(e)
	}
	bag, e := inventory.ReadBag(out)
	if e != nil {
		t.Fatal(e)
	}
	if len(bag.Equipment) != 1 {
		t.Fatalf("equipment rows = %d, want 1", len(bag.Equipment))
	}
	row := bag.Equipment[0]
	if row.Template != 500330719 {
		t.Fatalf("template = %d", row.Template)
	}
	if row.Slot < 9 || row.Slot > 64 {
		t.Fatalf("slot = %d, want inside [9,64]", row.Slot)
	}
	if row.Durability != 45 {
		t.Fatalf("durability = %d, want 45", row.Durability)
	}
	if len(bag.Items) != 0 {
		t.Fatalf("title must not land in the use tab, items = %+v", bag.Items)
	}
}

// creature 的 .equ 在索引里同样是 kind=equipment，必须按源路径改投宠物栏。
func TestShopDeliveryCreatureRoutedByPath(t *testing.T) {
	p := nativePilot(t, false)
	p.SupplementItemKinds(map[uint32]ItemInfo{
		63006: {ID: 63006, Kind: "equipment", Path: "equipment/creature/egg_faras.equ"},
	})
	out, e := p.deliverAmount(json.RawMessage(`{}`), 63006, 1)
	if e != nil {
		t.Fatal(e)
	}
	bag, e := inventory.ReadBag(out)
	if e != nil {
		t.Fatal(e)
	}
	if len(bag.Special[7]) != 1 || bag.Special[7][0].Template != 63006 {
		t.Fatalf("creature rows = %+v", bag.Special[7])
	}
	if len(bag.Equipment) != 0 {
		t.Fatalf("creature must not land in the equipment bag, rows = %+v", bag.Equipment)
	}
}

// 未补全分类的未知模板保持原兜底行为（[etc] → 消耗品区），不做行为漂移。
func TestShopDeliveryUnknownStillEtc(t *testing.T) {
	p := nativePilot(t, false)
	out, e := p.deliverAmount(json.RawMessage(`{}`), 999999999, 1)
	if e != nil {
		t.Fatal(e)
	}
	bag, e := inventory.ReadBag(out)
	if e != nil {
		t.Fatal(e)
	}
	if len(bag.Items) != 1 || bag.Items[0].Slot < 65 || bag.Items[0].Slot > 120 {
		t.Fatalf("unknown template must keep the [etc] fallback, items = %+v", bag.Items)
	}
	if len(bag.Equipment) != 0 {
		t.Fatalf("unknown template must not create equipment rows, rows = %+v", bag.Equipment)
	}
}
