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
// The worn title keeps its instance bytes; only CMD560 selects the book look.
func TestCumulativeTruthWeaponAndTitleSurviveDungeonReconstruction(t *testing.T) {
	rules, err := character.CurrentFameRules()
	if err != nil {
		t.Fatal(err)
	}
	fixtureRules := *rules
	fixtureRules.OathReinforcement = map[byte]int64{0: 0, 4: 1600}
	restoreFame, err := character.InstallFameRules(&fixtureRules)
	if err != nil {
		t.Fatal(err)
	}
	defer restoreFame()
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
	account, err := store.DevelopmentAccount(ctx, "title-dungeon-fixture")
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
			{ID: 786000585, Path: "worn-title.equ", SHA256: version},
			{ID: 101010912, Path: "weapon.equ", SHA256: version},
			{ID: 786000583, Path: "book-title.equ", SHA256: version},
		},
	}, version)
	if err != nil {
		t.Fatal(err)
	}
	for _, clone := range []bool{false, true} {
		for _, oath := range []bool{false, true} {
			for _, skin := range []uint32{0, 786000583} {
				name := fmt.Sprintf("clone-%t/oath-%t/skin-%d", clone, oath, skin)
				t.Run(name, func(t *testing.T) {
					title := inventory.BagEquipment{Slot: 13, Template: 786000585, Durability: 7}
					row := inventory.EquipmentRow(title)
					row[10], row[14], row[60] = 12, 99, 0x7a
					title.Record = append([]byte(nil), row[:]...)
					worn := []inventory.BagEquipment{title, {Slot: 12, Template: 101010912, Durability: 37}}
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
					raw, err = inventory.SaveBag(raw, inventory.Bag{Version: "ordinary-bag-v1", Worn: worn, TitleSkin: skin, TitleBookSeq: 4, WeaponSkin: 101010912, WeaponSkins: []uint32{101010912},
						TitleBook: map[byte][]inventory.TitleBookEntry{4: {{Equipment: inventory.BagEquipment{Slot: 10, Template: 786000583}}}},
					})
					if err != nil {
						t.Fatal(err)
					}
					raw, err = inventory.SaveOathTruth(raw, inventory.OathTruthState{Level: 4, Options: [10]byte{0, 0, 0, 0, 1}})
					if err != nil {
						t.Fatal(err)
					}
					role, err := store.CreateCharacter(ctx, database.Character{AccountID: account, Name: fmt.Sprintf("title%d", skin+uint32(len(worn))), Profession: 11, ConfigVersion: version, Request: []byte{0}, State: raw}, 24)
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
					entry, err := w.dungeonEntryPlan(ctx, "dungeon_select_ack", 16, protocol.DungeonSelection{ID: 22}, run)
					if err != nil {
						t.Fatal(err)
					}
					for room := 0; room < 2; room++ {
						plan, err := w.finishDungeonLoading(make([]byte, 16))
						if err != nil {
							t.Fatal(err)
						}
						truthSeen := false
						for _, packet := range append(append([]outboundPacket(nil), entry...), plan...) {
							if packet.ID == 2839 && packet.Kind == 0 {
								truthSeen = true
								if len(packet.Payload) < 15 || packet.Payload[4] != 4 || packet.Payload[9] != 1 {
									t.Fatal("truth state lost during combined reconstruction")
								}
							}
						}
						if !truthSeen {
							t.Fatal("missing persisted truth projection")
						}
						weaponRestores := 0
						for _, packet := range plan {
							if packet.Kind == 1 && packet.ID == 1565 {
								weaponRestores++
								if !bytes.Equal(packet.Payload, protocol.WeaponSkinSelectionEcho(101010912)) {
									t.Fatal("wrong cumulative weapon restore")
								}
							}
						}
						if weaponRestores != 1 {
							t.Fatalf("weapon restore count %d", weaponRestores)
						}
						for _, p := range plan {
							if p.Kind == 1 && p.ID == 560 {
								t.Fatal("title replayed before native actor readiness")
							}
						}
						ready, err := w.dungeonTitleActorReady(2423, make([]byte, 16), true)
						if err != nil {
							t.Fatal(err)
						}
						if skin == 0 && len(ready) != 0 {
							t.Fatal("unselected title replayed")
						}
						if skin != 0 && (len(ready) != 1 || ready[0].Kind != 1 || ready[0].ID != 560 || !bytes.Equal(ready[0].Payload, protocol.TitleAppearanceSuccess(skin))) {
							t.Fatalf("room %d: wrong ready replay %v", room, ready)
						}
						duplicate, err := w.dungeonTitleActorReady(2423, make([]byte, 16), true)
						if err != nil || len(duplicate) != 0 {
							t.Fatalf("duplicate readiness replay: %v %v", duplicate, err)
						}
						wantWorn, err := inventory.WornPayload(role.State)
						if err != nil {
							t.Fatal(err)
						}
						for _, p := range plan {
							if p.Name == "dungeon_worn_equipment_restored" && !bytes.Equal(p.Payload, wantWorn) {
								t.Fatal("worn title instance was replaced by cosmetic")
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
					w.state = database.WorldState{Position: database.WorldPosition{Town: 38, Area: 2, X: 150, Y: 249}}
					w.titleRestoreCharacter = role.ID
					returned, err := w.leaveDungeon()
					if err != nil {
						t.Fatal(err)
					}
					lastRebuild, restored := -1, -1
					for i, p := range returned {
						if p.Kind == 0 && (p.ID == 2 || p.ID == 13 || p.ID == 14) {
							lastRebuild = i
						}
						if p.Kind == 1 && p.ID == 560 {
							if restored >= 0 || !bytes.Equal(p.Payload, protocol.TitleAppearanceSuccess(skin)) {
								t.Fatal("wrong town selection replay")
							}
							restored = i
						}
					}
					if (skin != 0 && (restored < 0 || restored <= lastRebuild)) || (skin == 0 && restored >= 0) || w.titleRestoreCharacter != 0 {
						t.Fatalf("town restoration at %d after %d; pending=%d", restored, lastRebuild, w.titleRestoreCharacter)
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
