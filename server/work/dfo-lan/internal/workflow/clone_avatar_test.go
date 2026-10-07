package workflow

import (
	"bytes"
	"context"
	"crypto/sha256"
	"dfolan/internal/catalog/pvf"
	"dfolan/internal/database"
	"dfolan/internal/game/protocol"
	"dfolan/internal/inventory"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCloneAvatarMigrationTransactionAndGuardedRollback(t *testing.T) {
	for _, driver := range []string{database.DriverSQLite, database.DriverPostgres} {
		t.Run(driver, func(t *testing.T) {
			ctx := context.Background()
			var store *database.Store
			var err error
			if driver == database.DriverPostgres {
				if os.Getenv("DFO_TEST_POSTGRES_DSN") == "" {
					t.Skip("dedicated PostgreSQL test DSN required")
				}
				fixture, err := database.OpenTestFixture(ctx)
				require.NoError(t, err)
				t.Cleanup(func() {
					if err := fixture.Close(); err != nil {
						t.Error(err)
					}
				})
				store = fixture.Storage()
			} else {
				store, err = database.Open(ctx, database.Config{Driver: driver, SQLitePath: filepath.Join(t.TempDir(), "clone.sqlite")})
				require.NoError(t, err)
				t.Cleanup(store.Close)
			}
			for _, migrate := range []func(context.Context) error{store.Migrate, store.MigrateCharacterEvents} {
				if err := migrate(ctx); err != nil {
					t.Fatal(err)
				}
			}
			version := strings.Repeat("a", 64)
			defs := inventory.EquipmentCatalog{Source: pvf.ArchiveSnapshot{Checksum: version}, Rows: []inventory.EquipmentDefinition{
				{ID: 517560000, Path: "clone.equ", SHA256: version, Fields: map[string][]pvf.Token{"[equipment type]": {{Type: 6, Text: "[hair avatar]"}}, "[item category]": {{Type: 6, Text: "clear avatar"}}}},
				{ID: 517562678, Path: "look.equ", SHA256: version, Fields: map[string][]pvf.Token{"[equipment type]": {{Type: 6, Text: "[hair avatar]"}}}},
			}}
			path := filepath.Join(t.TempDir(), "equipment.json")
			raw, err := json.Marshal(defs)
			require.NoError(t, err)
			require.NoError(t, os.WriteFile(path, raw, 0600))
			equipment, err := inventory.LoadEquipmentCatalog(path, version)
			require.NoError(t, err)
			record := make([]byte, protocol.CurrentItemRecordSize)
			binary.LittleEndian.PutUint32(record[2:], 517562678)
			record[85] = 1
			record[32] = 77
			look := inventory.BagEquipment{Slot: 1, Template: 517562678, Group: 1, Record: record, AvatarOptions: []byte{3, 4}, AvatarSockets: []byte{5, 6}, Period: 12345, Durability: 17, Refine: 2, ExtraFields: map[string]json.RawMessage{"future_instance": json.RawMessage(`{"serial":9007199254740993}`)}}
			before, err := inventory.SaveBag(json.RawMessage(`{"level":115,"future_role":{"keep":true},"inventory":{"version":"ordinary-bag-v1","future_inventory":{"keep":true}}}`), inventory.Bag{Version: "ordinary-bag-v1", Worn: []inventory.BagEquipment{{Slot: 1, Template: 517560000}, look}})
			require.NoError(t, err)
			account, err := store.DevelopmentAccount(ctx, "clone-migration-fixture")
			require.NoError(t, err)
			role, err := store.CreateCharacter(ctx, database.Character{AccountID: account, Name: "clone-migration", Profession: 0, ConfigVersion: equipment.Source.SaveIdentity(), State: before, Request: []byte{}}, 24)
			require.NoError(t, err)
			s := &WearService{Store: store, WearService: inventory.WearService{Catalog: equipment}}
			saved, applied, err := s.MigrateCloneAvatars(ctx, role)
			if err != nil || !applied {
				t.Fatalf("migration: %t %v", applied, err)
			}
			bag, err := inventory.ReadBag(saved.State)
			require.NoError(t, err)
			want := look
			want.Slot = 0
			want.Group = 0
			if len(bag.Worn) != 1 || len(bag.Special[1]) != 1 || !reflect.DeepEqual(bag.Special[1][0], want) {
				t.Fatal("transaction did not retain the complete source instance")
			}
			if !bytes.Contains(saved.State, []byte("future_inventory")) || !bytes.Contains(saved.State, []byte("future_role")) {
				t.Fatal("migration lost opaque state")
			}
			if _, applied, err = s.MigrateCloneAvatars(ctx, saved); err != nil || applied {
				t.Fatal("migration replay moved items again")
			}
			key := fmt.Sprintf("clone-avatar-native-v1:%x", sha256.Sum256(role.State))
			// A later player action is never overwritten by a rollback receipt.
			later, _, err := store.CommitCharacterEvent(ctx, saved.AccountID, saved.ID, saved.ConfigVersion, "later-player-action", "test-clone-later", func(current database.Character) (json.RawMessage, json.RawMessage, error) {
				b, err := inventory.ReadBag(current.State)
				if err != nil {
					return nil, nil, err
				}
				b.Gold++
				state, err := inventory.SaveBag(current.State, b)
				return state, json.RawMessage(`{}`), err
			})
			require.NoError(t, err)
			if _, _, err = s.RollbackCloneAvatarMigration(ctx, later, key); err == nil {
				t.Fatal("rollback overwrote post-migration player change")
			}
			// Fresh second character: exact untouched after-state is reversible.
			second, err := store.CreateCharacter(ctx, database.Character{AccountID: account, Name: "clone-undo", Profession: 0, ConfigVersion: equipment.Source.SaveIdentity(), State: before, Request: []byte{}}, 24)
			require.NoError(t, err)
			migrated, _, err := s.MigrateCloneAvatars(ctx, second)
			require.NoError(t, err)
			undoKey := fmt.Sprintf("clone-avatar-native-v1:%x", sha256.Sum256(second.State))
			undone, applied, err := s.RollbackCloneAvatarMigration(ctx, migrated, undoKey)
			if err != nil || !applied {
				t.Fatalf("undo: %t %v", applied, err)
			}
			oldBag, err := inventory.ReadBag(second.State)
			require.NoError(t, err)
			undoBag, err := inventory.ReadBag(undone.State)
			require.NoError(t, err)
			oldJSON, err := json.Marshal(oldBag)
			require.NoError(t, err)
			undoJSON, err := json.Marshal(undoBag)
			require.NoError(t, err)
			require.Equal(t, oldJSON, undoJSON, "undo lost original dual-Worn data")
			foreign := role
			foreign.AccountID++
			if _, _, err = s.MigrateCloneAvatars(ctx, foreign); err == nil {
				t.Fatal("foreign ownership accepted")
			}
		})
	}
}
