package main

import (
	"context"
	"dfolan/internal/catalog"
	"dfolan/internal/catalog/pvf"
	"dfolan/internal/database"
	"dfolan/internal/inventory"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"strings"
	"testing"
)

func TestEquipmentTransformNetSoulsPersistsAcrossSQLiteReopen(t *testing.T) {
	ctx := context.Background()
	f, err := database.OpenTestFixture(ctx)
	must115(t, err)
	defer f.Close()
	store := f.Storage()
	for _, migrate := range []func(context.Context) error{store.Migrate, store.MigrateCharacterEvents, store.MigrateAccountMaterials, store.MigrateAccountVault} {
		must115(t, migrate(ctx))
	}
	account, err := store.DevelopmentAccount(ctx, "transform-net-cost-fixture")
	must115(t, err)
	source := pvf.ArchiveSnapshot{Checksum: strings.Repeat("d", 64)}
	var defs []inventory.EquipmentDefinition
	for _, id := range []uint32{700, 701, 800, 801} {
		d := primerDef(id, 2)
		d.Fields["[grade]"] = []pvf.Token{{Type: 0, Value: 115}}
		d.Fields["[equipment type]"] = []pvf.Token{{Type: 6, Text: "[coat]"}}
		d.Fields["[durability]"] = []pvf.Token{{Type: 0, Value: 60}}
		defs = append(defs, d)
	}
	eq, err := inventory.NewEquipmentCatalog(inventory.EquipmentCatalog{Source: source, Rows: defs}, source.Checksum)
	must115(t, err)
	table, err := catalog.ParseEquipmentTransformSystem(primerTransformFlowTable)
	must115(t, err)
	items := &inventory.ItemService{Catalog: catalog.LootCatalog{Source: source}, Equipment: eq,
		Journal: &catalog.EquipmentJournalRules{Maximum: 99}, CreateCost: &catalog.EquipmentCreateCost{Source: source}, Transform: &table}
	state, err := inventory.SaveBag(json.RawMessage(`{"level":115,"advancement":0,"future":{"keep":42}}`), inventory.Bag{
		Version: "ordinary-bag-v1", Gold: 100_000, Worn: []inventory.BagEquipment{{Slot: 14, Template: 700, Refine: 4}, {Slot: 15, Template: 701, Refine: 5}}})
	must115(t, err)
	state, err = inventory.SaveEquipmentJournal(state, inventory.EquipmentJournal{Counts: map[uint32]uint32{800: 1, 801: 1}})
	must115(t, err)
	role, err := store.CreateCharacter(ctx, database.Character{AccountID: account, Name: "transformnet", ConfigVersion: source.SaveIdentity(), Request: []byte{0}, State: state}, 24)
	must115(t, err)
	w := &worldSession{account: account, store: store, role: role, items: items}
	p, err := hex.DecodeString(craftNativeUserFrame)
	must115(t, err)
	for i, id := range []uint32{800, 801} {
		offset := 17 + (2+i)*7
		p[offset] = 3
		p[offset+1] = byte(14 + i)
		p[offset+2] = 0
		binary.LittleEndian.PutUint32(p[offset+3:], id)
	}
	packets, err := w.equipmentCraft(p, nil)
	must115(t, err)
	bag, err := inventory.ReadBag(w.role.State)
	must115(t, err)
	if bag.Gold != 50_000 || bag.Worn[0].Template != 800 || bag.Worn[1].Template != 801 || bag.Worn[0].Refine != 4 || bag.Worn[1].Refine != 5 {
		t.Fatalf("bag=%+v", bag)
	}
	for _, name := range []string{"account_materials_restored", "radiant_souls_restored", "equipment_transform_worn_refreshed", "equipment_transform_journal_refreshed"} {
		found := false
		for _, packet := range packets {
			found = found || packet.Name == name
		}
		if !found {
			t.Fatalf("missing %s", name)
		}
	}
	before := string(w.role.State)
	_, err = w.equipmentCraft(p, nil)
	must115(t, err)
	if string(w.role.State) != before {
		t.Fatal("replay changed character assets")
	}
	store.Close()
	reopened, err := f.Reopen(ctx)
	must115(t, err)
	defer reopened.Close()
	reloaded, err := reopened.AdminCharacter(ctx, role.ID)
	must115(t, err)
	if string(reloaded.State) != before {
		t.Fatal("persisted character reverted after reopening SQLite")
	}
	var saved map[string]json.RawMessage
	must115(t, json.Unmarshal(reloaded.State, &saved))
	if string(saved["future"]) != `{"keep":42}` {
		t.Fatal("unrelated save fields changed")
	}
	raw, err := reopened.AccountMaterials(ctx, account)
	must115(t, err)
	mats, err := inventory.ReadAccountMaterials(raw)
	must115(t, err)
	if mats.Count(10361512) != 0 {
		t.Fatal("same-rarity swap minted souls")
	}
	journal, err := inventory.ReadEquipmentJournal(reloaded.State)
	must115(t, err)
	if journal.Counts[800] != 0 || journal.Counts[801] != 0 || journal.Counts[700] != 1 || journal.Counts[701] != 1 {
		t.Fatalf("journal=%v", journal.Counts)
	}
}
