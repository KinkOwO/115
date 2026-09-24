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

func TestCreatureExperienceAwardPersistsAndLevels(t *testing.T) {
	bag := Bag{Version: "ordinary-bag-v1", Worn: []BagEquipment{{Slot: 26, Template: 63000}}}
	state, err := SaveBag(json.RawMessage(`{"other_state":7}`), bag)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []struct {
		exp   uint32
		level byte
	}{{1, 1}, {2, 2}} {
		var gained uint32
		state, gained, err = AwardEquippedCreatureExperience(state, 1)
		if err != nil || gained != 1 {
			t.Fatalf("award: gained=%d err=%v", gained, err)
		}
		growth, err := CreatureGrowthPayload(state)
		if err != nil || len(growth) != 6 || growth[0] != want.level || binary.LittleEndian.Uint32(growth[2:]) != want.exp {
			t.Fatalf("growth for exp %d: %x err=%v", want.exp, growth, err)
		}
		list, err := CreatureListPayload(state)
		if err != nil || binary.LittleEndian.Uint32(list[7:11]) != want.exp || list[11] != want.level {
			t.Fatalf("list for exp %d: %x err=%v", want.exp, list, err)
		}
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(state, &fields); err != nil || string(fields["other_state"]) != "7" {
		t.Fatalf("unrelated state changed: %s err=%v", state, err)
	}
	bag, err = ReadBag(state)
	if err != nil {
		t.Fatal(err)
	}
	bag.Special = map[byte][]BagEquipment{7: {{Slot: 0, Template: bag.Worn[0].Template, Record: bag.Worn[0].Record}}}
	bag.Worn = nil
	state, err = SaveBag(state, bag)
	if err != nil {
		t.Fatal(err)
	}
	if growth, err := CreatureGrowthPayload(state); err != nil || growth != nil {
		t.Fatalf("unequipped growth=%x err=%v", growth, err)
	}
	if _, gained, err := AwardEquippedCreatureExperience(state, 5); err != nil || gained != 0 {
		t.Fatalf("unequipped award=%d err=%v", gained, err)
	}
	bag.Worn = []BagEquipment{bag.Special[7][0]}
	bag.Worn[0].Slot = 26
	bag.Special = nil
	state, err = SaveBag(state, bag)
	if err != nil {
		t.Fatal(err)
	}
	growth, err := CreatureGrowthPayload(state)
	if err != nil || binary.LittleEndian.Uint32(growth[2:]) != 2 {
		t.Fatalf("re-equipped experience lost: %x err=%v", growth, err)
	}
}
