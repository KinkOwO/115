package main

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"reflect"
	"testing"

	"dfolan/internal/catalog"
	"dfolan/internal/database"
	"dfolan/internal/game/protocol"
	"dfolan/internal/inventory"
	"dfolan/internal/loot"
)

func TestBoosterFixedAvatarGrantsWholeOutfitAndPreservesState(t *testing.T) {
	const box uint32 = 50041807
	ids := []uint32{508550408, 508560408, 508570408, 508500408, 508510408, 508520408, 508530408, 508540408}
	previous := inventory.BagEquipment{Slot: 0, Template: 517562678, Durability: 3, Period: MaxExpireTime}
	// Captured save has a stackable skin selection box in the avatar container.
	// Do not rewrite it or serialize it into an incremental avatar update.
	legacyBox := inventory.BagEquipment{Slot: 1, Template: 590701702, Period: MaxExpireTime}
	bag := inventory.Bag{Version: "ordinary-bag-v1", Items: []inventory.BagItem{{Slot: 65, Template: box, Amount: 1}}, Special: map[byte][]inventory.BagEquipment{1: {previous, legacyBox}}}
	state, err := inventory.SaveBag(json.RawMessage(`{"unrelated":{"value":7}}`), bag)
	if err != nil {
		t.Fatal(err)
	}
	store := newMockBoosterStore(database.Character{ID: 1, AccountID: 1, State: state})
	w := &worldSession{role: store.character}
	def := catalog.BoosterDefinition{Template: box, Type: "[booster]"}
	cat := &BoosterCatalog{Definitions: map[uint32]BoosterDefinition{}, Items: map[uint32]ItemIndexInfo{}}
	for _, id := range ids {
		def.Pools = append(def.Pools, catalog.BoosterRewardPool{DrawCount: 1, Candidates: []catalog.BoosterRewardCandidate{{Template: id, Weight: 1000, Count: 1}}})
		cat.Items[id] = catalog.ItemIndexEntry{ID: id, Kind: "equipment", Path: "equipment/character/priest/avatar/test.equ"}
	}
	cat.Definitions[box] = def
	req := make([]byte, 8)
	binary.LittleEndian.PutUint16(req, 65)
	binary.LittleEndian.PutUint32(req[2:], 1)
	packets, err := w.openBoosterItem(context.Background(), store, nil, nil, cat, odysseyWeaponChoices{}, req, req)
	if err != nil {
		t.Fatal(err)
	}
	saved, err := inventory.ReadBag(store.character.State)
	if err != nil {
		t.Fatal(err)
	}
	if len(saved.Items) != 0 || len(saved.Special[1]) != 10 || !reflect.DeepEqual(saved.Special[1][:2], []inventory.BagEquipment{previous, legacyBox}) {
		t.Fatalf("box consumption or existing outfit changed: %+v", saved)
	}
	want := make([]protocol.BoosterGrantedItem, 0, 8)
	for i, id := range ids {
		if got := saved.Special[1][i+2]; got.Template != id {
			t.Fatalf("grant %d = %+v", i, got)
		}
		want = append(want, protocol.BoosterGrantedItem{Template: id, Count: 1})
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(store.character.State, &fields); err != nil {
		t.Fatal(err)
	}
	if string(fields["unrelated"]) != `{"value":7}` {
		t.Fatal("unrelated character state changed")
	}
	found := false
	for _, p := range packets {
		if p.Name == "booster_avatar_inventory_updated" {
			// Each newly granted avatar: 181-byte base, two empty u32 socket
			// lengths and one u32 period. The unchanged legacy box is omitted.
			const stride = protocol.CurrentItemRecordSize + 12
			if len(p.Payload) != 3+8*stride || p.Payload[0] != 1 || binary.LittleEndian.Uint16(p.Payload[1:3]) != 8 {
				t.Fatalf("avatar delta must contain exactly eight rows: size=%d", len(p.Payload))
			}
			for i, id := range ids {
				off := 3 + i*stride
				if binary.LittleEndian.Uint16(p.Payload[off:]) != uint16(i+2) || binary.LittleEndian.Uint32(p.Payload[off+2:]) != id {
					t.Fatalf("wrong avatar delta row %d", i)
				}
			}
		}
		if p.Name == "booster_open_ack" {
			found = true
			if !reflect.DeepEqual(p.Payload, protocol.BoosterOpenSuccess(box, 65, want)) {
				t.Fatalf("wrong eight-item reply: %x", p.Payload)
			}
		}
	}
	if !found {
		t.Fatal("missing booster success reply")
	}
}

func TestBoosterStackableAvatarNamedBoxStaysInMainBag(t *testing.T) {
	const parent uint32 = 590701769
	const child uint32 = 590701702
	bag := inventory.Bag{Version: "ordinary-bag-v1", Items: []inventory.BagItem{{Slot: 65, Template: parent, Amount: 1}}}
	state, err := inventory.SaveBag(json.RawMessage(`{}`), bag)
	if err != nil {
		t.Fatal(err)
	}
	store := newMockBoosterStore(database.Character{ID: 1, AccountID: 1, State: state})
	w := &worldSession{role: store.character}
	cat := &BoosterCatalog{
		Definitions: map[uint32]BoosterDefinition{parent: {Template: parent, Type: "[booster]", Pools: []catalog.BoosterRewardPool{{DrawCount: 1, Candidates: []catalog.BoosterRewardCandidate{{Template: child, Weight: 1000, Count: 1}}}}}},
		Items:       map[uint32]ItemIndexInfo{child: {ID: child, Kind: "stackable", Path: "stackable/dfo/cash/2020/0804/etc/skin_avatar_box.stk"}},
	}
	lootSvc := &loot.Service{
		Catalog:  catalog.LootCatalog{Items: map[uint32]catalog.LootItem{child: {ID: child, Kind: "stackable", StackableType: "[booster selection]", StackLimit: 1000}}},
		BagRules: inventory.BagRules{Source: "test", Slots: map[string][2]uint16{"[booster selection]": {65, 120}}, MissingStackLimit: 1000},
	}
	req := make([]byte, 8)
	binary.LittleEndian.PutUint16(req, 65)
	binary.LittleEndian.PutUint32(req[2:], 1)
	packets, err := w.openBoosterItem(context.Background(), store, nil, lootSvc, cat, odysseyWeaponChoices{}, req, req)
	if err != nil {
		t.Fatal(err)
	}
	saved, err := inventory.ReadBag(store.character.State)
	if err != nil {
		t.Fatal(err)
	}
	if len(saved.Special[1]) != 0 || len(saved.Items) != 1 || saved.Items[0].Template != child || saved.Items[0].Amount != 1 {
		t.Fatalf("stackable skin box must stay stackable: %+v", saved)
	}
	for _, p := range packets {
		if p.Name == "booster_avatar_inventory_updated" {
			t.Fatal("stackable skin box produced an avatar update")
		}
	}
}
