package main

import (
	"bytes"
	"context"
	"dfolan/internal/catalog"
	"dfolan/internal/catalog/pvf"
	"dfolan/internal/character"
	"dfolan/internal/database"
	"dfolan/internal/dungeon"
	"dfolan/internal/game/protocol"
	"dfolan/internal/inventory"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"
)

// Exercise the actual loading planner with disposable persisted characters.
// The worn weapon keeps its instance bytes; native CMD1565 changes the look.
func TestWeaponSkinSurvivesDungeonReconstruction(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	fixture, err := database.OpenTestFixture(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer fixture.Close()
	store := fixture.Storage()
	for _, migrate := range []func(context.Context) error{store.Migrate, store.MigrateOathOptions} {
		if err := migrate(ctx); err != nil {
			t.Fatal(err)
		}
	}
	account, err := store.DevelopmentAccount(ctx, "weapon-loading-fixture")
	if err != nil {
		t.Fatal(err)
	}
	version := strings.Repeat("a", 64)
	equipment, err := inventory.NewEquipmentCatalog(inventory.EquipmentCatalog{
		Source: pvf.ArchiveSnapshot{Checksum: version},
		Rows: []inventory.EquipmentDefinition{
			{ID: 517500000, Path: "clone.equ", SHA256: version, Fields: map[string][]pvf.Token{"[item category]": {{Type: 6, Text: "clear avatar"}}}},
			{ID: 112500000, Path: "cover.equ", SHA256: version},
			{ID: 100610059, Path: "oath.equ", SHA256: version},
			{ID: 100401626, Path: "crystal.equ", SHA256: version},
			{ID: 101010438, Path: "worn-weapon.equ", SHA256: version},
			{ID: 101010912, Path: "skin-weapon.equ", SHA256: version},
		},
	}, version)
	if err != nil {
		t.Fatal(err)
	}
	for _, clone := range []bool{false, true} {
		for _, oath := range []bool{false, true} {
			for _, skin := range []uint32{0, 101010912} {
				name := fmt.Sprintf("clone-%t/oath-%t/skin-%d", clone, oath, skin)
				t.Run(name, func(t *testing.T) {
					weapon := inventory.BagEquipment{Slot: inventory.WeaponSlot, Template: 101010438, Durability: 7}
					row := inventory.EquipmentRow(weapon)
					row[10], row[14], row[60] = 12, 99, 0x7a
					weapon.Record = append([]byte(nil), row[:]...)
					worn := []inventory.BagEquipment{weapon}
					if clone {
						worn = append(worn, inventory.BagEquipment{Slot: 3, Template: 517500000}, inventory.BagEquipment{Slot: 3, Group: 1, Template: 112500000})
					}
					if oath {
						worn = append(worn, inventory.BagEquipment{Slot: 47, Template: 100610059})
						for slot := uint16(36); slot <= 46; slot++ {
							worn = append(worn, inventory.BagEquipment{Slot: slot, Template: 100401626})
						}
					}
					raw := json.RawMessage(fmt.Sprintf(`{"level":115,"source_sha256":%q,"attributes":{"[hp max]":100,"[mp max]":100},"future":{"keep":true}}`, version))
					raw, err = inventory.SaveBag(raw, inventory.Bag{Version: "ordinary-bag-v1", Worn: worn, WeaponSkin: skin, WeaponSkins: []uint32{101010912},
						TitleBook: map[byte][]inventory.TitleBookEntry{4: {{Equipment: inventory.BagEquipment{Slot: 10, Template: 786000583}}}},
					})
					if err != nil {
						t.Fatal(err)
					}
					role, err := store.CreateCharacter(ctx, database.Character{AccountID: account, Name: fmt.Sprintf("weapon%d", skin+uint32(len(worn))), Profession: 11, ConfigVersion: version, Request: []byte{0}, State: raw}, 24)
					if err != nil {
						t.Fatal(err)
					}
					if oath {
						if _, err := store.SelectEquippedOathOption(ctx, account, role.ID, 100610059, 3); err != nil {
							t.Fatal(err)
						}
					}
					before, err := fixture.CharacterState(ctx, role.ID)
					if err != nil {
						t.Fatal(err)
					}
					run := &dungeon.Session{Definition: catalog.DungeonDefinition{ID: 22}, Room: catalog.DungeonRoom{Map: 36250}}
					w := &worldSession{account: account, store: store, role: role, activeDungeon: run, characters: &character.Service{DetailedWornCandidate: true, Equipment: equipment}}
					if _, err := w.dungeonEntryPlan(ctx, "dungeon_select_ack", 16, protocol.DungeonSelection{ID: 22}, run); err != nil {
						t.Fatal(err)
					}
					for room := 0; room < 2; room++ {
						plan, err := w.finishDungeonLoading(make([]byte, 16))
						if err != nil {
							t.Fatal(err)
						}
						found, lastRebuild := -1, -1
						for i, p := range plan {
							if p.Kind == 0 && (p.ID == 2 || p.ID == 13 || p.ID == 14 || p.ID == 30) {
								lastRebuild = i
							}
							if p.Kind == 1 && p.ID == 1565 {
								if found >= 0 || !bytes.Equal(p.Payload, protocol.WeaponSkinSelectionEcho(skin)) {
									t.Fatalf("duplicate or wrong weapon selection: %x", p.Payload)
								}
								found = i
							}
						}
						if skin != 0 && (found < 0 || found <= lastRebuild) {
							t.Fatalf("room %d: weapon selection at %d, last reconstruction at %d", room, found, lastRebuild)
						}
						if skin == 0 && found >= 0 {
							t.Fatal("unselected weapon unexpectedly replayed")
						}
						wantWorn, err := inventory.WornPayload(role.State)
						if err != nil {
							t.Fatal(err)
						}
						for _, p := range plan {
							if p.Name == "dungeon_worn_equipment_restored" && !bytes.Equal(p.Payload, wantWorn) {
								t.Fatal("worn weapon instance was replaced by cosmetic")
							}
							if oath && p.Kind == 0 && (p.ID == 2 || p.ID == 13) {
								t.Fatal("direct entry repeated actor/worn rebuild")
							}
						}
						if !bytes.Equal(w.role.State, role.State) {
							t.Fatal("loading changed session save")
						}
						run.Room.Map++
					}
					after, err := fixture.CharacterState(ctx, role.ID)
					if err != nil || !bytes.Equal(before, after) {
						t.Fatalf("loading changed persisted save: %v", err)
					}
				})
			}
		}
	}
}
