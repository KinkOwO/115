package workflow

import (
	"context"
	"dfolan/internal/catalog/pvf"
	"dfolan/internal/inventory"
	"dfolan/internal/loot"
	"strings"
	"testing"
)

func TestPreparedLegionEquipmentUsesSeparateSourceAndGrantsOnce(t *testing.T) {
	s, store, _ := legionRewardFixture()
	hash := s.Awarder.Catalog.Source.Checksum
	eq, err := inventory.NewEquipmentCatalog(inventory.EquipmentCatalog{Source: pvf.ArchiveSnapshot{Checksum: hash}, Rows: []inventory.EquipmentDefinition{
		{ID: 7301, Path: "coat.equ", SHA256: strings.Repeat("b", 64), Fields: map[string][]pvf.Token{
			"[rarity]": {{Type: 0, Value: 3}}, "[equipment type]": {{Type: 6, Text: "[coat]"}, {Type: 0, Value: 0}}, "[durability]": {{Type: 0, Value: 37}},
		}},
	}}, hash)
	if err != nil {
		t.Fatal(err)
	}
	s.Awarder.Equipment = eq
	s.Awarder.Rules.EquipmentSlots = [2]uint16{1, 2}
	draws := 0
	draw := func() ([]loot.Award, []loot.Award, error) {
		draws++
		return []loot.Award{{Template: 7301, Amount: 1}}, []loot.Award{{Template: 21, Amount: 8}}, nil
	}
	ctx := context.Background()
	role, plan, err := s.FreezePrepared(ctx, store.role, "forest:gear", "ForestOfAwakeningNormal", 100, draw)
	if err != nil {
		t.Fatal("valid EQU absent from stackable catalog rejected", err)
	}
	bag, _ := inventory.ReadBag(role.State)
	bag.Equipment = []inventory.BagEquipment{{Slot: 2, Template: 7301}, {Slot: 1, Template: 7301}}
	store.role.State, _ = inventory.SaveBag(role.State, bag)
	before := string(store.role.State)
	if _, _, err = s.Claim(ctx, role, plan.Run); err == nil || string(store.role.State) != before {
		t.Fatal("full equipment bag committed partial grant")
	}
	bag.Equipment = nil
	store.role.State, _ = inventory.SaveBag(store.role.State, bag)
	role, err = s.Recover(ctx, store.role)
	if err != nil {
		t.Fatal(err)
	}
	bag, _ = inventory.ReadBag(role.State)
	if len(bag.Equipment) != 1 || bag.Equipment[0].Template != 7301 || len(bag.Items) != 1 || bag.Items[0].Amount != 8 {
		t.Fatal("wrong recovered products", bag)
	}
	before = string(role.State)
	role, _, err = s.Claim(ctx, role, plan.Run)
	if err != nil || string(role.State) != before || draws != 1 {
		t.Fatal("duplicate reward", err)
	}
	eq.Source.Checksum = strings.Repeat("c", 64)
	if _, _, err = s.FreezePrepared(ctx, role, "foreign:gear", "ForestOfAwakeningNormal", 100, draw); err == nil {
		t.Fatal("foreign equipment source accepted")
	}
}
