package inventory

import (
	"dfolan/internal/catalog"
	"dfolan/internal/catalog/pvf"
	"dfolan/internal/game/protocol"
	"dfolan/internal/savecontract"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// The knight's shield ([support weapon], worn slot 24) is a source-verified
// exception to two rules every other weapon-shaped piece goes through:
//
//   - the 27 live-PVF shields (113370002..113370040) carry no [durability]
//     row at all, so Reward must read zero instead of refusing the piece;
//   - [usable job] is [knight] only, so the wear gate has to accept the job
//     the character catalog reports for the knight profession.
//
// Before the catalog carried these rows the wear gate refused every shield
// with "equipment definition missing" - the live report "骑士的shield无效".
func TestWearAcceptsKnightShield(t *testing.T) {
	const sum = "1111111111111111111111111111111111111111111111111111111111111111"
	shield := EquipmentDefinition{
		ID: 113370002, Path: "equipment/character/knight/weapon/shield/113370002.equ", SHA256: sum,
		Fields: map[string][]pvf.Token{
			"[name]":           {{Type: 8, Reference: "<3::name_113370002>"}},
			"[grade]":          {{Type: 0, Value: 1}},
			"[rarity]":         {{Type: 0, Value: 0}},
			"[usable job]":     {{Type: 6, Text: "[knight]"}},
			"[attach type]":    {{Type: 6, Text: "[trade]"}},
			"[minimum level]":  {{Type: 0, Value: 1}},
			"[equipment type]": {{Type: 6, Text: "[support weapon]"}, {Type: 0, Value: 21}},
		},
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "equipment.json")
	b, e := json.Marshal(EquipmentCatalog{Rows: []EquipmentDefinition{shield}})
	if e != nil {
		t.Fatal(e)
	}
	if e = os.WriteFile(path, b, 0o600); e != nil {
		t.Fatal(e)
	}
	eq, e := LoadEquipmentCatalog(path, "")
	if e != nil {
		t.Fatal(e)
	}
	d, e := eq.Reward(113370002)
	if e != nil {
		t.Fatalf("shield fails the reward check: %v", e)
	}
	if d != 0 {
		t.Fatalf("shield durability=%d, want 0 (source has no [durability])", d)
	}
	if _, e = eq.Basic(113370002); e == nil {
		t.Fatal("[trade] shield must stay out of the drop pool")
	}
	svc := &WearService{
		Catalog:     eq,
		Professions: catalog.Characters{Professions: map[byte]catalog.Profession{9: {Job: "[knight]"}}},
		BagRules:    BagRules{EquipmentSlots: [2]uint16{9, 44}},
		Rules:       WearRules{Slots: map[string]uint16{"[support weapon]": 24}},
	}
	bag := Bag{Version: "ordinary-bag-v1", Equipment: []BagEquipment{{Slot: 9, Template: 113370002}}}
	state, e := SaveBag(json.RawMessage(`{"level":1,"advancement":0}`), bag)
	if e != nil {
		t.Fatal(e)
	}
	role := Role{Profession: 9, ConfigVersion: savecontract.Identity(), State: state}
	r := protocol.ItemMoveRequest{SourceSlot: 9, SourceItem: 113370002, DestinationList: 3, DestinationSlot: 24, Count: 1, Selection: 0xffffffff}
	raw, e := svc.MoveOrdinary(role, r)
	if e != nil {
		t.Fatalf("knight could not equip the shield: %v", e)
	}
	worn, e := ReadBag(raw)
	if e != nil || len(worn.Worn) != 1 || worn.Worn[0].Template != 113370002 || worn.Worn[0].Slot != 24 {
		t.Fatalf("shield not worn on slot 24: %+v %v", worn, e)
	}
}
