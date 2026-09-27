package inventory

import (
	"dfolan/internal/catalog/pvf"
	"dfolan/internal/game/protocol"
	"encoding/hex"
	"testing"
)

func TestVaultPetWithdrawalFromObservedCMD19(t *testing.T) {
	catalog := &EquipmentCatalog{index: map[uint32]EquipmentDefinition{
		63003: {ID: 63003, Fields: map[string][]pvf.Token{"[equipment type]": {{Type: 6, Text: "[creature]"}}}},
		63502: {ID: 63502, Fields: map[string][]pvf.Token{"[equipment type]": {{Type: 6, Text: "[artifact red]"}}}},
	}}
	vault := Vault{Slots: 216, Items: []VaultItem{
		{Slot: 0, Template: 63502, IsEquip: true, Equipment: &BagEquipment{Slot: 0, Template: 63502, Durability: 7}},
		{Slot: 1, Template: 63003, IsEquip: true, Equipment: &BagEquipment{Slot: 1, Template: 63003, Period: 86400}},
	}}
	bag := Bag{Version: "ordinary-bag-v1"}
	for _, sample := range []struct {
		hex  string
		slot uint16
		id   uint32
	}{
		{"0201001bf60000010000000700000000000000000000ffffffff000000000000", 0, 63003},
		{"0200000ef80000010000000741010000000000000000ffffffff000000000000", 321, 63502},
	} {
		body, err := hex.DecodeString(sample.hex)
		if err != nil {
			t.Fatal(err)
		}
		request, err := protocol.DecodeItemMove(body)
		if err != nil || request.SourceList != 2 || request.DestinationList != 7 || request.DestinationSlot != sample.slot {
			t.Fatalf("observed CMD19 decoded unexpectedly: %+v, %v", request, err)
		}
		var moved uint32
		bag, vault, moved, err = MoveVaultItem(bag, vault, BagRules{EquipmentSlots: [2]uint16{9, 64}}, request, catalog)
		if err != nil || moved != 1 {
			t.Fatalf("withdraw %d: moved=%d err=%v", sample.id, moved, err)
		}
	}
	if len(vault.Items) != 0 || len(bag.Special[7]) != 2 || bag.Special[7][0].Slot != 0 || bag.Special[7][0].Period != 86400 || bag.Special[7][1].Slot != 321 || bag.Special[7][1].Durability != 7 {
		t.Fatalf("pet instances were not preserved: bag=%+v vault=%+v", bag, vault)
	}
	if _, err := PetContainerBody(bag, true); err != nil {
		t.Fatalf("pet container cannot be restored: %v", err)
	}
	wrong := Vault{Slots: 216, Items: []VaultItem{{Slot: 0, Template: 63502, IsEquip: true}}}
	if _, after, _, err := MoveVaultItem(Bag{Version: "ordinary-bag-v1"}, wrong, BagRules{}, protocol.ItemMoveRequest{SourceList: 2, SourceSlot: 0, DestinationList: 7, DestinationSlot: 0, Count: 1}, catalog); err == nil || len(after.Items) != 1 {
		t.Fatalf("artifact accepted in creature slot or lost from vault: %+v, %v", after, err)
	}
}
