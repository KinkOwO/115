package main

import (
	"context"
	"dfolan/internal/catalog"
	"dfolan/internal/database"
	"dfolan/internal/game/protocol"
	"dfolan/internal/inventory"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"os"
	"reflect"
	"testing"
)

// Use the actual archer and dark-knight CMD160 requests, checking committed
// inventory and the result packet together rather than only decoding the list.
func TestBoosterNativeAvatarPackagePersistsAllSelections(t *testing.T) {
	data, err := os.ReadFile("../../internal/game/protocol/testdata/native_booster_avatar_package_20261001.json")
	if err != nil {
		t.Fatal(err)
	}
	var vectors []struct {
		Name       string   `json:"name"`
		Box        uint32   `json:"box"`
		Slot       uint16   `json:"slot"`
		PlainHex   string   `json:"plain_hex"`
		Selections []uint32 `json:"selections"`
	}
	if err := json.Unmarshal(data, &vectors); err != nil {
		t.Fatal(err)
	}
	for _, v := range vectors {
		t.Run(v.Name, func(t *testing.T) {
			raw, err := hex.DecodeString(v.PlainHex)
			if err != nil {
				t.Fatal(err)
			}
			previous := inventory.BagEquipment{Slot: 0, Template: 517562678, Durability: 3, Period: MaxExpireTime}
			bag := inventory.Bag{Version: "ordinary-bag-v1", Items: []inventory.BagItem{{Slot: v.Slot, Template: v.Box, Amount: 1}}, Special: map[byte][]inventory.BagEquipment{1: {previous}}}
			state, err := inventory.SaveBag(json.RawMessage(`{"unrelated":{"value":7}}`), bag)
			if err != nil {
				t.Fatal(err)
			}
			char := database.Character{ID: 1, AccountID: 1, State: state}
			store := newMockBoosterStore(char)
			w := &worldSession{role: char}
			cat := &BoosterCatalog{Items: map[uint32]ItemIndexInfo{}}
			for _, tpl := range v.Selections {
				cat.Items[tpl] = catalog.ItemIndexEntry{ID: tpl, Kind: "equipment", Path: "equipment/character/archer/avatar/test.equ"}
			}
			packets, err := w.openBoosterItem(context.Background(), store, nil, nil, cat, odysseyWeaponChoices{}, raw, raw)
			if err != nil {
				t.Fatal(err)
			}
			saved, err := inventory.ReadBag(store.character.State)
			if err != nil {
				t.Fatal(err)
			}
			if len(saved.Items) != 0 || len(saved.Special[1]) != 9 || !reflect.DeepEqual(saved.Special[1][0], previous) {
				t.Fatalf("box consumption or existing avatar changed: %+v", saved)
			}
			for i, tpl := range v.Selections {
				got := saved.Special[1][i+1]
				if got.Template != tpl || got.Durability != 0 {
					t.Fatalf("grant %d=%+v want template=%d with no ability option", i, got, tpl)
				}
			}
			var fields map[string]json.RawMessage
			if err := json.Unmarshal(store.character.State, &fields); err != nil {
				t.Fatal(err)
			}
			if string(fields["unrelated"]) != `{"value":7}` {
				t.Fatalf("unrelated state changed: %s", store.character.State)
			}
			var ack []byte
			for _, packet := range packets {
				if packet.Name == "booster_open_ack" {
					ack = packet.Payload
				}
			}
			want := make([]protocol.BoosterGrantedItem, 0, len(v.Selections))
			for _, tpl := range v.Selections {
				want = append(want, protocol.BoosterGrantedItem{Template: tpl, Count: 1})
			}
			if !reflect.DeepEqual(ack, protocol.BoosterOpenSuccess(v.Box, v.Slot, want)) {
				t.Fatalf("ack did not list all eight grants: %x", ack)
			}
			// NOTI14 space 1 restores the old avatar and all eight new arrivals.
			found := false
			for _, packet := range packets {
				if packet.Name == "booster_avatar_inventory_updated" {
					found = true
					if len(packet.Payload) < 3 || packet.Payload[0] != 1 || binary.LittleEndian.Uint16(packet.Payload[1:3]) != 9 {
						t.Fatalf("avatar update lost items: %x", packet.Payload)
					}
				}
			}
			if !found {
				t.Fatal("missing avatar inventory update")
			}
		})
	}
}
