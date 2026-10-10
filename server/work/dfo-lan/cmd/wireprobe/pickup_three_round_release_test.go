package main

import (
	"bytes"
	"context"
	"dfolan/internal/catalog"
	"dfolan/internal/catalog/pvf"
	"dfolan/internal/database"
	"dfolan/internal/dungeon"
	"dfolan/internal/inventory"
	"dfolan/internal/loot"
	"dfolan/internal/workflow"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

// Exercises the durable gateway path with a full ordinary tab, a real CMD18
// discard and CMD43 retry for three consecutive full-bag rounds. Only a private SQLite is opened.
func TestPickupFullBagDiscardAndRetryThreeRounds(t *testing.T) {
	for _, variant := range []struct {
		equipment bool
		expansion byte
	}{{false, 0}, {true, 0}, {false, 1}, {true, 1}, {false, 2}, {true, 2}} {
		t.Run(fmt.Sprintf("equipment=%t/expansion=%d", variant.equipment, variant.expansion), func(t *testing.T) {
			equipment := variant.equipment
			ctx := context.Background()
			f, err := database.OpenTestFixture(ctx)
			must115(t, err)
			defer f.Close()
			store := f.Storage()
			must115(t, store.Migrate(ctx))
			must115(t, store.MigrateCharacterEvents(ctx))
			account, err := store.DevelopmentAccount(ctx, "pickup-full-fixture")
			must115(t, err)
			source := pvf.ArchiveSnapshot{Checksum: strings.Repeat("b", 64)}
			const template uint32 = 700
			c := catalog.LootCatalog{Source: source, Items: map[uint32]catalog.LootItem{
				template: {ID: template, Kind: "stackable", StackableType: "[throw]", StackLimit: 1},
			}}
			rules := inventory.BagRules{EquipmentSlots: [2]uint16{9, 64}, Slots: map[string][2]uint16{"[throw]": {65, 120}}, MissingStackLimit: 1}
			bag := inventory.Bag{Version: "ordinary-bag-v1", Expansion: variant.expansion}
			first := uint16(65)
			var eq *inventory.EquipmentCatalog
			if equipment {
				first = 9
				def := primerDef(template, 2)
				def.Fields["[equipment type]"] = []pvf.Token{{Type: 6, Text: "[coat]"}}
				def.Fields["[durability]"] = []pvf.Token{{Type: 0, Value: 60}}
				eq, err = inventory.NewEquipmentCatalog(inventory.EquipmentCatalog{Source: source, Rows: []inventory.EquipmentDefinition{def}}, source.Checksum)
				must115(t, err)
				c.Items[template] = catalog.LootItem{ID: template, Kind: "equipment"}
			}
			for slot := first; slot < first+40+8*uint16(variant.expansion); slot++ {
				if equipment {
					bag.Equipment = append(bag.Equipment, inventory.BagEquipment{Slot: slot, Template: template, Durability: 60})
				} else {
					bag.Items = append(bag.Items, inventory.BagItem{Slot: slot, Template: template, Amount: 1})
				}
			}
			state, err := inventory.SaveBag(json.RawMessage(`{"level":115,"future":{"keep":42}}`), bag)
			must115(t, err)
			role, err := store.CreateCharacter(ctx, database.Character{AccountID: account, Name: "fullbag", ConfigVersion: source.SaveIdentity(), State: state, Request: []byte{0}}, 24)
			must115(t, err)
			run := &dungeon.Session{RunID: strings.Repeat("a", 32), Loaded: true, Room: catalog.DungeonRoom{Map: 1}}
			ls := &loot.Service{Catalog: c, BagRules: rules, Equipment: eq, Rules: loot.Rules{Model: "reference90-gold-stack-v1", MaximumPickupX: 100, MaximumPickupY: 100}}
			drops := loot.NewSession(c, loot.Tables{}, ls.Rules, nil, run.RunID, account, role.ID, role.WireID)
			for n := range 3 {
				object := uint32(4096 + n)
				drops.Objects[object] = loot.Drop{Run: run.RunID, Map: 1, Owner: role.WireID, Object: object, Award: loot.Award{Template: template, Amount: 1}}
			}
			for round := range 3 {
				object := uint32(4096 + round)
				state = append([]byte(nil), role.State...)
				w := &worldSession{account: account, store: store, role: role, loot: ls, vault: &workflow.VaultService{}, activeDungeon: run, drops: drops}
				// All remaining objects were already on the ground before the
				// first full-bag failure. Refuse each before freeing one slot.
				for n := round; n < 3; n++ {
					pending := make([]byte, 24)
					binary.LittleEndian.PutUint32(pending, uint32(4096+n))
					if _, err := w.pickup(pending); err == nil {
						t.Fatal("full bag accepted a remaining object")
					}
					reply, err := w.pickupRefusal(pending)
					must115(t, err)
					if binary.LittleEndian.Uint32(reply[3:]) != template {
						t.Fatal("remaining object has an invalid release index")
					}
				}
				request := make([]byte, 24)
				binary.LittleEndian.PutUint32(request, object)
				if _, err := w.pickup(request); err == nil {
					t.Fatal("full tab accepted pickup")
				}
				refusal, err := w.pickupRefusal(request)
				must115(t, err)
				if len(refusal) != 7 || !bytes.Equal(refusal[:3], []byte{0, 4, 0}) || binary.LittleEndian.Uint32(refusal[3:]) != template {
					t.Fatal("failure must release the item template, not the scene object", refusal)
				}
				unknown := append([]byte(nil), request...)
				binary.LittleEndian.PutUint32(unknown, object+100)
				refusal, err = w.pickupRefusal(unknown)
				must115(t, err)
				if binary.LittleEndian.Uint32(refusal[3:]) != 0 {
					t.Fatal("unknown object leaked an unverified item index")
				}
				before, err := store.AdminCharacter(ctx, role.ID)
				must115(t, err)
				if !bytes.Equal(before.State, state) {
					t.Fatal("failed pickup changed saved bag")
				}
				if _, err := store.CharacterEventReceipt(ctx, account, role.ID, fmt.Sprintf("pickup:%s:%d", run.RunID, object)); err == nil {
					t.Fatal("failed pickup wrote success receipt")
				}
				entry := binary.AppendUvarint([]byte{8, 1, 16}, uint64(first))
				entry = binary.AppendUvarint(append(entry, 24), uint64(template))
				entry = append(entry, 32, 1)
				body := binary.AppendUvarint([]byte{16, 0, 26}, uint64(len(entry)))
				body = append(append(body, entry...), 32, 0)
				deleted := binary.LittleEndian.AppendUint32(nil, uint32(len(body)))
				deleted = append(deleted, body...)
				_, err = w.deleteItems(deleted, []byte(fmt.Sprintf("unique-discard-request-%d", round)))
				must115(t, err)
				packets, err := w.pickup(request)
				must115(t, err)
				removed := 0
				for _, packet := range packets {
					if packet.Name == "pickup_scene_removed" {
						removed++
					}
				}
				if removed != 1 {
					t.Fatal("retry did not remove exactly one ground object")
				}
				committed := append([]byte(nil), w.role.State...)
				packets, err = w.pickup(request)
				must115(t, err)
				if !bytes.Equal(w.role.State, committed) {
					t.Fatal("replayed pickup duplicated reward")
				}
				for _, packet := range packets {
					if packet.Name == "pickup_scene_removed" {
						t.Fatal("replay removed the object twice")
					}
				}
				b, err := inventory.ReadBag(w.role.State)
				must115(t, err)
				row, ok := b.RowAt(first)
				if !ok || binary.LittleEndian.Uint32(row[2:]) != template {
					t.Fatal("discarded slot was not reused")
				}
				if !bytes.Contains(w.role.State, []byte(`"keep":42`)) {
					t.Fatal("unrelated state lost")
				}
				role = w.role
			}
		})
	}
}

func TestPickupRefusalSeparatesTemplatesAndRejectsUnownedObjects(t *testing.T) {
	run := &dungeon.Session{RunID: strings.Repeat("a", 32), Loaded: true, Room: catalog.DungeonRoom{Map: 1}}
	drops := loot.NewSession(catalog.LootCatalog{}, loot.Tables{}, loot.Rules{}, nil, run.RunID, 1, 2, 3)
	w := &worldSession{account: 1, role: database.Character{ID: 2, WireID: 3}, activeDungeon: run, drops: drops}
	for _, row := range []struct {
		object, template uint32
		owner            uint16
		mapID            uint32
		expected         uint32
	}{{4096, 700, 3, 1, 700}, {4097, 701, 3, 1, 701}, {4098, 702, 4, 1, 0}, {4099, 703, 3, 2, 0}} {
		drops.Objects[row.object] = loot.Drop{Run: run.RunID, Map: row.mapID, Owner: row.owner, Object: row.object, Award: loot.Award{Template: row.template, Amount: 1}}
		request := make([]byte, 24)
		binary.LittleEndian.PutUint32(request, row.object)
		reply, err := w.pickupRefusal(request)
		must115(t, err)
		if binary.LittleEndian.Uint32(reply[3:]) != row.expected {
			t.Fatalf("object %d release index = %x", row.object, reply)
		}
	}
}
