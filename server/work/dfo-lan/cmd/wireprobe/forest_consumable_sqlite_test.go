//go:build sqlite

package main

import (
	"context"
	"dfolan/internal/catalog"
	"dfolan/internal/catalog/pvf"
	"dfolan/internal/database"
	"dfolan/internal/dungeon"
	"dfolan/internal/inventory"
	"dfolan/internal/legion"
	"encoding/binary"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
)

func TestForestPotionSQLiteFailureReplayAndSourceCap(t *testing.T) {
	ctx := context.Background()
	store, err := database.Open(ctx, database.Config{Driver: database.DriverSQLite, SQLitePath: filepath.Join(t.TempDir(), "forest.sqlite")})
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	for _, migrate := range []func(context.Context) error{store.Migrate, store.MigrateCharacterEvents, store.MigrateGrants, store.MigratePremiums} {
		if err := migrate(ctx); err != nil {
			t.Fatal(err)
		}
	}
	account, err := store.DevelopmentAccount(ctx, "forest-potion-fixture")
	if err != nil {
		t.Fatal(err)
	}
	source := pvf.ArchiveSnapshot{Checksum: strings.Repeat("a", 64)}
	state, err := inventory.SaveBag(json.RawMessage(`{"opaque":{"keep":17}}`), inventory.Bag{Version: "ordinary-bag-v1", Items: []inventory.BagItem{{Slot: 65, Template: 700, Amount: 6}, {Slot: 66, Template: 701, Amount: 2}}})
	if err != nil {
		t.Fatal(err)
	}
	role, err := store.CreateCharacter(ctx, database.Character{AccountID: account, Name: "ForestQuota", ConfigVersion: source.SaveIdentity(), State: state, Request: []byte{0, 255}}, 24)
	if err != nil {
		t.Fatal(err)
	}
	id := legion.ForestStageDungeons[0]
	w := &worldSession{store: store, role: role, forest: &forestRun{stage: 0}, activeDungeon: &dungeon.Session{Loaded: true, Definition: catalog.DungeonDefinition{ID: id}}, items: &inventory.ItemService{Model: "forest-potion-fixture", Catalog: catalog.LootCatalog{Source: source, Items: map[uint32]catalog.LootItem{700: {Kind: "stackable", StackableType: "[etc]", StackLimit: 100}, 701: {Kind: "stackable", StackableType: "[etc]", StackLimit: 100}}}}, dungeons: &catalog.DungeonCatalog{StackableLimits: &catalog.StackableDungeonLimits{Potions: map[uint32]bool{700: true}, Dungeons: map[uint32]uint32{id: 2}}}}
	use := func(slot uint16, template, sequence uint32) error {
		payload := make([]byte, 16)
		binary.LittleEndian.PutUint16(payload, slot)
		binary.LittleEndian.PutUint32(payload[3:], sequence)
		binary.LittleEndian.PutUint32(payload[7:], template)
		_, err := w.useStackable(payload, func(map[string]any) {})
		return err
	}
	if err := use(99, 700, 1); err == nil || w.forest.potionUsed[0] != 0 {
		t.Fatal("failed use spent quota", err)
	}
	for _, sequence := range []uint32{10, 11, 10, 11} {
		if err := use(65, 700, sequence); err != nil {
			t.Fatal(err)
		}
	}
	if w.forest.potionUsed[0] != 2 {
		t.Fatal("replay spent quota", w.forest.potionUsed)
	}
	before := string(w.role.State)
	if err := use(65, 700, 12); err == nil || string(w.role.State) != before {
		t.Fatal("source cap ignored", err)
	}
	if err := use(66, 701, 13); err != nil || w.forest.potionUsed[0] != 2 {
		t.Fatal("non-potion counted", err)
	}
	// Changing the source limit affects the next transaction without Go tables.
	w.dungeons.StackableLimits.Dungeons[id] = 3
	if err := use(65, 700, 14); err != nil || w.forest.potionUsed[0] != 3 {
		t.Fatal("source change ignored", err)
	}
	bag, err := inventory.ReadBag(w.role.State)
	if err != nil || bag.Items[0].Amount != 3 || !strings.Contains(string(w.role.State), `"keep":17`) {
		t.Fatal("inventory or unrelated state changed", bag, err)
	}
}
