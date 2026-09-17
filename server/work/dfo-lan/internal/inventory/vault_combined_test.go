package inventory

import (
	"bytes"
	"dfolan/internal/game/protocol"
	"encoding/json"
	"testing"
)

func TestCombinedVaultPreservesEquipmentAndStackFixes(t *testing.T) {
	s, role, v := vaultFixture()
	s.BagRules.EquipmentSlots = [2]uint16{9, 64}
	b, _ := ReadBag(role.State)
	record := protocol.OrdinaryItem(10, 100261068, 0)
	record[50] = 17
	b.Equipment = []BagEquipment{{Slot: 10, Template: 100261068, Durability: 40, Record: record[:]}}
	role.State, _ = SaveBag(role.State, b)
	r := protocol.ItemMoveRequest{SourceList: 0, SourceSlot: 10, SourceItem: 100261068, DestinationList: 2, DestinationSlot: 1, Count: 1, Selection: 0xffffffff}
	state, items, err := s.TransferCombined(role, v, r)
	if err != nil {
		t.Fatal(err)
	}
	role.State, v.Items = state, items
	r = protocol.ItemMoveRequest{SourceList: 0, SourceSlot: 65, SourceItem: 15, DestinationList: 2, DestinationSlot: 0, Count: 4, Selection: 0xffffffff}
	state, items, err = s.TransferCombined(role, v, r)
	if err != nil {
		t.Fatal(err)
	}
	role.State, v.Items = state, items
	vault, err := ReadExtendedVault(v)
	if err != nil || len(vault.Items) != 2 || vault.ItemAt(0).Amount != 4 || vault.ItemAt(1).Equipment.Record[50] != 17 {
		t.Fatal(vault, err)
	}
	r = protocol.ItemMoveRequest{SourceList: 2, SourceSlot: 1, SourceItem: 100261068, DestinationList: 0, DestinationSlot: 11, Count: 1, Selection: 0xffffffff}
	state, items, err = s.TransferCombined(role, v, r)
	if err != nil {
		t.Fatal(err)
	}
	b, err = ReadBag(state)
	if err != nil || len(b.Equipment) != 1 || b.Equipment[0].Slot != 11 || !bytes.Equal(b.Equipment[0].Record, record[:]) {
		t.Fatal(b, err)
	}
	var remaining []VaultItem
	if err = json.Unmarshal(items, &remaining); err != nil || len(remaining) != 1 || remaining[0].Amount != 4 {
		t.Fatal(remaining, err)
	}
}

func TestCombinedVaultRejectsStaleAndUnknownMetadata(t *testing.T) {
	s, role, v := vaultFixture()
	r := protocol.ItemMoveRequest{SourceList: 0, SourceSlot: 65, SourceItem: 999, DestinationList: 2, DestinationSlot: 0, Count: 1, Selection: 0xffffffff}
	if _, _, err := s.TransferCombined(role, v, r); err == nil {
		t.Fatal("stale source accepted")
	}
	v.Items = []byte(`[{"slot":0,"template":15,"amount":1,"unknown_instance_field":1}]`)
	if _, err := ReadExtendedVault(v); err == nil {
		t.Fatal("unknown metadata discarded")
	}
}
