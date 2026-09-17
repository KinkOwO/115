package inventory

import (
	"dfolan/internal/game/protocol"
	"dfolan/internal/storage"
	"testing"
)

func TestVaultMoveBagToVault(t *testing.T) {
	rules := BagRules{
		EquipmentSlots:    [2]uint16{9, 64},
		MissingStackLimit: 1000,
	}
	bag := Bag{
		Version: "ordinary-bag-v1",
		Equipment: []BagEquipment{
			{Slot: 10, Template: 100261068, Durability: 40},
		},
		Items: []BagItem{
			{Slot: 121, Template: 3037, Amount: 500},
		},
	}
	vault := Vault{Slots: 8}

	rEquip := protocol.ItemMoveRequest{
		SourceList: 0, SourceSlot: 10, SourceItem: 100261068, Count: 1,
		DestinationList: 2, DestinationSlot: 0,
	}
	newBag, newVault, count, err := MoveVaultItem(bag, vault, rules, rEquip)
	if err != nil {
		t.Fatalf("unexpected equip move error: %v", err)
	}
	if count != 1 || len(newBag.Equipment) != 0 {
		t.Fatalf("expected equipment removed from bag")
	}
	if len(newVault.Items) != 1 || newVault.Items[0].Template != 100261068 || !newVault.Items[0].IsEquip {
		t.Fatalf("expected equipment in vault slot 0: %+v", newVault.Items)
	}

	rStack := protocol.ItemMoveRequest{
		SourceList: 0, SourceSlot: 121, SourceItem: 3037, Count: 100,
		DestinationList: 2, DestinationSlot: 1,
	}
	newBag2, newVault2, count2, err := MoveVaultItem(newBag, newVault, rules, rStack)
	if err != nil {
		t.Fatalf("unexpected stack move error: %v", err)
	}
	if count2 != 100 || len(newBag2.Items) != 1 || newBag2.Items[0].Amount != 400 {
		t.Fatalf("expected bag remaining 400, got %+v", newBag2.Items)
	}
	if len(newVault2.Items) != 2 || newVault2.ItemAt(1) == nil || newVault2.ItemAt(1).Amount != 100 {
		t.Fatalf("expected vault item 100, got %+v", newVault2.Items)
	}

	rStackMerge := protocol.ItemMoveRequest{
		SourceList: 0, SourceSlot: 121, SourceItem: 3037, Count: 200,
		DestinationList: 2, DestinationSlot: 1,
	}
	newBag3, newVault3, count3, err := MoveVaultItem(newBag2, newVault2, rules, rStackMerge)
	if err != nil {
		t.Fatalf("unexpected stack merge error: %v", err)
	}
	if count3 != 200 || newBag3.Items[0].Amount != 200 || newVault3.ItemAt(1).Amount != 300 {
		t.Fatalf("merge mismatch: bag=%+v vault=%+v", newBag3.Items, newVault3.Items)
	}

	rOOB := protocol.ItemMoveRequest{
		SourceList: 0, SourceSlot: 121, DestinationList: 2, DestinationSlot: 8,
	}
	if _, _, _, err := MoveVaultItem(newBag3, newVault3, rules, rOOB); err == nil {
		t.Fatal("expected out of bounds vault slot error")
	}
}

func TestVaultMoveVaultToBag(t *testing.T) {
	rules := BagRules{
		EquipmentSlots:    [2]uint16{9, 64},
		MissingStackLimit: 1000,
	}
	bag := Bag{Version: "ordinary-bag-v1"}
	vault := Vault{
		Slots: 8,
		Items: []VaultItem{
			{Slot: 0, Template: 100261068, Durability: 40, IsEquip: true},
			{Slot: 1, Template: 3037, Amount: 300, IsEquip: false},
		},
	}

	rWithdrawEquip := protocol.ItemMoveRequest{
		SourceList: 2, SourceSlot: 0, SourceItem: 100261068,
		DestinationList: 0, DestinationSlot: 9,
	}
	newBag, newVault, count, err := MoveVaultItem(bag, vault, rules, rWithdrawEquip)
	if err != nil {
		t.Fatalf("unexpected withdraw equip error: %v", err)
	}
	if count != 1 || len(newBag.Equipment) != 1 || newBag.Equipment[0].Slot != 9 {
		t.Fatalf("equip not placed in bag slot 9: %+v", newBag.Equipment)
	}
	if newVault.ItemAt(0) != nil {
		t.Fatalf("vault slot 0 should be empty")
	}

	rWithdrawStack := protocol.ItemMoveRequest{
		SourceList: 2, SourceSlot: 1, SourceItem: 3037, Count: 150,
		DestinationList: 0, DestinationSlot: 121,
	}
	newBag2, newVault2, count2, err := MoveVaultItem(newBag, newVault, rules, rWithdrawStack)
	if err != nil {
		t.Fatalf("unexpected withdraw stack error: %v", err)
	}
	if count2 != 150 || len(newBag2.Items) != 1 || newBag2.Items[0].Amount != 150 {
		t.Fatalf("stackable not placed in bag slot 121: %+v", newBag2.Items)
	}
	if newVault2.ItemAt(1) == nil || newVault2.ItemAt(1).Amount != 150 {
		t.Fatalf("vault slot 1 should have remaining 150")
	}
}

func TestVaultMoveWithinVault(t *testing.T) {
	rules := BagRules{MissingStackLimit: 1000}
	bag := Bag{Version: "ordinary-bag-v1"}
	vault := Vault{
		Slots: 8,
		Items: []VaultItem{
			{Slot: 0, Template: 100261068, Durability: 40, IsEquip: true},
			{Slot: 2, Template: 3037, Amount: 100, IsEquip: false},
			{Slot: 3, Template: 3037, Amount: 200, IsEquip: false},
		},
	}

	rMove := protocol.ItemMoveRequest{SourceList: 2, SourceSlot: 0, DestinationList: 2, DestinationSlot: 5}
	_, newVault, _, err := MoveVaultItem(bag, vault, rules, rMove)
	if err != nil {
		t.Fatalf("unexpected vault move error: %v", err)
	}
	if newVault.ItemAt(0) != nil || newVault.ItemAt(5) == nil || newVault.ItemAt(5).Template != 100261068 {
		t.Fatalf("item should have moved from 0 to 5: %+v", newVault.Items)
	}

	rMerge := protocol.ItemMoveRequest{SourceList: 2, SourceSlot: 2, DestinationList: 2, DestinationSlot: 3, Count: 100}
	_, newVault2, _, err := MoveVaultItem(bag, newVault, rules, rMerge)
	if err != nil {
		t.Fatalf("unexpected vault merge error: %v", err)
	}
	if newVault2.ItemAt(2) != nil || newVault2.ItemAt(3).Amount != 300 {
		t.Fatalf("merge failed: %+v", newVault2.Items)
	}
}

func TestVaultSerialization(t *testing.T) {
	v := Vault{
		Slots: 8,
		Items: []VaultItem{
			{Slot: 0, Template: 3037, Amount: 100},
			{Slot: 1, Template: 100261068, Durability: 40, IsEquip: true},
		},
	}
	raw, err := SaveVault(v)
	if err != nil {
		t.Fatalf("SaveVault error: %v", err)
	}
	readBack, err := ReadExtendedVault(storage.VaultState{Slots: 8, Items: raw})
	if err != nil {
		t.Fatalf("ReadVault error: %v", err)
	}
	if len(readBack.Items) != 2 ||
		readBack.Items[0].Amount != 100 ||
		readBack.Items[1].Durability != 40 ||
		!readBack.Items[1].IsEquip {
		t.Fatalf("deserialized items mismatch: %+v", readBack.Items)
	}
}

func TestVaultBootstrapPopulated(t *testing.T) {
	rules := VaultRules{
		SourceSHA256:  "fda6c33f2a155124a4262f3e7cd6f7a18d4e2191e5f147aec5754716afe5aa7c",
		InitialSlots:  8,
		VerifiedSlots: []uint16{8},
	}
	if !rules.allows(8) {
		t.Fatal("expected rules to allow 8 slots")
	}
	v := Vault{Slots: 8, Items: []VaultItem{{Slot: 0, Template: 3037, Amount: 100}}}
	rows := v.Rows()
	if len(rows) != 1 {
		t.Fatalf("expected 1 row, got %d", len(rows))
	}
	p, err := protocol.PersonalVaultRestore(v.Slots, rows)
	if err != nil {
		t.Fatalf("PersonalVaultRestore error: %v", err)
	}
	if len(p) != 1+2+2+protocol.CurrentItemRecordSize {
		t.Fatalf("unexpected length %d", len(p))
	}
}
