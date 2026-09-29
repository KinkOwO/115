package inventory

import (
	"bytes"
	"dfolan/internal/catalog/pvf"
	"dfolan/internal/game/protocol"
	"dfolan/internal/storage"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"reflect"
	"testing"
)

func amalgamationWearFixture(t *testing.T) (*WearService, storage.Character) {
	t.Helper()
	s, role := wearFixture(t)
	s.Rules.Special = true
	s.Rules.Slots = map[string]uint16{"[coat]": 14, "[oath]": 47, "[primer]": 36}
	// Relevant fields from the current exported PVF. Different parts share
	// the same eight client-selected slots, rather than normal body slots.
	for _, row := range []struct {
		id     uint32
		kind   string
		part   string
		level  int32
		rarity int32
	}{
		{100323404, "[amalgamation stone]", "[ring]", 110, 8},
		{100251186, "[amalgamation stone]", "[shoes]", 115, 3},
		{100051351, "[amalgamation stone]", "[coat]", 115, 3},
		{5001, "[coat]", "", 1, 3},
		{5002, "[unknown equipment]", "", 1, 3},
		{5003, "[talisman]", "", 1, 3},
		{5004, "[primer]", "", 115, 8},
		{5005, "[oath]", "", 115, 8},
	} {
		s.Catalog.index[row.id] = EquipmentDefinition{ID: row.id, Fields: map[string][]pvf.Token{
			"[equipment type]":    {{Type: 6, Text: row.kind}},
			"[amalgamation part]": {{Type: 6, Text: row.part}},
			"[minimum level]":     {{Type: 0, Value: row.level}},
			"[usable job]":        {{Type: 6, Text: "[all]"}},
			"[rarity]":            {{Type: 0, Value: row.rarity}},
		}}
	}
	role.State = json.RawMessage(`{"level":115,"advancement":0,"quest_marker":3146}`)
	return s, role
}

func amalgamationInstance(slot uint16, template, key uint32) BagEquipment {
	record := bytes.Repeat([]byte{0x53}, protocol.CurrentItemRecordSize)
	binary.LittleEndian.PutUint16(record, slot)
	binary.LittleEndian.PutUint32(record[2:], template)
	binary.LittleEndian.PutUint32(record[6:], key)
	return BagEquipment{Slot: slot, Template: template, Record: record, Durability: 12, Period: 456}
}

func TestWearAmalgamationStoneUsesEightSlots(t *testing.T) {
	s, role := amalgamationWearFixture(t)
	for _, id := range []uint32{100323404, 100251186, 100051351} {
		for slot := uint16(36); slot <= 43; slot++ {
			t.Run(fmt.Sprintf("%d/slot%d", id, slot), func(t *testing.T) {
				item := amalgamationInstance(9, id, 123)
				state, err := SaveBag(role.State, Bag{Version: "ordinary-bag-v1", Equipment: []BagEquipment{item}})
				if err != nil {
					t.Fatal(err)
				}
				current := role
				current.State = state
				raw, err := s.MoveOrdinary(current, protocol.ItemMoveRequest{SourceSlot: 9, SourceItem: id,
					DestinationList: 3, DestinationSlot: slot, Count: 1, Selection: 0xffffffff})
				if err != nil {
					t.Fatal(err)
				}
				bag, err := ReadBag(raw)
				item.Slot = slot
				if err != nil || len(bag.Equipment) != 0 || !reflect.DeepEqual(bag.Worn, []BagEquipment{item}) {
					t.Fatalf("move lost instance data: %+v, %v", bag, err)
				}
			})
		}
	}
}

func TestWearAmalgamationCapturedRequests(t *testing.T) {
	s, role := amalgamationWearFixture(t)
	// Native CMD19 bodies from the 2026-09-29 11:57 session, lines 480/535.
	for _, body := range []string{
		"00200032b6f905010000000324000000000000000000ffffffff000000000000",
		"00240097a9f605010000000324000000000000000000ffffffff000000000000",
	} {
		p, err := hex.DecodeString(body)
		if err != nil {
			t.Fatal(err)
		}
		r, err := protocol.DecodeItemMove(p)
		if err != nil {
			t.Fatal(err)
		}
		current := role
		current.State, err = SaveBag(role.State, Bag{Version: "ordinary-bag-v1",
			Equipment: []BagEquipment{amalgamationInstance(r.SourceSlot, r.SourceItem, 123)}})
		if err != nil {
			t.Fatal(err)
		}
		if _, err = s.MoveOrdinary(current, r); err != nil {
			t.Fatalf("captured request still refused: %v", err)
		}
	}
}

func TestWearAmalgamationRejectsWrongSlotsAndKeepsOtherGates(t *testing.T) {
	s, role := amalgamationWearFixture(t)
	for _, slot := range []uint16{0, 14, 21, 32, 33, 35, 44, 45, 46, 47} {
		if err := s.wearable(role, amalgamationInstance(9, 100323404, 123), slot); err == nil {
			t.Fatalf("fusion stone admitted to slot %d", slot)
		}
	}
	if err := s.wearable(role, BagEquipment{Template: 5001}, 14); err != nil {
		t.Fatal(err)
	}
	for _, id := range []uint32{5001, 5002} {
		if err := s.wearable(role, BagEquipment{Template: id}, 36); err == nil {
			t.Fatalf("ordinary or unknown type %d leaked into special slot", id)
		}
	}
	s.Rules.Special = false
	if err := s.wearable(role, BagEquipment{Template: 100323404}, 36); err == nil {
		t.Fatal("fusion stone admitted while special slots disabled")
	}
	s.Rules.Special = true
	role.State = json.RawMessage(`{"level":109,"advancement":0}`)
	if err := s.wearable(role, BagEquipment{Template: 100323404}, 36); err == nil {
		t.Fatal("minimum level bypassed")
	}
	role.State = json.RawMessage(`{"level":110,"advancement":0}`)
	if err := s.wearable(role, BagEquipment{Template: 100323404}, 36); err != nil {
		t.Fatalf("level-110 stone inherited oath level-115 gate: %v", err)
	}
	for _, id := range []uint32{5004, 5005} {
		if err := s.wearable(role, BagEquipment{Template: id}, s.Rules.Slots[s.Catalog.index[id].Fields["[equipment type]"][0].Text]); err == nil {
			t.Fatal("oath level gate bypassed")
		}
	}
	role.State = json.RawMessage(`{"level":115,"advancement":0}`)
	if err := s.wearable(role, BagEquipment{Template: 5003}, 35); err != nil {
		t.Fatalf("tableless talisman exception is still short-circuited: %v", err)
	}
	if err := s.wearable(role, BagEquipment{Template: 5004}, 43); err == nil {
		t.Fatal("primeval crystal leaked into fusion-only slot")
	}
	if err := s.wearable(role, BagEquipment{Template: 5004}, 44); err != nil {
		t.Fatal(err)
	}
	d := s.Catalog.index[100323404]
	d.Fields["[usable job]"] = []pvf.Token{{Type: 6, Text: "[archer]"}}
	if err := s.wearable(role, BagEquipment{Template: 100323404}, 36); err == nil {
		t.Fatal("profession gate bypassed")
	}
}

func TestWearAmalgamationFullSetReplacementAndUnequipPreserveState(t *testing.T) {
	s, role := amalgamationWearFixture(t)
	bag := Bag{Version: "ordinary-bag-v1", Gold: 345,
		Equipment: []BagEquipment{amalgamationInstance(9, 100051351, 999)}}
	for slot := uint16(36); slot <= 43; slot++ {
		bag.Worn = append(bag.Worn, amalgamationInstance(slot, 100323404, uint32(slot)))
	}
	var err error
	role.State, err = SaveBag(role.State, bag)
	if err != nil {
		t.Fatal(err)
	}
	before := append([]byte(nil), role.State...)
	r := protocol.ItemMoveRequest{SourceSlot: 9, SourceItem: 100051351,
		DestinationList: 3, DestinationSlot: 46, Count: 1, Selection: 0xffffffff}
	if _, err = s.MoveOrdinary(role, r); err == nil || !bytes.Equal(before, role.State) {
		t.Fatal("overflow was accepted or changed input state")
	}
	r.DestinationSlot = 39
	r.DestinationItem = 100323404
	role.State, err = s.MoveOrdinary(role, r)
	if err != nil {
		t.Fatalf("full set should still allow replacement: %v", err)
	}
	replaced, err := ReadBag(role.State)
	expectedBag := amalgamationInstance(39, 100323404, 39)
	expectedBag.Slot = 9 // The instance's opaque record is preserved; encoding writes the current slot.
	if err != nil || len(replaced.Worn) != 8 || !reflect.DeepEqual(replaced.Equipment, []BagEquipment{expectedBag}) {
		t.Fatalf("replacement lost old item: %+v, %v", replaced, err)
	}
	// Empty-source right-click unequip moves the new stone back to the bag.
	r.SourceSlot, r.SourceItem, r.DestinationItem = 10, 0, 100051351
	role.State, err = s.MoveOrdinary(role, r)
	if err != nil {
		t.Fatal(err)
	}
	restored, err := ReadBag(role.State)
	expectedNew := amalgamationInstance(9, 100051351, 999)
	expectedNew.Slot = 10
	if err != nil || len(restored.Worn) != 7 || !reflect.DeepEqual(restored.Equipment,
		[]BagEquipment{expectedBag, expectedNew}) || restored.Gold != 345 {
		t.Fatalf("unequip lost assets: %+v, %v", restored, err)
	}
	var state map[string]json.RawMessage
	if err := json.Unmarshal(role.State, &state); err != nil || string(state["quest_marker"]) != "3146" {
		t.Fatal("unrelated character state changed", err)
	}
	if _, err := WornPayload(role.State); err != nil {
		t.Fatalf("saved stones cannot be restored through worn snapshot: %v", err)
	}
}
