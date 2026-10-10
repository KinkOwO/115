package inventory

import (
	"bytes"
	"dfolan/internal/catalog"
	"dfolan/internal/catalog/pvf"
	"dfolan/internal/game/protocol"
	"encoding/binary"
	"encoding/json"
	"strings"
	"testing"
)

func tokenRules() BagRules {
	return BagRules{Source: strings.Repeat("a", 64), MissingStackLimit: 1000, Slots: map[string][2]uint16{"[waste]": {65, 120}, "[material]": {121, 176}}}
}
func tokenEquipment() *EquipmentCatalog {
	return &EquipmentCatalog{index: map[uint32]EquipmentDefinition{100: {ID: 100, Fields: map[string][]pvf.Token{"[equipment type]": {{Type: 6, Text: "[weapon]"}}, "[usable job]": {{Type: 6, Text: "[swordman]"}}}}, 101: {ID: 101, Fields: map[string][]pvf.Token{"[equipment type]": {{Type: 6, Text: "[weapon]"}}, "[usable job]": {{Type: 6, Text: "[swordman]"}}}}}}
}

func TestWeaponShapeRefusalAndSourceCosts(t *testing.T) {
	b := Bag{Version: "ordinary-bag-v1", Equipment: []BagEquipment{{Slot: 9, Template: 100}}, Items: []BagItem{{Slot: 121, Template: 10, Amount: 2}, {Slot: 122, Template: 10, Amount: 2}}, WeaponSkin: 101}
	r := catalog.WeaponSkinRecipe{Mode: 9, Material: 10, Count: 3, Product: 20, Attach: "[free]", StackableType: "[waste]"}
	out, c, e := b.GenerateWeaponShape(9, tokenEquipment(), tokenRules(), r, ReplicationActor{Job: "[gunner]"})
	if e != nil || c.Product != 20 || c.Mold != 10 || len(out.Equipment) != 0 || len(out.Items) != 2 || out.Items[0].Amount != 1 || out.Items[1].WeaponShape != 100 || len(out.WeaponSkinStorage()) != 1 {
		t.Fatalf("source costs/product/legacy skin: %+v %+v %v", out, c, e)
	}
	// An unusable receiving character must keep the product for transfer.
	before, _ := json.Marshal(out)
	refused, _, e := out.RegisterWeaponShape(65, 20, tokenEquipment(), ReplicationActor{Job: "[gunner]"})
	after, _ := json.Marshal(refused)
	if e == nil || !bytes.Equal(before, after) {
		t.Fatal("foreign job ate product")
	}
	registered, skin, e := out.RegisterWeaponShape(65, 20, tokenEquipment(), ReplicationActor{Job: "[swordman]"})
	if e != nil || skin != 100 || len(registered.WeaponSkinStorage()) != 2 || registered.WeaponSkin != 101 {
		t.Fatal("registration lost legacy applied skin or auto-applied")
	}
	dup := out
	dup.WeaponSkins = []uint32{100}
	before, _ = json.Marshal(dup)
	refused, _, e = dup.RegisterWeaponShape(65, 20, tokenEquipment(), ReplicationActor{Job: "[swordman]"})
	after, _ = json.Marshal(refused)
	if e == nil || !bytes.Equal(before, after) {
		t.Fatal("duplicate skin consumed transfer product")
	}
	for _, name := range []string{"full", "missing-selected", "foreign-bound"} {
		t.Run(name, func(t *testing.T) {
			input := b
			recipe := r
			switch name {
			case "full":
				input.Items = append([]BagItem(nil), b.Items...)
				for s := uint16(65); s <= 120; s++ {
					input.Items = append(input.Items, BagItem{Slot: s, Template: 30, Amount: 1})
				}
			case "missing-selected":
				recipe.Material = 11
			case "foreign-bound":
				recipe.Attach = "[trade]"
			}
			before, _ := json.Marshal(input)
			result, _, e := input.GenerateWeaponShape(9, tokenEquipment(), tokenRules(), recipe, ReplicationActor{Job: "[gunner]"})
			after, _ := json.Marshal(result)
			if e == nil || !bytes.Equal(before, after) {
				t.Fatal("refusal charged weapon/material")
			}
		})
	}
}

func TestWeaponShapeMovesPreserveDistinctInstances(t *testing.T) {
	rules := tokenRules()
	c := catalog.LootCatalog{Source: pvf.ArchiveSnapshot{Checksum: rules.Source}, Items: map[uint32]catalog.LootItem{20: {ID: 20, Kind: "stackable", StackableType: "[waste]", StackLimit: 1000}}}
	b := Bag{Version: "ordinary-bag-v1", Items: []BagItem{{Slot: 65, Template: 20, Amount: 1, WeaponShape: 100}, {Slot: 66, Template: 20, Amount: 1, WeaponShape: 101}}}
	out, e := b.MoveStackRequest(c, rules, protocol.ItemMoveRequest{SourceList: 0, DestinationList: 0, SourceSlot: 65, DestinationSlot: 66, SourceItem: 20, DestinationItem: 20, Count: 1, Selection: 0xffffffff})
	if e != nil {
		t.Fatal(e)
	}
	a, _ := out.RowAt(65)
	z, _ := out.RowAt(66)
	if binary.LittleEndian.Uint32(a[14:]) != 101 || binary.LittleEndian.Uint32(z[14:]) != 100 {
		t.Fatal("same-template different-shape swap lost identity")
	}
	if len(ChangedItemRows(b, out)) != 2 {
		t.Fatal("same-template swap emitted no update")
	}
	v := Vault{Slots: 8, Items: []VaultItem{{Slot: 0, Template: 20, Amount: 1, WeaponShape: 101}}}
	out, v, n, e := MoveVaultItem(b, v, rules, protocol.ItemMoveRequest{SourceList: 0, SourceSlot: 65, DestinationList: 2, DestinationSlot: 0, Count: 1})
	if e != nil || n != 1 || len(v.Items) != 2 || v.ItemAt(0).WeaponShape != 101 || v.ItemAt(1).WeaponShape != 100 {
		t.Fatalf("vault merged different shapes: %+v %v", v, e)
	}
	raw, e := SaveVault(v)
	if e != nil {
		t.Fatal(e)
	}
	loaded, e := ReadExtendedVault(VaultState{Slots: 8, Items: raw})
	if e != nil {
		t.Fatal(e)
	}
	secondary := Vault{Slots: 8}
	loaded, secondary, n, e = MoveVaultCross(loaded, secondary, 1000, protocol.ItemMoveRequest{SourceList: 2, SourceSlot: 1, SourceItem: 20, DestinationList: 45, DestinationSlot: 0, Count: 1})
	if e != nil || n != 1 || secondary.ItemAt(0).WeaponShape != 100 {
		t.Fatal("cross-vault lost dynamic skin")
	}
	_ = loaded
	full := Vault{Slots: 1, Items: []VaultItem{{Slot: 0, Template: 20, Amount: 1, WeaponShape: 101}}}
	before, _ := json.Marshal(b)
	failed, _, _, e := MoveVaultItem(b, full, rules, protocol.ItemMoveRequest{SourceList: 0, SourceSlot: 65, DestinationList: 2, DestinationSlot: 0, Count: 1})
	if e == nil {
		t.Fatal("full vault merged two different weapons")
	}
	// Caller discards a failed operation's copy; original source remains intact.
	_ = failed
	after, _ := json.Marshal(b)
	if !bytes.Equal(before, after) {
		t.Fatal("failed vault operation mutated source")
	}
}

func TestWeaponShapeAccountVaultHonorsSourceBinding(t *testing.T) {
	for _, attach := range []string{"[free]", "[account]", "[trade]", "[character]"} {
		c := catalog.LootCatalog{Items: map[uint32]catalog.LootItem{20: {ID: 20, Kind: "stackable", Script: catalog.ScriptRecord{Cells: []pvf.Token{{Type: 3, Text: "[attach type]"}, {Type: 6, Text: attach}}}}}}
		e := accountVaultItemAllowed(VaultItem{Template: 20, Amount: 1, WeaponShape: 100}, c, nil)
		if (e == nil) != (attach == "[free]" || attach == "[account]") {
			t.Fatalf("source attachment %s: %v", attach, e)
		}
	}
}
