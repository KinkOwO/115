package inventory

import (
	"encoding/binary"
	"encoding/json"
	"testing"
)

func TestCreatureListPayloadAndEquipmentRow(t *testing.T) {
	b := Bag{
		Version: "ordinary-bag-v1",
		Worn: []BagEquipment{
			{Slot: 26, Template: 63000}, // Faras
		},
		Special: map[byte][]BagEquipment{
			7: {
				{Slot: 0, Template: 63009}, // Marbas
				{Slot: 1, Template: 63006}, // unhatched Pareas egg
			},
		},
	}
	raw, err := SaveBag(json.RawMessage(`{}`), b)
	if err != nil {
		t.Fatal(err)
	}

	payload, err := CreatureListPayload(raw)
	if err != nil {
		t.Fatal(err)
	}
	// Expected 2 hatched creatures (slot 26 + slot 0), egg at slot 1 skipped
	if payload[0] != 2 {
		t.Fatalf("expected count 2, got %d", payload[0])
	}
	// Verify first entry (equipped creature at slot 26) has Key = 1
	key1 := binary.LittleEndian.Uint32(payload[1:5])
	if key1 != 1 {
		t.Fatalf("expected equipped creature key = 1, got %d", key1)
	}
	// Verify creature name Faras is written
	nameLen := binary.LittleEndian.Uint32(payload[12:16])
	if nameLen != 5 || string(payload[16:21]) != "Faras" {
		t.Fatalf("expected Faras name, got len=%d str=%s", nameLen, string(payload[16:16+nameLen]))
	}

	// Verify EquipmentRow of slot 26 has Key = 1 at offset 6
	row := EquipmentRow(b.Worn[0])
	dataVal := binary.LittleEndian.Uint32(row[6:10])
	if dataVal != 1 {
		t.Fatalf("expected EquipmentRow offset 6 = 1, got %d", dataVal)
	}
	if binary.LittleEndian.Uint16(row[0:2]) != 26 {
		t.Fatalf("expected slot 26, got %d", binary.LittleEndian.Uint16(row[0:2]))
	}
	if binary.LittleEndian.Uint32(row[2:6]) != 63000 {
		t.Fatalf("expected template 63000, got %d", binary.LittleEndian.Uint32(row[2:6]))
	}

	// Verify HasEquippedCreature
	if !HasEquippedCreature(raw) {
		t.Fatal("expected HasEquippedCreature to be true")
	}

	// Verify empty worn returns false
	emptyRaw, _ := SaveBag(json.RawMessage(`{}`), Bag{Version: "ordinary-bag-v1"})
	if HasEquippedCreature(emptyRaw) {
		t.Fatal("expected empty bag HasEquippedCreature to be false")
	}
}
