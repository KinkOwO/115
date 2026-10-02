package main

import (
	"bytes"
	"context"
	"dfolan/internal/catalog"
	"dfolan/internal/catalog/pvf"
	"dfolan/internal/character"
	"dfolan/internal/dungeon"
	"dfolan/internal/game/protocol"
	"dfolan/internal/inventory"
	"dfolan/internal/storage"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"
)

func TestOathDirectEntryWithoutWorldStore(t *testing.T) {
	// Domain persistence no longer supplies the world session's oath reader.
	w := &worldSession{account: 1, role: storage.Character{ID: 1}, characters: &character.Service{Store: &storage.Store{}}}
	active, err := w.oathDirectEntryActive(context.Background())
	if err != nil || active {
		t.Fatalf("missing world store: active=%t err=%v", active, err)
	}
	packets, err := w.oathDirectEntryPackets(context.Background())
	if err != nil || len(packets) != 0 {
		t.Fatalf("missing world store: packets=%v err=%v", packets, err)
	}
}

// Use a dedicated PostgreSQL DSN and a disposable schema. This checks real
// persisted selections, packet timing and unchanged saves, without player data.
func TestOathDirectEntryIntegration(t *testing.T) {
	dsn := os.Getenv("DFO_TEST_POSTGRES_DSN")
	if dsn == "" {
		t.Skip("DFO_TEST_POSTGRES_DSN requires a dedicated PostgreSQL test database")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	cfg := storage.Config{PostgresDSN: dsn, MaxConnections: 2}
	admin, err := storage.Open(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer admin.Close()
	cfg.PostgresSchema = fmt.Sprintf("oath_entry_%d", time.Now().UnixNano())
	if _, err := admin.DB.Exec(ctx, "CREATE SCHEMA "+cfg.PostgresSchema); err != nil {
		t.Fatal(err)
	}
	defer func() {
		if _, err := admin.DB.Exec(context.Background(), "DROP SCHEMA "+cfg.PostgresSchema+" CASCADE"); err != nil {
			t.Error(err)
		}
	}()
	store, err := storage.Open(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	for _, migrate := range []func(context.Context) error{store.Migrate, store.MigrateOathOptions} {
		if err := migrate(ctx); err != nil {
			t.Fatal(err)
		}
	}
	account, err := store.DevelopmentAccount(ctx, "oath-entry-fixture")
	if err != nil {
		t.Fatal(err)
	}
	version := strings.Repeat("a", 64)
	equipment, err := inventory.NewEquipmentCatalog(inventory.EquipmentCatalog{
		Source: pvf.ArchiveSnapshot{Checksum: version},
		Rows: []inventory.EquipmentDefinition{
			{ID: 517500000, Path: "clone.equ", SHA256: version, Fields: map[string][]pvf.Token{"[item category]": {{Type: 6, Text: "clear avatar"}}}},
			{ID: 112500000, Path: "cover.equ", SHA256: version},
			{ID: 101000013, Path: "weapon.equ", SHA256: version},
			{ID: 100610059, Path: "oath.equ", SHA256: version},
			{ID: 100401626, Path: "crystal.equ", SHA256: version},
		},
	}, version)
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name        string
		clone, oath bool
	}{
		{"clone-oath", true, true},
		{"ordinary-oath", false, true},
		{"clone-no-oath", true, false},
		{"ordinary-no-oath", false, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			weapon := inventory.BagEquipment{Slot: 12, Template: 101000013}
			row := inventory.EquipmentRow(weapon)
			row[60] = 0x7a // Preserve instance data, not just its template.
			weapon.Record = append([]byte(nil), row[:]...)
			worn := []inventory.BagEquipment{weapon}
			if tc.clone {
				worn = append(worn, inventory.BagEquipment{Slot: 3, Template: 517500000}, inventory.BagEquipment{Slot: 3, Group: 1, Template: 112500000})
			}
			if tc.oath {
				worn = append(worn, inventory.BagEquipment{Slot: 47, Template: 100610059})
				for slot := uint16(36); slot <= 46; slot++ {
					worn = append(worn, inventory.BagEquipment{Slot: slot, Template: 100401626})
				}
			}
			raw := json.RawMessage(fmt.Sprintf(`{"level":115,"source_sha256":%q,"attributes":{"[hp max]":100,"[mp max]":100},"future":{"keep":true}}`, version))
			raw, err = inventory.SaveBag(raw, inventory.Bag{Version: "ordinary-bag-v1", Worn: worn})
			if err != nil {
				t.Fatal(err)
			}
			role, err := store.CreateCharacter(ctx, storage.Character{AccountID: account, Name: tc.name, Profession: 11, ConfigVersion: version, Request: []byte{0}, State: raw}, 24)
			if err != nil {
				t.Fatal(err)
			}
			if tc.oath {
				if _, err := store.SelectEquippedOathOption(ctx, account, role.ID, 100610059, 3); err != nil {
					t.Fatal(err)
				}
			}
			var before json.RawMessage
			if err := store.DB.QueryRow(ctx, "SELECT state FROM characters WHERE id=$1", role.ID).Scan(&before); err != nil {
				t.Fatal(err)
			}
			for _, dungeonID := range []uint32{22, 5000} {
				t.Run(fmt.Sprintf("dungeon-%d", dungeonID), func(t *testing.T) {
					s := &dungeon.Session{Definition: catalog.DungeonDefinition{ID: dungeonID, NoFatigue: dungeonID == 5000}, Room: catalog.DungeonRoom{Map: 36250}}
					w := &worldSession{account: account, store: store, role: role, activeDungeon: s, characters: &character.Service{DetailedWornCandidate: true, Equipment: equipment}}
					entry, err := w.dungeonEntryPlan(ctx, "dungeon_select_ack", 16, protocol.DungeonSelection{ID: dungeonID}, s)
					if err != nil {
						t.Fatal(err)
					}
					loading, err := w.finishDungeonLoading(make([]byte, 16))
					if err != nil {
						t.Fatal(err)
					}
					index := func(plan []outboundPacket, name string) int {
						found := -1
						for i, p := range plan {
							if p.Name == name {
								if found >= 0 {
									t.Fatalf("duplicate %s", name)
								}
								found = i
							}
						}
						return found
					}
					active := tc.oath
					if active {
						names := []string{"dungeon_worn_equipment_direct", "dungeon_worn_visuals_direct"}
						if tc.clone {
							names = append(names, "dungeon_clone_detached_pre_direct", "dungeon_clone_reattached_pre_direct", "dungeon_nonavatar_worn_restored_pre_direct")
						}
						names = append(names, "dungeon_oath_selection_restored", "dungeon_start_map_sent")
						previous := -1
						for _, name := range names {
							i := index(entry, name)
							if i <= previous {
								t.Fatalf("entry %s at %d, preceding packet at %d", name, i, previous)
							}
							previous = i
						}
						wantWorn, err := inventory.WornPayload(role.State)
						if err != nil || !bytes.Equal(entry[index(entry, "dungeon_worn_equipment_direct")].Payload, wantWorn) {
							t.Fatalf("pre-map worn snapshot changed: %v", err)
						}
						for _, p := range loading {
							if p.ID == 2 || p.ID == 13 || p.ID == 14 || p.ID == 2839 {
								t.Fatalf("loading repeats actor/equipment/oath packet: %s", p.Name)
							}
						}
					} else {
						if index(entry, "dungeon_worn_equipment_direct") >= 0 || index(entry, "dungeon_clone_detached_pre_direct") >= 0 {
							t.Fatal("inactive entry used direct oath or Clone reconstruction")
						}
						if index(loading, "dungeon_worn_equipment_restored") < 0 {
							t.Fatal("fallback lost worn restoration")
						}
						if tc.clone {
							previous := -1
							for _, name := range []string{"dungeon_clone_detached", "dungeon_clone_reattached", "dungeon_nonavatar_worn_restored", "dungeon_oath_selection_restored"} {
								i := index(loading, name)
								if i <= previous {
									t.Fatalf("fallback %s at %d, preceding packet at %d", name, i, previous)
								}
								previous = i
							}
						}
					}
					for _, plan := range [][]outboundPacket{entry, loading} {
						for _, p := range plan {
							if p.ID == 2839 && (len(p.Payload) != 24 || (tc.oath && binary.LittleEndian.Uint32(p.Payload) != 3)) {
								t.Fatalf("persisted option lost: %x", p.Payload)
							}
							if strings.Contains(p.Name, "nonavatar_worn_restored") {
								want, err := inventory.NonAvatarWornSpaceUpdate(role.State)
								if err != nil || !bytes.Equal(p.Payload, want) {
									t.Fatalf("non-avatar instance rows changed: %v", err)
								}
							}
						}
					}
					if len(loading) == 0 || loading[len(loading)-1].ID != 1361 {
						t.Fatal("buff registration must follow actor reconstruction")
					}
					if !bytes.Equal(w.role.State, role.State) {
						t.Fatal("packet planning modified session save")
					}
				})
			}
			var after json.RawMessage
			if err := store.DB.QueryRow(ctx, "SELECT state FROM characters WHERE id=$1", role.ID).Scan(&after); err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(before, after) {
				t.Fatal("packet planning modified persisted save")
			}
		})
	}
}
