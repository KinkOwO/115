package main

import (
	"bytes"
	"context"
	"dfolan/internal/game/protocol"
	"dfolan/internal/gamedata"
	"dfolan/internal/inventory"
	"dfolan/internal/storage"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"
)

// Uses current native PVF rules and an explicitly selected disposable database.
func TestUnsealNativeSaveIdentity(t *testing.T) {
	configPath, archive := os.Getenv("DFO_TEST_STORAGE_CONFIG"), os.Getenv("DFO_PVF_CORE_TEST_ARCHIVE")
	if configPath == "" || archive == "" {
		t.Skip("set DFO_TEST_STORAGE_CONFIG and DFO_PVF_CORE_TEST_ARCHIVE for isolated native unseal regression")
	}
	source, err := gamedata.Open(gamedata.Options{Mode: gamedata.PVF, ArchivePath: archive, ExpectedChecksum: os.Getenv("DFO_PVF_CORE_TEST_SHA256")})
	if err != nil {
		t.Fatal(err)
	}
	defer source.Close()
	index, err := source.ItemIndex("")
	if err != nil {
		t.Fatal(err)
	}
	full, err := source.Equipment(index)
	if err != nil {
		t.Fatal(err)
	}
	defer full.Close()
	data, err := source.RandomOptions()
	if err != nil {
		t.Fatal(err)
	}
	options, err := inventory.NewRandomOptionCatalog(data, source.Snapshot().Checksum)
	if err != nil {
		t.Fatal(err)
	}
	if source.Snapshot().Checksum == source.Snapshot().SaveIdentity() {
		t.Fatal("native source must differ from save identity")
	}
	cfg, err := storage.LoadConfig(configPath)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	admin, err := storage.Open(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer admin.Close()
	schema := fmt.Sprintf("unseal_test_%d", time.Now().UnixNano())
	if _, err = admin.DB.Exec(ctx, "CREATE SCHEMA "+schema); err != nil {
		t.Fatal(err)
	}
	defer func() {
		if _, err := admin.DB.Exec(context.Background(), "DROP SCHEMA "+schema+" CASCADE"); err != nil {
			t.Error(err)
		}
	}()
	cfg.PostgresSchema, cfg.RedisPrefix = schema, schema+":"
	store, err := storage.Open(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	for _, migrate := range []func(context.Context) error{store.Migrate, store.MigrateCharacterEvents} {
		if err := migrate(ctx); err != nil {
			t.Fatal(err)
		}
	}
	account, err := store.DevelopmentAccount(ctx, "unseal-fixture")
	if err != nil {
		t.Fatal(err)
	}
	service := &inventory.UnsealService{Store: store, Equipment: &inventory.EquipmentCatalog{Full: full}, RandomOptions: options, Model: "current115-randomoption-v1"}
	request := []byte{10, 0, 255, 255, 0, 0, 0, 0} // Native/live slot-10 CMD393 vector.
	for i, version := range []string{source.Snapshot().SaveIdentity(), source.Snapshot().Checksum} {
		t.Run(fmt.Sprintf("identity_%d", i), func(t *testing.T) {
			bag := inventory.Bag{Version: "ordinary-bag-v1", Gold: 1234, Equipment: []inventory.BagEquipment{{Slot: 10, Template: 100310840}, {Slot: 11, Template: 100310581}}}
			state, err := inventory.SaveBag(json.RawMessage(`{"unrelated":{"keep":true}}`), bag)
			if err != nil {
				t.Fatal(err)
			}
			role, err := store.CreateCharacter(ctx, storage.Character{AccountID: account, Name: fmt.Sprintf("UnsealFixture%d", i), Request: []byte{0}, ConfigVersion: version, State: state}, 24)
			if err != nil {
				t.Fatal(err)
			}
			if i == 0 {
				_, _, _, err = service.Unseal(ctx, role, source.Snapshot().Checksum, protocol.UnsealRequest{TargetSlot: 10, ScrollSlot: protocol.UnsealNoScrollSlot})
				if err == nil || !strings.Contains(err.Error(), "character event source mismatch") {
					t.Fatalf("old call did not reproduce live refusal: %v", err)
				}
			}
			w := worldSession{role: role}
			packets, _, err := w.unsealRandomOption(service, request)
			if err != nil {
				t.Fatal(err)
			}
			if len(packets) != 2 || packets[0].Kind != 1 || packets[0].ID != 393 || !bytes.Equal(packets[0].Payload, []byte{1}) || packets[1].Kind != 0 || packets[1].ID != 14 {
				t.Fatalf("unseal ACK/update: %+v", packets)
			}
			updated, err := inventory.ReadBag(w.role.State)
			if err != nil || updated.Gold != bag.Gold || len(updated.Equipment) != 2 || inventory.EquipmentRow(updated.Equipment[1]) != inventory.EquipmentRow(bag.Equipment[1]) {
				t.Fatalf("unrelated inventory changed: %+v %v", updated, err)
			}
			row := inventory.EquipmentRow(updated.Equipment[0])
			if inventory.RandomOptionBlockEmpty(row) || len(updated.Equipment[0].Record) != protocol.CurrentItemRecordSize {
				t.Fatal("unseal did not persist random options")
			}
			want, err := protocol.InventoryUpdate([][protocol.CurrentItemRecordSize]byte{row})
			if err != nil || !bytes.Equal(packets[1].Payload, want) {
				t.Fatal("inventory update differs from persisted record", err)
			}
			roles, err := store.Characters(ctx, account)
			if err != nil {
				t.Fatal(err)
			}
			var reloaded storage.Character
			for _, saved := range roles {
				if saved.ID == role.ID {
					reloaded = saved
				}
			}
			var fields map[string]json.RawMessage
			if err := json.Unmarshal(reloaded.State, &fields); err != nil || !bytes.Contains(fields["unrelated"], []byte("true")) || reloaded.ConfigVersion != version {
				t.Fatal("save identity or unrelated state changed", err)
			}
			restored, err := inventory.ReadBag(reloaded.State)
			if err != nil || inventory.EquipmentRow(restored.Equipment[0]) != row {
				t.Fatal("options changed on reload", err)
			}
			// A stale session retries the exact pre-image and must replay its roll.
			w.role = role
			replayed, _, err := w.unsealRandomOption(service, request)
			if err != nil || len(replayed) != 2 || !bytes.Equal(replayed[1].Payload, want) {
				t.Fatal("stale retry rerolled or failed", err)
			}
			if _, _, err := w.unsealRandomOption(service, request); err == nil {
				t.Fatal("already unsealed item rerolled")
			}
			var events int
			if err := store.DB.QueryRow(ctx, "SELECT count(*) FROM character_events WHERE character_id=$1", role.ID).Scan(&events); err != nil || events != 1 {
				t.Fatalf("event count %d: %v", events, err)
			}
		})
	}
}
