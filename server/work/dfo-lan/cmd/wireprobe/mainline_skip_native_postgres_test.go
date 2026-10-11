package main

import (
	"context"
	"dfolan/internal/catalog"
	"dfolan/internal/character"
	"dfolan/internal/database"
	"dfolan/internal/gamedata"
	"dfolan/internal/inventory"
	"dfolan/internal/quest"
	"dfolan/internal/workflow"
	"encoding/json"
	"os"
	"testing"
	"time"
)

// The actual local PVF runs against a separate test cluster. No player data,
// game listener or client process is used by this integration test.
func TestMainlineSkipCurrentPVFPostgres(t *testing.T) {
	path := os.Getenv("DFO_HANDOFF_PVF")
	if path == "" || os.Getenv("DFO_TEST_POSTGRES_DSN") == "" {
		t.Skip("requires current PVF and dedicated test PostgreSQL")
	}
	c, err := gamedata.PrepareCatalogs(gamedata.CatalogInputs{Selection: "items,equipment,quests,progression,characters", ArchivePath: path, CharacterPolicyPath: "../../configs/pvf-character-policy.json"}, gamedata.CatalogAdapters{})
	if err != nil {
		t.Fatal(err)
	}
	defer c.Quests.Close()
	defer c.Equipment.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	fixture, err := database.OpenTestFixture(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer fixture.Close()
	store := fixture.Storage()
	for _, m := range []func(context.Context) error{store.Migrate, store.MigrateQuests, store.MigrateQuestObjectives, store.MigrateQuestRewards, store.MigrateCharacterEvents} {
		if err = m(ctx); err != nil {
			t.Fatal(err)
		}
	}
	account, err := store.DevelopmentAccount(ctx, "native-mainline")
	if err != nil {
		t.Fatal(err)
	}
	version := c.Quests.Source.SaveIdentity()
	// Profession selection comes from this PVF's imported character catalog.
	var profession byte
	for id, p := range c.Characters.Professions {
		if p.Job == "[archer]" {
			profession = id
			break
		}
	}
	if profession == 0 {
		t.Fatal("native archer profession absent")
	}
	state, err := inventory.SaveBag(json.RawMessage(`{"level":115,"advancement":3,"awakening":2,"unknown":{"keep":true}}`), inventory.Bag{Version: "ordinary-bag-v1", Gold: 12345})
	if err != nil {
		t.Fatal(err)
	}
	role, err := store.CreateCharacter(ctx, database.Character{AccountID: account, Name: "NativeSkip", Profession: profession, ConfigVersion: version, Request: []byte{0}, State: state}, 24)
	if err != nil {
		t.Fatal(err)
	}
	rules, err := inventory.LoadBagRules("../../configs/inventory.next29.json", c.SourceChecksum)
	if err != nil {
		t.Fatal(err)
	}
	items := catalogLootForMainline(c)
	eq := &inventory.EquipmentCatalog{Source: c.Items.Source, Full: c.Equipment}
	growth := character.GrowthRules{Model: "native-mainline-test-v1"}
	service := &quest.Service{Store: store, Catalog: *c.Quests, Skip: c.QuestSkip, Professions: *c.Characters, Inventory: &inventory.Awarder{Catalog: items, Equipment: eq, Rules: rules}, Progression: &character.ProgressionService{Catalog: *c.Progression, Rules: growth}}
	result, err := (&workflow.QuestService{Store: store, Quest: service}).SkipMainlineNow(ctx, role, "native-test", "source verification", 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Settled) == 0 || len(result.KeyItems) != len(c.QuestSkip.Items) {
		t.Fatalf("no native settlement or keys: %d %v", len(result.Settled), result.KeyItems)
	}
	index := service.Index()
	paid := []uint16{}
	for _, r := range result.Settled {
		if c.QuestSkip.Blocked[uint32(r.Receipt.Quest)] {
			t.Fatalf("blacklisted quest %d skipped", r.Receipt.Quest)
		}
		if r.Receipt.Gold != 0 || r.Receipt.Experience != 0 {
			t.Fatal("skip paid currency or XP")
		}
		if len(r.Receipt.Items) > 0 {
			if !index.Entries[uint32(r.Receipt.Quest)].PaysOnForceClear {
				t.Fatal("reward outside native marker")
			}
			paid = append(paid, r.Receipt.Quest)
		}
	}
	repeat, err := (&workflow.QuestService{Store: store, Quest: service}).SkipMainlineNow(ctx, role, "native-test", "replay", 0)
	if err != nil || len(repeat.Settled) > 0 || len(repeat.KeyItems) > 0 {
		t.Fatalf("repeat native settlement %v %v", repeat, err)
	}
	t.Logf("source=%s settled=%d paid=%v keys=%v stopped=%s blocked=%v", c.SourceChecksum, len(result.Settled), paid, result.KeyItems, result.Stopped, result.Blocked)
}

func catalogLootForMainline(c *gamedata.Catalogs) catalog.LootCatalog {
	items := catalog.LootCatalog{Source: c.Items.Source, Items: map[uint32]catalog.LootItem{}}
	for id, row := range c.Items.Items {
		items.Items[id] = catalog.LootItem{ID: id, Kind: row.Kind, StackableType: row.StackableType, StackLimit: row.StackLimit}
	}
	return items
}
