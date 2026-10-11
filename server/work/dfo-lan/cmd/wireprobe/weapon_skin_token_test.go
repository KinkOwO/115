package main

import (
	"bytes"
	"context"
	"dfolan/internal/catalog"
	"dfolan/internal/catalog/pvf"
	"dfolan/internal/character"
	"dfolan/internal/database"
	"dfolan/internal/game/protocol"
	"dfolan/internal/inventory"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

func shapeTestEquipment(t *testing.T) *inventory.EquipmentCatalog {
	t.Helper()
	sha := strings.Repeat("a", 64)
	rows := []inventory.EquipmentDefinition{}
	for _, id := range []uint32{101010438, 101010912} {
		rows = append(rows, inventory.EquipmentDefinition{ID: id, Path: fmt.Sprintf("equipment/%d.equ", id), SHA256: sha, Fields: map[string][]pvf.Token{"[equipment type]": {{Type: 6, Text: "[weapon]"}}, "[usable job]": {{Type: 6, Text: "[swordman]"}}}})
	}
	eq, e := inventory.NewEquipmentCatalog(inventory.EquipmentCatalog{Source: pvf.ArchiveSnapshot{Checksum: sha}, Rows: rows}, sha)
	if e != nil {
		t.Fatal(e)
	}
	return eq
}

func shapeTestRecipes() map[uint32]catalog.WeaponSkinRecipe {
	return map[uint32]catalog.WeaponSkinRecipe{1: {Mode: 1, Material: 10308357, Count: 1, Product: 10308358, Attach: "[free]", StackableType: "[waste]"}, 2: {Mode: 2, Material: 10308836, Count: 1, Product: 10308800, Attach: "[trade]", StackableType: "[waste]"}}
}

func shapeTestBagRules() inventory.BagRules {
	return inventory.BagRules{Source: strings.Repeat("a", 64), MissingStackLimit: 1000, Slots: map[string][2]uint16{"[waste]": {65, 120}, "[material]": {121, 176}}}
}

func TestWeaponShapeTwoStepCrossCharacterSQLite(t *testing.T) {
	ctx := context.Background()
	f, e := database.OpenTestFixture(ctx)
	if e != nil {
		t.Fatal(e)
	}
	defer f.Close()
	st := f.Storage()
	for _, migrate := range []func(context.Context) error{st.Migrate, st.MigrateCharacterEvents, st.MigrateAccountMaterials, st.MigrateAccountVault, st.MigrateSkinCargo} {
		if e = migrate(ctx); e != nil {
			t.Fatal(e)
		}
	}
	account, e := st.DevelopmentAccount(ctx, "shape-chain")
	if e != nil {
		t.Fatal(e)
	}
	chars, e := catalog.LoadCharacters("../../configs/characters.generated.json")
	if e != nil {
		t.Fatal(e)
	}
	svc := &character.Service{Store: st, Catalog: chars, Equipment: shapeTestEquipment(t), WeaponSkinRecipes: shapeTestRecipes(), WeaponSkinBagRules: shapeTestBagRules()}
	create := func(name string, b inventory.Bag) database.Character {
		b.Version = "ordinary-bag-v1"
		raw, e := inventory.SaveBag(json.RawMessage(`{"level":115,"future":{"keep":true}}`), b)
		if e != nil {
			t.Fatal(e)
		}
		c, e := st.CreateCharacter(ctx, database.Character{AccountID: account, Name: name, Profession: 0, ConfigVersion: strings.Repeat("a", 64), Request: []byte{0}, State: raw}, 24)
		if e != nil {
			t.Fatal(e)
		}
		return c
	}
	sender := create("sender", inventory.Bag{Equipment: []inventory.BagEquipment{{Slot: 41, Template: 101010438}, {Slot: 42, Template: 101010912}}, Items: []inventory.BagItem{{Slot: 168, Template: 10308357, Amount: 2}}, WeaponSkin: 101010438})
	receiver := create("receiver", inventory.Bag{})
	w := &worldSession{store: st, role: sender, characters: svc}
	forge := []byte{4, 1, 0, 0, 0, 0, 41, 0}
	plan, e := w.makeSkin(forge, func(map[string]any) {})
	if e != nil {
		t.Fatal(e)
	}
	if len(plan) != 2 || plan[0].ID != 13 || plan[1].ID != 1592 {
		t.Fatalf("forge must only emit inventory and completion: %+v", plan)
	}
	bag, _ := inventory.ReadBag(w.role.State)
	if len(bag.WeaponSkinStorage()) != 1 || bag.Items[1].WeaponShape != 101010438 || bag.Items[1].Template != 10308358 {
		t.Fatal("forge directly registered skin or lost product")
	}
	stale := sender
	replay, _, applied, e := svc.GenerateWeaponShape(ctx, stale, 41, 1)
	if e != nil || applied || !bytes.Equal(replay.State, w.role.State) {
		t.Fatalf("forge retry double-charged: %v", e)
	}
	forge[6] = 42
	if _, e = w.makeSkin(forge, func(map[string]any) {}); e != nil {
		t.Fatal(e)
	}
	bag, _ = inventory.ReadBag(w.role.State)
	if len(bag.Items) != 2 || bag.Items[0].WeaponShape != 101010438 || bag.Items[1].WeaponShape != 101010912 {
		t.Fatal("different weapons merged by product template")
	}
	itemCat := catalog.LootCatalog{Source: pvf.ArchiveSnapshot{Checksum: strings.Repeat("a", 64)}, Items: map[uint32]catalog.LootItem{10308358: {ID: 10308358, Kind: "stackable", StackableType: "[waste]", StackLimit: 1000, Script: catalog.ScriptRecord{Cells: []pvf.Token{{Type: 3, Text: "[attach type]"}, {Type: 6, Text: "[free]"}}}}}}
	// Persist a temporary shared vault, then move through the real DB transaction
	// with a second role on the same account. No production save is opened.
	_, _, _, _, e = st.CommitAccountVault(ctx, account, sender.ID, sender.ConfigVersion, "shape-test-open", 1, func(c database.Character, m json.RawMessage, v inventory.AccountVaultState) (json.RawMessage, json.RawMessage, inventory.AccountVaultState, error) {
		v.Slots = 8
		v.Items = json.RawMessage(`[]`)
		return c.State, m, v, nil
	})
	if e != nil {
		t.Fatal(e)
	}
	move := func(c database.Character, key string, r protocol.ItemMoveRequest) database.Character {
		saved, _, _, applied, e := st.CommitAccountVault(ctx, account, c.ID, c.ConfigVersion, key, 19, func(cur database.Character, m json.RawMessage, v inventory.AccountVaultState) (json.RawMessage, json.RawMessage, inventory.AccountVaultState, error) {
			state, next, _, e := inventory.MoveAccountVault(inventory.Role{State: cur.State}, v, shapeTestBagRules(), itemCat, svc.Equipment, r)
			return state, m, next, e
		})
		if e != nil || !applied {
			t.Fatalf("vault transaction: %v", e)
		}
		return saved
	}
	w.role = move(w.role, "shape-test-deposit", protocol.ItemMoveRequest{SourceList: 0, SourceSlot: 65, SourceItem: 10308358, DestinationList: 12, DestinationSlot: 0, Count: 1, Selection: 0xffffffff})
	receiver = move(receiver, "shape-test-withdraw", protocol.ItemMoveRequest{SourceList: 12, SourceSlot: 0, SourceItem: 10308358, DestinationList: 0, DestinationSlot: 65, Count: 1, Selection: 0xffffffff})
	got, _ := inventory.ReadBag(receiver.State)
	if len(got.Items) != 1 || got.Items[0].WeaponShape != 101010438 || len(got.WeaponSkinStorage()) != 0 {
		t.Fatal("shared vault changed weapon or registered before use")
	}
	row := got.Items[0].Row()
	if binary.LittleEndian.Uint32(row[14:]) != 101010438 {
		t.Fatal("native instance field missing after withdrawal")
	}
	w.role = receiver
	w.items = &inventory.ItemService{}
	w.skinCatalog = map[uint32]catalog.SkinStorageEntry{10308358: {Template: 10308358, SkinID: 40000, SkinType: "weapon skin"}}
	use := make([]byte, 64)
	binary.LittleEndian.PutUint16(use, 65)
	binary.LittleEndian.PutUint32(use[7:], 169)
	plan, e = w.useAddSkinStorage(use, func(map[string]any) {})
	if e != nil {
		t.Fatal(e)
	}
	if len(plan) != 3 || plan[0].ID != 14 || plan[1].ID != 1545 || plan[2].ID != 1547 {
		t.Fatalf("registration page push: %+v", plan)
	}
	got, _ = inventory.ReadBag(w.role.State)
	if len(got.Items) != 0 || len(got.WeaponSkinStorage()) != 1 || got.WeaponSkinStorage()[0] != 101010438 || got.WeaponSkin != 0 {
		t.Fatal("item use must register receiver only without auto-applying")
	}
	if _, _, applied, e = svc.RegisterWeaponShape(ctx, receiver, 65, 10308358); e != nil || applied {
		t.Fatalf("registration retry consumed twice: %v", e)
	}
	skins, e := st.ListSkins(ctx, account)
	if e != nil || len(skins) != 0 {
		t.Fatal("weapon shape leaked into account-global static 40000")
	}
	persisted, e := f.CharacterState(ctx, receiver.ID)
	if e != nil || !bytes.Equal(persisted, w.role.State) {
		t.Fatal("reconnect disagrees with registration")
	}
	if !bytes.Contains(persisted, []byte(`"future":{"keep":true}`)) {
		t.Fatal("future fields discarded")
	}
}
