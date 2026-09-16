package inventory

import (
	"bytes"
	"dfolan/internal/catalog"
	"dfolan/internal/game/protocol"
	"dfolan/internal/storage"
	"encoding/json"
	"testing"
)

func wearFixture(t *testing.T) (*WearService, storage.Character) {
	t.Helper()
	c, e := catalog.LoadCharacters("../../configs/characters.next25.json")
	if e != nil {
		t.Fatal(e)
	}
	eq, e := LoadEquipmentCatalog("../../configs/equipment.current35.json", c.Source.Checksum)
	if e != nil {
		t.Fatal(e)
	}
	rules, e := LoadWearRules("../../configs/equipment-wear.current35.json", c.Source.Checksum)
	if e != nil {
		t.Fatal(e)
	}
	bagRules, e := LoadBagRules("../../configs/inventory.next29.json")
	if e != nil {
		t.Fatal(e)
	}
	b := Bag{Version: "ordinary-bag-v1", Gold: 345, Equipment: []BagEquipment{{Slot: 9, Template: 20002}, {Slot: 10, Template: 24002}}}
	raw, e := SaveBag(json.RawMessage(`{"level":6,"advancement":0,"quest_marker":3146}`), b)
	if e != nil {
		t.Fatal(e)
	}
	return &WearService{Catalog: eq, Professions: c, BagRules: bagRules, Rules: rules}, storage.Character{Profession: 0, ConfigVersion: c.Source.Checksum, State: raw}
}

func TestWearUnequipPreservesAssetsAndOtherModules(t *testing.T) {
	s, role := wearFixture(t)
	r := protocol.ItemMoveRequest{SourceSlot: 9, SourceItem: 20002, DestinationList: 3, DestinationSlot: 19, Count: 1, Selection: 0xffffffff}
	raw, e := s.MoveOrdinary(role, r)
	if e != nil {
		t.Fatal(e)
	}
	b, e := ReadBag(raw)
	if e != nil || len(b.Equipment) != 1 || len(b.Worn) != 1 || b.Worn[0].Template != 20002 || b.Worn[0].Slot != 19 || b.Gold != 345 {
		t.Fatal(b, e)
	}
	role.State = raw
	// Native empty-source right-click unequip: bag9 empty, worn19 occupied.
	r.SourceItem = 0
	r.DestinationItem = 20002
	raw, e = s.MoveOrdinary(role, r)
	if e != nil {
		t.Fatal(e)
	}
	b, e = ReadBag(raw)
	if e != nil || len(b.Equipment) != 2 || len(b.Worn) != 0 || b.Gold != 345 {
		t.Fatal(b, e)
	}
	var state map[string]json.RawMessage
	json.Unmarshal(raw, &state)
	if string(state["quest_marker"]) != "3146" || string(state["level"]) != "6" {
		t.Fatal("unrelated progress changed")
	}
}

func TestWearRejectsWrongSlotStaleIdentityAndBulk(t *testing.T) {
	s, role := wearFixture(t)
	base := protocol.ItemMoveRequest{SourceSlot: 9, SourceItem: 20002, DestinationList: 3, DestinationSlot: 19, Count: 1, Selection: 0xffffffff}
	for _, change := range []func(*protocol.ItemMoveRequest){func(r *protocol.ItemMoveRequest) { r.DestinationSlot = 20 }, func(r *protocol.ItemMoveRequest) { r.SourceItem = 12345 }, func(r *protocol.ItemMoveRequest) { r.Count = 2 }, func(r *protocol.ItemMoveRequest) { r.SourceList = 12 }, func(r *protocol.ItemMoveRequest) { r.Extra = 1 }} {
		r := base
		change(&r)
		before := append([]byte(nil), role.State...)
		if _, e := s.MoveOrdinary(role, r); e == nil {
			t.Fatal("invalid wear accepted", r)
		}
		if !bytes.Equal(before, role.State) {
			t.Fatal("refusal mutated state")
		}
	}
}

func TestWornBootstrapMatchesNativeReader(t *testing.T) {
	s, role := wearFixture(t)
	raw, e := s.MoveOrdinary(role, protocol.ItemMoveRequest{SourceSlot: 9, SourceItem: 20002, DestinationList: 3, DestinationSlot: 19, Count: 1, Selection: 0xffffffff})
	if e != nil {
		t.Fatal(e)
	}
	body, e := WornPayload(raw)
	if e != nil {
		t.Fatal(e)
	}
	// u8 list3/u16 count,181 bytes and a period u32, verified natively.
	if len(body) != 188 || body[0] != 3 || body[1] != 1 || body[3] != 19 {
		t.Fatal("worn packet layout", len(body))
	}
	if _, e = ReadBag(raw); e != nil {
		t.Fatal(e)
	}
}
