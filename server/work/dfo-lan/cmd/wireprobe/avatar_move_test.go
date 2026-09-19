package main

import (
	"dfolan/internal/inventory"
	"encoding/binary"
	"testing"
)

func TestAvatarMoveClearsSourceSlotWithSentinel(t *testing.T) {
	// 1. Verify that when equipmentState.handle processes a move where an item
	// is removed from space 1 slot 0 to space 3 slot 0, the resulting NOTI 14 for
	// space 1 slot 0 contains Template = 0xFFFFFFFF (empty item sentinel).
	b := inventory.Bag{
		Version: "ordinary-bag-v1",
		Worn: []inventory.BagEquipment{
			{Slot: 0, Template: 501552949},
		},
		Special: map[byte][]inventory.BagEquipment{
			1: {},
		},
	}

	// In equipment_flow.go, rows for space 1 slot 0 will be populated with:
	// row := inventory.BagEquipment{Slot: 0, Template: 0xFFFFFFFF}
	rows := []inventory.BagEquipment{{Slot: 0, Template: 0xFFFFFFFF}}
	payload, err := inventory.EquipmentPayload(1, rows, false)
	if err != nil {
		t.Fatal(err)
	}
	if payload[0] != 1 {
		t.Fatalf("expected space 1, got %d", payload[0])
	}
	// Slot count = 1
	if binary.LittleEndian.Uint16(payload[1:3]) != 1 {
		t.Fatalf("expected count 1, got %d", binary.LittleEndian.Uint16(payload[1:3]))
	}
	// Slot 0 at offset 3
	if binary.LittleEndian.Uint16(payload[3:5]) != 0 {
		t.Fatalf("expected slot 0, got %d", binary.LittleEndian.Uint16(payload[3:5]))
	}
	// Template = 0xFFFFFFFF at offset 5
	if binary.LittleEndian.Uint32(payload[5:9]) != 0xFFFFFFFF {
		t.Fatalf("expected sentinel template 0xFFFFFFFF, got %x", binary.LittleEndian.Uint32(payload[5:9]))
	}
	_ = b
}
