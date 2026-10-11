package inventory

import (
	"dfolan/internal/catalog"
	"dfolan/internal/catalog/pvf"
	"dfolan/internal/game/protocol"
	"encoding/json"
	"testing"
)

func TestAvatarSelectionPersistsNativeField(t *testing.T) {
	s, role := wearFixture(t)
	s.Rules.Special = true
	s.Catalog.index[40601] = EquipmentDefinition{ID: 40601, Fields: map[string][]pvf.Token{
		"[equipment type]": {{Type: 6, Text: "[coat avatar]"}}, "[avatar type select]": {{Type: 0, Value: 7}}}}
	b, e := ReadBag(role.State)
	if e != nil {
		t.Fatal(e)
	}
	b.Special = map[byte][]BagEquipment{1: {{Slot: 4, Template: 40601}}}
	role.State, e = SaveBag(role.State, b)
	if e != nil {
		t.Fatal(e)
	}
	r := protocol.AvatarOptionRequest{Location: 2, Slot: 4, Template: 40601, Option: 15}
	raw, e := s.SelectAvatarOption(role, r, catalog.LootCatalog{})
	if e != nil {
		t.Fatal(e)
	}
	saved, e := ReadBag(raw)
	if e != nil {
		t.Fatal(e)
	}
	row := EquipmentRow(saved.Special[1][0])
	if row[11] != 15 || row[12] != 0 {
		t.Fatal("native field not persisted")
	}
	before, _ := json.Marshal(b.Equipment)
	after, _ := json.Marshal(saved.Equipment)
	if string(before) != string(after) {
		t.Fatal("ordinary items changed")
	}
	role.State = raw
	if _, e = s.SelectAvatarOption(role, r, catalog.LootCatalog{}); e == nil {
		t.Fatal("free reselection accepted")
	}
	r.Template++
	if _, e = s.SelectAvatarOption(role, r, catalog.LootCatalog{}); e == nil {
		t.Fatal("stale template accepted")
	}
}

func TestPaidRareCloneAvatarOptionConsumesBoxAndPersistsWornValue(t *testing.T) {
	s, role := wearFixture(t)
	s.Rules.Special = true
	const clone = uint32(517560103)
	s.Catalog.index[clone] = EquipmentDefinition{ID: clone, Fields: map[string][]pvf.Token{
		"[equipment type]": {{Type: 6, Text: "[hair avatar]"}},
		"[item category]":  {{Type: 6, Text: "clear avatar"}},
		"[rarity]":         {{Type: 0, Value: 2}},
		"[avatar select ability]": {
			{Type: 0, Value: 0}, {Type: 8, Reference: "[MAGICAL_ATTACK]"}, {Type: 6, Text: "+"}, {Type: 0, Value: 55},
			{Type: 0, Value: 1}, {Type: 8, Reference: "[MAGICAL_DEFENSE]"}, {Type: 6, Text: "+"}, {Type: 0, Value: 55},
			{Type: 0, Value: 2}, {Type: 8, Reference: "[HP_REGENRATE]"}, {Type: 6, Text: "+"}, {Type: 0, Value: 75},
		},
	}}
	b, err := ReadBag(role.State)
	if err != nil {
		t.Fatal(err)
	}
	b.Worn = []BagEquipment{{Slot: 1, Template: clone, Durability: 1}}
	b.Items = []BagItem{{Slot: 70, Template: rareAvatarOptionBoxTemplate, Amount: 4}, {Slot: 71, Template: 700, Amount: 8}}
	role.State, err = SaveBag(role.State, b)
	if err != nil {
		t.Fatal(err)
	}
	original := append(json.RawMessage(nil), role.State...)
	items := catalog.LootCatalog{Items: map[uint32]catalog.LootItem{
		rareAvatarOptionBoxTemplate: {ID: rareAvatarOptionBoxTemplate, Kind: "stackable", StackableType: "[etc]", StackLimit: 100},
	}}
	r := protocol.AvatarOptionRequest{
		ConsumableSlot: 70, ConsumableTemplate: rareAvatarOptionBoxTemplate, Paid: true,
		Location: 0, Slot: 1, Template: clone, Option: 2,
	}
	raw, err := s.SelectAvatarOption(role, r, items)
	if err != nil {
		t.Fatal(err)
	}
	if string(role.State) != string(original) {
		t.Fatal("selection mutated the caller's state")
	}
	saved, err := ReadBag(raw)
	if err != nil {
		t.Fatal(err)
	}
	if len(saved.Worn) != 1 || saved.Worn[0].Slot != 1 || saved.Worn[0].Template != clone || saved.Worn[0].Durability != 2 {
		t.Fatalf("worn rare Clone option = %+v", saved.Worn)
	}
	if len(saved.Items) != 2 || saved.Items[0].Slot != 70 || saved.Items[0].Template != rareAvatarOptionBoxTemplate || saved.Items[0].Amount != 3 || saved.Items[1] != (BagItem{Slot: 71, Template: 700, Amount: 8}) {
		t.Fatalf("consumable and unrelated items = %+v", saved.Items)
	}

	for _, invalid := range []protocol.AvatarOptionRequest{
		{ConsumableSlot: 70, ConsumableTemplate: 700, Paid: true, Location: 0, Slot: 1, Template: clone, Option: 2},
		{ConsumableSlot: 70, ConsumableTemplate: rareAvatarOptionBoxTemplate, Paid: true, Location: 0, Slot: 1, Template: clone, Option: 3},
		{ConsumableSlot: 99, ConsumableTemplate: rareAvatarOptionBoxTemplate, Paid: true, Location: 0, Slot: 1, Template: clone, Option: 2},
	} {
		if _, err := s.SelectAvatarOption(role, invalid, items); err == nil {
			t.Fatalf("invalid paid selection accepted: %+v", invalid)
		}
	}
}
