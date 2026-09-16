package inventory

import (
	"dfolan/internal/catalog"
	"testing"
)

func testBagCatalog(t *testing.T) (catalog.LootCatalog, BagRules) {
	t.Helper()
	c, err := catalog.LoadLoot("../../configs/loot.next25.json")
	if err != nil {
		t.Fatalf("LoadLoot failed: %v", err)
	}
	rules, err := LoadBagRules("../../configs/inventory.current37.json")
	if err != nil {
		t.Fatalf("LoadBagRules failed: %v", err)
	}
	return c, rules
}

func TestBagDisjointSuccess(t *testing.T) {
	c, rules := testBagCatalog(t)
	b := Bag{
		Equipment: []BagEquipment{
			{Slot: 11, Template: 27054, Durability: 35},
		},
		Items: []BagItem{
			{Slot: 121, Template: ClearCubeFragmentID, Amount: 10},
		},
	}

	updated, res, err := b.Disjoint(c, rules, []uint16{11}, 0xFFFF)
	if err != nil {
		t.Fatalf("Disjoint failed: %v", err)
	}
	if len(updated.Equipment) != 0 {
		t.Fatalf("expected equipment slot 11 to be removed, got len=%d", len(updated.Equipment))
	}
	if len(updated.Items) != 1 {
		t.Fatalf("expected 1 item stack, got %d", len(updated.Items))
	}
	if updated.Items[0].Slot != 121 || updated.Items[0].Amount != 30 {
		t.Fatalf("expected slot 121 to have 30 cubes, got slot=%d amount=%d", updated.Items[0].Slot, updated.Items[0].Amount)
	}
	if len(res.DeletedSlots) != 1 || res.DeletedSlots[0] != 11 {
		t.Fatalf("expected deleted slot 11, got %v", res.DeletedSlots)
	}
	if len(res.Rewards) != 1 || res.Rewards[0].Slot != 121 || res.Rewards[0].Count != 20 {
		t.Fatalf("unexpected rewards: %+v", res.Rewards)
	}
}

func TestBagDisjointNewSlot(t *testing.T) {
	c, rules := testBagCatalog(t)
	b := Bag{
		Equipment: []BagEquipment{
			{Slot: 11, Template: 27054, Durability: 35},
		},
	}

	updated, res, err := b.Disjoint(c, rules, []uint16{11}, 0xFFFF)
	if err != nil {
		t.Fatalf("Disjoint failed: %v", err)
	}
	if len(updated.Equipment) != 0 {
		t.Fatalf("expected equipment slot 11 to be removed, got len=%d", len(updated.Equipment))
	}
	if len(updated.Items) != 1 {
		t.Fatalf("expected 1 new item stack, got %d", len(updated.Items))
	}
	if updated.Items[0].Slot != 121 || updated.Items[0].Template != ClearCubeFragmentID || updated.Items[0].Amount != 20 {
		t.Fatalf("expected slot 121 to have 20 cubes, got %+v", updated.Items[0])
	}
	if len(res.Rewards) != 1 || res.Rewards[0].Slot != 121 || res.Rewards[0].Count != 20 {
		t.Fatalf("unexpected rewards: %+v", res.Rewards)
	}
}

func TestBagDisjointWornRejected(t *testing.T) {
	c, rules := testBagCatalog(t)
	b := Bag{
		Worn: []BagEquipment{
			{Slot: 15, Template: 10001, Durability: 30},
		},
		Equipment: []BagEquipment{
			{Slot: 11, Template: 27054, Durability: 35},
		},
	}

	_, _, err := b.Disjoint(c, rules, []uint16{15}, 0xFFFF)
	if err == nil {
		t.Fatal("expected error when disjointing worn equipment, got nil")
	}
}

func TestBagDisjointMissingItemRejected(t *testing.T) {
	c, rules := testBagCatalog(t)
	b := Bag{
		Equipment: []BagEquipment{
			{Slot: 11, Template: 27054, Durability: 35},
		},
	}

	_, _, err := b.Disjoint(c, rules, []uint16{99}, 0xFFFF)
	if err == nil {
		t.Fatal("expected error for nonexistent slot 99, got nil")
	}
}
