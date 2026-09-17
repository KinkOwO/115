package inventory

import (
	"dfolan/internal/catalog"
	"dfolan/internal/catalog/pvf"
	"dfolan/internal/game/protocol"
	"testing"
)

func TestQuickSlotConsumablesMoveRequest(t *testing.T) {
	rules, err := LoadBagRules("../../configs/inventory.current37.json")
	if err != nil {
		t.Fatalf("LoadBagRules failed: %v", err)
	}

	checksum := "7ef2db59331f7e5b18b2f250b8b907526bf2c94b17a7312036cf599644d88e80"
	cat := catalog.LootCatalog{
		Source: pvf.ArchiveSnapshot{Checksum: checksum},
		Items: map[uint32]catalog.LootItem{
			1106:    {ID: 1106, Kind: "stackable", StackableType: "[waste]", StackLimit: 1000},
			1112:    {ID: 1112, Kind: "stackable", StackableType: "[waste]", StackLimit: 1000},
			2660671: {ID: 2660671, Kind: "stackable", StackableType: "[waste]", StackLimit: 1000},
			15:      {ID: 15, Kind: "stackable", StackableType: "[material]", StackLimit: 1000},
		},
	}

	// Bag with HP potion at 67, MP potion at 66, Remy's Touch at 68, Material at 121
	b := Bag{
		Gold: 1000,
		Items: []BagItem{
			{Slot: 66, Template: 1112, Amount: 50},
			{Slot: 67, Template: 1106, Amount: 30},
			{Slot: 68, Template: 2660671, Amount: 100},
			{Slot: 121, Template: 15, Amount: 200},
		},
	}

	// Case 1: Drag Remy's Touch (68) onto quick slot 5 (live client sends Count=0, SourceSlot=5, DestSlot=68, DestItem=2660671)
	req1 := protocol.ItemMoveRequest{
		SourceList:      0,
		SourceSlot:      5,
		SourceItem:      0,
		Count:           0,
		DestinationList: 0,
		DestinationSlot: 68,
		DestinationItem: 2660671,
		Selection:       0xffffffff,
	}
	b1, err := b.MoveStackRequest(cat, rules, req1)
	if err != nil {
		t.Fatalf("Move Remy's Touch to quick slot 5 failed: %v", err)
	}
	var found5 *BagItem
	for _, it := range b1.Items {
		if it.Slot == 5 {
			x := it
			found5 = &x
		}
		if it.Slot == 68 {
			t.Fatalf("slot 68 was not emptied")
		}
	}
	if found5 == nil || found5.Template != 2660671 || found5.Amount != 100 {
		t.Fatalf("slot 5 expected Remy's Touch, got %+v", found5)
	}

	// Case 2: Drag HP potion (67) onto quick slot 4
	req2 := protocol.ItemMoveRequest{
		SourceList:      0,
		SourceSlot:      4,
		SourceItem:      0,
		Count:           0,
		DestinationList: 0,
		DestinationSlot: 67,
		DestinationItem: 1106,
		Selection:       0xffffffff,
	}
	b2, err := b1.MoveStackRequest(cat, rules, req2)
	if err != nil {
		t.Fatalf("Move HP potion to quick slot 4 failed: %v", err)
	}

	// Case 3: Drag MP potion (66) onto quick slot 3
	req3 := protocol.ItemMoveRequest{
		SourceList:      0,
		SourceSlot:      3,
		SourceItem:      0,
		Count:           0,
		DestinationList: 0,
		DestinationSlot: 66,
		DestinationItem: 1112,
		Selection:       0xffffffff,
	}
	b3, err := b2.MoveStackRequest(cat, rules, req3)
	if err != nil {
		t.Fatalf("Move MP potion to quick slot 3 failed: %v", err)
	}

	// Case 4: Swap two quick slots (swap slot 3 and slot 4)
	req4 := protocol.ItemMoveRequest{
		SourceList:      0,
		SourceSlot:      3,
		SourceItem:      1112,
		Count:           50,
		DestinationList: 0,
		DestinationSlot: 4,
		DestinationItem: 1106,
		Selection:       0xffffffff,
	}
	b4, err := b3.MoveStackRequest(cat, rules, req4)
	if err != nil {
		t.Fatalf("Swap quick slots 3 and 4 failed: %v", err)
	}
	for _, it := range b4.Items {
		if it.Slot == 3 && it.Template != 1106 {
			t.Fatalf("slot 3 expected 1106 after swap, got %d", it.Template)
		}
		if it.Slot == 4 && it.Template != 1112 {
			t.Fatalf("slot 4 expected 1112 after swap, got %d", it.Template)
		}
	}

	// Case 5: Drag from quick slot 5 back to consumable bag (slot 68)
	req5 := protocol.ItemMoveRequest{
		SourceList:      0,
		SourceSlot:      68,
		SourceItem:      0,
		Count:           0,
		DestinationList: 0,
		DestinationSlot: 5,
		DestinationItem: 2660671,
		Selection:       0xffffffff,
	}
	b5, err := b4.MoveStackRequest(cat, rules, req5)
	if err != nil {
		t.Fatalf("Move from quick slot back to bag failed: %v", err)
	}
	for _, it := range b5.Items {
		if it.Slot == 5 {
			t.Fatalf("quick slot 5 was not emptied")
		}
		if it.Slot == 68 && (it.Template != 2660671 || it.Amount != 100) {
			t.Fatalf("slot 68 expected 2660671, got %+v", it)
		}
	}

	// Case 6: Reject material (slot 121) dragged to quick slot 3
	req6 := protocol.ItemMoveRequest{
		SourceList:      0,
		SourceSlot:      3,
		SourceItem:      1106,
		Count:           200,
		DestinationList: 0,
		DestinationSlot: 121,
		DestinationItem: 15,
		Selection:       0xffffffff,
	}
	if _, err := b5.MoveStackRequest(cat, rules, req6); err == nil {
		t.Fatalf("expected error when moving material to quick slot, but succeeded")
	}

	// Case 7: Fallback test - even if item is NOT in cat.Items, but in consumable bag
	emptyCat := catalog.LootCatalog{Source: pvf.ArchiveSnapshot{Checksum: checksum}}
	bFallback, err := b.MoveStackRequest(emptyCat, rules, req1)
	if err != nil {
		t.Fatalf("fallback move for consumable in bag failed: %v", err)
	}
	var foundFallback *BagItem
	for _, it := range bFallback.Items {
		if it.Slot == 5 {
			x := it
			foundFallback = &x
		}
	}
	if foundFallback == nil || foundFallback.Template != 2660671 {
		t.Fatalf("fallback expected item at slot 5")
	}

	// Case 8: Consume item from quick slot
	bConsumed, remaining, err := b1.Consume(cat, 5, 2660671)
	if err != nil {
		t.Fatalf("consume from quick slot failed: %v", err)
	}
	if remaining != 99 {
		t.Fatalf("expected 99 remaining, got %d", remaining)
	}
	for _, it := range bConsumed.Items {
		if it.Slot == 5 && it.Amount != 99 {
			t.Fatalf("expected slot 5 amount 99, got %d", it.Amount)
		}
	}
}
