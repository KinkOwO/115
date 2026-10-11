//go:build sqlite

package workflow

import (
	"context"
	"dfolan/internal/catalog/pvf"
	"dfolan/internal/database"
	"dfolan/internal/inventory"
	"dfolan/internal/loot"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
)

func TestPreparedLegionEquipmentSQLiteReconnectAndReplay(t *testing.T) {
	ctx := context.Background()
	db, err := database.Open(ctx, database.Config{Driver: database.DriverSQLite, SQLitePath: filepath.Join(t.TempDir(), "forest-gear.sqlite")})
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err = db.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	account, err := db.DevelopmentAccount(ctx, "forest-gear-fixture")
	if err != nil {
		t.Fatal(err)
	}
	source := pvf.ArchiveSnapshot{Checksum: strings.Repeat("a", 64)}
	role, err := db.CreateCharacter(ctx, database.Character{AccountID: account, Name: "ForestGear", ConfigVersion: source.SaveIdentity(), State: json.RawMessage(`{"unrelated":{"keep":17}}`), Request: []byte{}}, 24)
	if err != nil {
		t.Fatal(err)
	}
	service, _, _ := legionRewardFixture()
	service.Store = db
	eq, err := inventory.NewEquipmentCatalog(inventory.EquipmentCatalog{Source: source, Rows: []inventory.EquipmentDefinition{{ID: 7311, Path: "gear.equ", SHA256: strings.Repeat("b", 64), Fields: map[string][]pvf.Token{
		"[rarity]": {{Type: 0, Value: 2}}, "[equipment type]": {{Type: 6, Text: "[coat]"}, {Type: 0, Value: 0}}, "[durability]": {{Type: 0, Value: 37}},
	}}}}, source.Checksum)
	if err != nil {
		t.Fatal(err)
	}
	service.Awarder.Equipment = eq
	service.Awarder.Rules.EquipmentSlots = [2]uint16{1, 2}
	draw := func() ([]loot.Award, []loot.Award, error) {
		return []loot.Award{{Template: 7311, Amount: 1}}, []loot.Award{{Template: 21, Amount: 8}}, nil
	}
	saved, frozen, err := service.FreezePrepared(ctx, role, "forest:sqlite", "ForestOfAwakeningNormal", 100, draw)
	if err != nil {
		t.Fatal(err)
	}
	// A new service instance restores the durable plan and cannot reroll it.
	second := *service
	restored, _, err := second.FreezePrepared(ctx, saved, frozen.Run, frozen.Content, frozen.Dungeon, func() ([]loot.Award, []loot.Award, error) { t.Fatal("durable plan rerolled"); return nil, nil, nil })
	if err != nil {
		t.Fatal(err)
	}
	restored, err = second.Recover(ctx, restored)
	if err != nil {
		t.Fatal(err)
	}
	bag, err := inventory.ReadBag(restored.State)
	if err != nil || len(bag.Equipment) != 1 || bag.Equipment[0].Template != 7311 || len(bag.Items) != 1 || bag.Items[0].Amount != 8 {
		t.Fatal("wrong durable reward", err, bag)
	}
	before := string(restored.State)
	again, _, err := second.Claim(ctx, restored, frozen.Run)
	if err != nil || string(again.State) != before || !strings.Contains(before, `"keep":17`) {
		t.Fatal("replay/opaque state changed", err)
	}
}
