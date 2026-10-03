package main

import (
	"dfolan/internal/gamedata"
	"strings"
	"testing"
)

func TestRemainingContentPathsRefusedBeforeStorage(t *testing.T) {
	for _, row := range []struct {
		domain string
		set    func(*Config)
	}{
		{"town", func(c *Config) { c.TownCatalog = "old.json" }},
		{"journal", func(c *Config) { c.EquipmentJournalRules = "old.json" }},
		{"create-cost", func(c *Config) { c.EquipmentCreateCost = "old.json" }},
		{"tutorial", func(c *Config) { c.TutorialRoutes = "old.json" }},
		{"tutorial-dungeons", func(c *Config) { c.TutorialDungeons = "old.json" }},
		{"random-options", func(c *Config) { c.RandomOptionCatalog = "old.json" }},
		{"oath-grades", func(c *Config) { c.OathGradesTable = "old.json" }},
		{"vault", func(c *Config) { c.VaultRules = "old.json" }},
		{"boxes", func(c *Config) { c.Boxes = "old.json" }},
		{"item-shops", func(c *Config) { c.ItemShop = "old.json" }},
		{"bleeding-mine", func(c *Config) { c.BleedingMineRewards = "old.json" }},
		{"apocalypse", func(c *Config) { c.ApocalypseCatalog = "old.json" }},
		{"attunement", func(c *Config) { c.AttunementRewards = "old.json" }},
		{"characters", func(c *Config) {}},
	} {
		t.Run(row.domain, func(t *testing.T) {
			cfg := bootstrapTestConfig(t)
			cfg.CharacterStorage = "missing-storage.json"
			row.set(&cfg)
			runtime, cleanup, err := prepareRuntime(cfg)
			if runtime != nil || cleanup != nil || err == nil || !strings.Contains(err.Error(), row.domain+" requires a prepared native PVF") {
				t.Fatalf("content reached storage or bypassed PVF: %v", err)
			}
		})
	}
}

func TestShieldAndEnvironmentPathsCannotSelectJSON(t *testing.T) {
	cfg := Config{QuestEquipmentCatalog: "old.json", EquipmentWearRules: "slots.json", KnightShieldCatalog: "old-shields.json"}
	if err := requireRuntimeContent(cfg, &gamedata.Catalogs{}); err == nil || !strings.Contains(err.Error(), "shields") {
		t.Fatal(err)
	}
	for _, key := range []string{"DFO_ODYSSEY_GROWTH", "DFO_ODYSSEY_CHAPTERS", "DFO_ODYSSEY_WEAPON_BOX", "DFO_ODYSSEY_CHAPTER_DROP", "DFO_ODYSSEY_COIN_RULES", "DFO_CLEAR_CUBE_SOURCE", "DFO_TRAINING_ROOM_CATALOG"} {
		t.Run(key, func(t *testing.T) {
			t.Setenv(key, "old.json")
			if err := requireRuntimeContent(Config{}, &gamedata.Catalogs{}); err == nil || !strings.Contains(err.Error(), "native PVF") {
				t.Fatal(err)
			}
		})
	}
}
