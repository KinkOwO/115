package inventory

import (
	"testing"
)

func testShopBagRules() BagRules {
	return BagRules{
		Model:             "reference90-bag-v1",
		Source:            "7ef2db59331f7e5b18b2f250b8b907526bf2c94b17a7312036cf599644d88e80",
		MissingStackLimit: 1000,
		EquipmentSlots:    [2]uint16{9, 64},
		QuickSlots:        [2]uint16{0, 8},
		Slots: map[string][2]uint16{
			"[throw]":    {65, 120},
			"[material]": {121, 176},
		},
	}
}

func TestShopBuySuccessAndStack(t *testing.T) {
	rules := testShopBagRules()
	bag := Bag{
		Version: "ordinary-bag-v1",
		Gold:    1000,
	}

	// 1. First purchase: enters throw range [65, 120] by default
	bag, slot, err := bag.Buy(rules, 1001, 2, 20)
	if err != nil {
		t.Fatalf("unexpected buy error: %v", err)
	}
	if slot != 65 {
		t.Fatalf("expected slot 65, got %d", slot)
	}
	if bag.Gold != 980 {
		t.Fatalf("expected gold 980, got %d", bag.Gold)
	}
	if len(bag.Items) != 1 || bag.Items[0].Amount != 2 {
		t.Fatalf("unexpected items: %+v", bag.Items)
	}

	// 2. Second purchase of same template: stacks into slot 65
	bag, slot2, err := bag.Buy(rules, 1001, 3, 30)
	if err != nil {
		t.Fatalf("unexpected buy stack error: %v", err)
	}
	if slot2 != 65 {
		t.Fatalf("expected slot 65 on stack, got %d", slot2)
	}
	if bag.Gold != 950 {
		t.Fatalf("expected gold 950, got %d", bag.Gold)
	}
	if len(bag.Items) != 1 || bag.Items[0].Amount != 5 {
		t.Fatalf("expected amount 5, got %+v", bag.Items)
	}

	// 3. Purchase with stackableType [material]: enters [121, 176]
	bag, matSlot, err := bag.Buy(rules, 2001, 10, 50, "[material]")
	if err != nil {
		t.Fatalf("unexpected material buy error: %v", err)
	}
	if matSlot != 121 {
		t.Fatalf("expected material slot 121, got %d", matSlot)
	}
	if bag.Gold != 900 {
		t.Fatalf("expected gold 900, got %d", bag.Gold)
	}
	if len(bag.Items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(bag.Items))
	}
}

func TestShopBuyInsufficientGold(t *testing.T) {
	rules := testShopBagRules()
	bag := Bag{
		Version: "ordinary-bag-v1",
		Gold:    10,
	}

	_, _, err := bag.Buy(rules, 1001, 5, 20)
	if err == nil {
		t.Fatal("expected insufficient gold error")
	}
	if bag.Gold != 10 {
		t.Fatalf("gold should not be modified on error: %d", bag.Gold)
	}
}

func TestShopSellStackableAndEquipment(t *testing.T) {
	rules := testShopBagRules()
	bag := Bag{
		Version: "ordinary-bag-v1",
		Gold:    100,
		Items: []BagItem{
			{Slot: 65, Template: 1001, Amount: 2},
		},
		Equipment: []BagEquipment{
			{Slot: 15, Template: 5001, Durability: 100},
		},
	}

	// 1. Sell 1 stackable from slot 65
	bag, tmpl, gold, err := bag.Sell(rules, 0, 65, 5)
	if err != nil {
		t.Fatalf("unexpected sell stackable error: %v", err)
	}
	if tmpl != 1001 || gold != 5 || bag.Gold != 105 {
		t.Fatalf("sell result mismatch: tmpl=%d, gold=%d, totalGold=%d", tmpl, gold, bag.Gold)
	}
	if len(bag.Items) != 1 || bag.Items[0].Amount != 1 {
		t.Fatalf("expected 1 item left with amount 1: %+v", bag.Items)
	}

	// 2. Sell the remaining stackable from slot 65 -> slot is emptied
	bag, tmpl, gold, err = bag.Sell(rules, 0, 65, 5)
	if err != nil {
		t.Fatalf("unexpected sell last stackable error: %v", err)
	}
	if tmpl != 1001 || gold != 5 || bag.Gold != 110 {
		t.Fatalf("sell result mismatch: tmpl=%d, gold=%d, totalGold=%d", tmpl, gold, bag.Gold)
	}
	if len(bag.Items) != 0 {
		t.Fatalf("expected items to be empty, got %+v", bag.Items)
	}

	// 3. Sell equipment from slot 15
	bag, tmpl, gold, err = bag.Sell(rules, 0, 15, 25)
	if err != nil {
		t.Fatalf("unexpected sell equipment error: %v", err)
	}
	if tmpl != 5001 || gold != 25 || bag.Gold != 135 {
		t.Fatalf("sell equipment mismatch: tmpl=%d, gold=%d, totalGold=%d", tmpl, gold, bag.Gold)
	}
	if len(bag.Equipment) != 0 {
		t.Fatalf("expected equipment to be empty, got %+v", bag.Equipment)
	}
}

func TestShopSellWornRefusedAndSlotOverlap(t *testing.T) {
	rules := testShopBagRules()

	// 1. Worn item alone at slot 15 -> refused
	bag := Bag{
		Version: "ordinary-bag-v1",
		Gold:    50,
		Worn: []BagEquipment{
			{Slot: 15, Template: 6001, Durability: 100},
		},
	}
	_, _, _, err := bag.Sell(rules, 0, 15, 10)
	if err == nil {
		t.Fatal("expected error selling worn equipment")
	}

	// 2. Overlap resolution: equipment in Equipment at slot 15, and gear in Worn at slot 15.
	// Selling slot 15 must sell the inventory equipment, leaving Worn untouched.
	bagWithBoth := Bag{
		Version: "ordinary-bag-v1",
		Gold:    50,
		Equipment: []BagEquipment{
			{Slot: 15, Template: 7001, Durability: 80},
		},
		Worn: []BagEquipment{
			{Slot: 15, Template: 6001, Durability: 100},
		},
	}
	bagWithBoth, tmpl, gold, err := bagWithBoth.Sell(rules, 0, 15, 10)
	if err != nil {
		t.Fatalf("unexpected sell error with overlap: %v", err)
	}
	if tmpl != 7001 || gold != 10 {
		t.Fatalf("expected to sell equipment 7001, got tmpl=%d, gold=%d", tmpl, gold)
	}
	if len(bagWithBoth.Equipment) != 0 {
		t.Fatalf("expected equipment to be cleared, got %+v", bagWithBoth.Equipment)
	}
	if len(bagWithBoth.Worn) != 1 || bagWithBoth.Worn[0].Template != 6001 {
		t.Fatalf("worn equipment should remain untouched, got %+v", bagWithBoth.Worn)
	}
}
