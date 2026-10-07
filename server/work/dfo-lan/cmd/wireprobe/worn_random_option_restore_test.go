package main

import (
	"bytes"
	"dfolan/internal/character"
	"dfolan/internal/database"
	"dfolan/internal/dungeon"
	"dfolan/internal/game/protocol"
	"dfolan/internal/inventory"
	"encoding/json"
	"testing"
)

func TestDungeonLoadingRestoresIdentifiedGearAfterReconstruction(t *testing.T) {
	// Exact identified weapon option bytes from character 4's 2026-10-04
	// NOTI14: count=1, id=21, valueA=12, valueB=11, type=2, no special.
	row := protocol.OrdinaryItem(12, 104010184, 0)
	copy(row[60:72], []byte{1, 21, 0, 0, 12, 0, 0, 11, 0, 0, 2, 255})
	gear := inventory.BagEquipment{Slot: 12, Template: 104010184, Durability: 40, Record: row[:]}
	state, err := inventory.SaveBag(json.RawMessage(`{"attributes":{"[hp max]":100,"[mp max]":100}}`), inventory.Bag{
		Version: "ordinary-bag-v1", Worn: []inventory.BagEquipment{gear, {Slot: 14, Template: 100050243}},
	})
	if err != nil {
		t.Fatal(err)
	}
	w := &worldSession{role: database.Character{ID: 4, WireID: 4, State: state}, activeDungeon: &dungeon.Session{}, characters: &character.Service{}}
	original := append([]byte(nil), state...)
	plan, err := w.finishDungeonLoading(make([]byte, 16))
	if err != nil {
		t.Fatal(err)
	}
	reconstruction, replay := -1, -1
	for i, p := range plan {
		if p.Name == "dungeon_worn_visuals_restored" {
			reconstruction = i
		}
		if p.Name == "dungeon_worn_random_options_restored" {
			if replay >= 0 {
				t.Fatal("duplicate option replay")
			}
			replay = i
			want, e := inventory.EquipmentPayload(3, []inventory.BagEquipment{gear}, false)
			if e != nil || p.Kind != 0 || p.ID != 14 || !bytes.Equal(p.Payload, want) {
				t.Fatal("identified instance changed or unsealed gear included")
			}
		}
	}
	if reconstruction < 0 || replay <= reconstruction {
		t.Fatal("options must follow worn object reconstruction")
	}
	if !bytes.Equal(original, w.role.State) {
		t.Fatal("refresh changed saved state")
	}
	if plan[len(plan)-1].ID != 1361 {
		t.Fatal("buff registration must still follow restoration")
	}
}

func TestDungeonWornOptionReplayScope(t *testing.T) {
	row := protocol.OrdinaryItem(3, 517500000, 0)
	row[60] = 1
	state, err := inventory.SaveBag(json.RawMessage(`{}`), inventory.Bag{Version: "ordinary-bag-v1",
		Worn:      []inventory.BagEquipment{{Slot: 3, Template: 517500000, Record: row[:]}, {Slot: 12, Template: 104010184}},
		Equipment: []inventory.BagEquipment{{Slot: 10, Template: 517500000, Record: row[:]}},
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, w := range []*worldSession{nil, {role: database.Character{State: state}}, {role: database.Character{State: state}, activeDungeon: &dungeon.Session{}}} {
		plan, err := appendDungeonWornRandomOptions([]outboundPacket{{Name: "existing"}}, w)
		if err != nil || len(plan) != 1 || plan[0].Name != "existing" {
			t.Fatal("town, bag, avatar or unidentified equipment triggered replay")
		}
	}
}
