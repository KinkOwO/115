package main

import (
	"context"
	"dfolan/internal/boostup"
	"dfolan/internal/catalog"
	"dfolan/internal/catalog/pvf"
	"dfolan/internal/database"
	"dfolan/internal/game/protocol"
	"dfolan/internal/inventory"
	"dfolan/internal/loot"
	"encoding/hex"
	"encoding/json"
	"strings"
	"testing"
)

const craftNativeUserFrame = "0000000000000000b22d6f29b9010000002e0000ffffffff2e0000ffffffff030e0055a9f6052e0000ffffffff2e0000ffffffff2e0000ffffffff2e0000ffffffff2e0000ffffffff2e0000ffffffff2e0000ffffffff2e0000ffffffff2e0000ffffffff2e0000ffffffff2e0000ffffffff00010000000000000000000000"

func TestCraftFingerprintIgnoresNativeStackResidue(t *testing.T) {
	p, e := hex.DecodeString(craftNativeUserFrame)
	must115(t, e)
	r, e := protocol.DecodeEquipmentCraftRequest(p)
	must115(t, e)
	slots, templates := r.Wanted()
	first := craftFingerprint(r, slots, templates)
	for i := 0; i < 13; i++ {
		p[i] = 254
	}
	r, e = protocol.DecodeEquipmentCraftRequest(p)
	must115(t, e)
	if craftFingerprint(r, slots, templates) != first {
		t.Fatal("stack residue changed confirmation")
	}
	r.PayOption = 2
	if craftFingerprint(r, slots, templates) == first {
		t.Fatal("different payment matched confirmation")
	}
}

// Native request -> real SQLite transaction -> inventory/journal/progress packets.
// The fixture is private; it never opens the player's database.
func TestCraftNativeTransformCommitsAndAdvancesTutorial(t *testing.T) {
	ctx := context.Background()
	fixture, e := database.OpenTestFixture(ctx)
	must115(t, e)
	defer fixture.Close()
	store := fixture.Storage()
	for _, migrate := range []func(context.Context) error{store.Migrate, store.MigrateCharacterEvents, store.MigrateAccountMaterials, store.MigrateAccountVault} {
		must115(t, migrate(ctx))
	}
	account, e := store.DevelopmentAccount(ctx, "craft-native-fixture")
	must115(t, e)
	version := strings.Repeat("c", 64)
	source := pvf.ArchiveSnapshot{Checksum: version}
	const old, target = uint32(100051282), uint32(100051285)
	var defs []inventory.EquipmentDefinition
	for _, id := range []uint32{old, target} {
		d := primerDef(id, 2)
		d.Fields["[equipment type]"] = []pvf.Token{{Type: 6, Text: "[coat]"}}
		d.Fields["[durability]"] = []pvf.Token{{Type: 0, Value: 40}}
		defs = append(defs, d)
	}
	eq, e := inventory.NewEquipmentCatalog(inventory.EquipmentCatalog{Source: source, Rows: defs}, version)
	must115(t, e)
	rules := catalog.EquipmentJournalRules{Maximum: 99}
	transformSource := strings.ReplaceAll(primerTransformFlowTable, "0 25000", "0 25")
	transformSource = strings.ReplaceAll(transformSource, "    10361512 1\n", "")
	transform, e := catalog.ParseEquipmentTransformSystem(transformSource)
	must115(t, e)
	items := &inventory.ItemService{Catalog: catalog.LootCatalog{Source: source}, Equipment: eq, Journal: &rules,
		CreateCost: &catalog.EquipmentCreateCost{Source: source}, Transform: &transform,
		WearRules: inventory.WearRules{Slots: map[string]uint16{"[coat]": 14}}}
	c := &boostup.Catalog{Town: 222, Groups: boostTestGroups{target: {77}}, Steps: make([]boostup.Step, 11)}
	for i := range c.Steps {
		c.Steps[i] = boostup.Step{Number: byte(i + 1), Guide: "normal", Mission: "none"}
	}
	c.Steps[9].Mission = "transform equip journal or equip item"
	c.Steps[9].MissionCells = []pvf.Token{{Type: 3, Text: "[equip grouping]"}, {Type: 3, Text: "[count]"}, {Type: 0, Value: 1}, {Type: 3, Text: "[index]"}, {Type: 0, Value: 77}, {Type: 3, Text: "[/equip grouping]"}}
	for _, inBag := range []bool{false, true} {
		bag := inventory.Bag{Version: "ordinary-bag-v1", Gold: 100}
		if inBag {
			bag.Equipment = []inventory.BagEquipment{{Slot: 7, Template: old, Refine: 3}}
		} else {
			bag.Worn = []inventory.BagEquipment{{Slot: 14, Template: old, Refine: 3}}
		}
		raw, e := inventory.SaveBag(json.RawMessage(`{"level":115,"advancement":0,"unrelated":42}`), bag)
		must115(t, e)
		raw, e = inventory.SaveEquipmentJournal(raw, inventory.EquipmentJournal{Counts: map[uint32]uint32{target: 1}})
		must115(t, e)
		raw, e = boostup.WriteState(raw, boostup.State{Version: 1, Activated: true, Training: boostup.Training{Step: 10, Phase: 2, Claimed: map[byte]bool{10: true}}})
		must115(t, e)
		name := "craftworn"
		if inBag {
			name = "craftbag"
		}
		role, e := store.CreateCharacter(ctx, database.Character{AccountID: account, Name: name, Profession: 0, ConfigVersion: source.SaveIdentity(), Request: []byte{0}, State: raw}, 24)
		must115(t, e)
		w := &worldSession{account: account, store: store, role: role, items: items, boostup: c, loot: &loot.Service{Catalog: items.Catalog, Equipment: eq}}
		p, e := hex.DecodeString(craftNativeUserFrame)
		must115(t, e)
		if inBag {
			// The native tuple is space:u8 + source slot:u16.
			p[31], p[32], p[33] = 0, 7, 0
		}
		packets, e := w.equipmentCraft(p, nil)
		must115(t, e)
		got, e := inventory.ReadBag(w.role.State)
		must115(t, e)
		if len(got.Worn) != 1 || got.Worn[0].Template != target || got.Worn[0].Refine != 3 || len(got.Equipment) != 0 || got.Gold != 75 {
			t.Fatalf("bag=%+v inBag=%v", got, inBag)
		}
		ledger, e := inventory.ReadEquipmentJournal(w.role.State)
		must115(t, e)
		if ledger.Counts[target] != 0 || ledger.Counts[old] != 1 {
			t.Fatalf("journal=%v", ledger.Counts)
		}
		st, e := boostup.ReadState(w.role.State)
		must115(t, e)
		if st.Training.Step != 11 || !st.Training.Claimed[10] {
			t.Fatalf("training=%+v", st.Training)
		}
		have := map[uint16]bool{}
		for _, p := range packets {
			have[p.ID] = true
		}
		for _, id := range []uint16{2259, 14, 13, 2610, 2638} {
			if !have[id] {
				t.Fatalf("missing packet %d", id)
			}
		}
		before := string(w.role.State)
		_, e = w.equipmentCraft(p, nil)
		must115(t, e)
		if string(w.role.State) != before {
			t.Fatal("duplicate request changed assets/progress")
		}
	}
}
