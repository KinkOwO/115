package main

import (
	"dfolan/internal/adventure"
	"os"
	"testing"
)

func TestPVFMigrationSourceOnlyLocalArchive(t *testing.T) {
	path := os.Getenv("DFO_PVF_CORE_TEST_ARCHIVE")
	if path == "" {
		t.Skip("set DFO_PVF_CORE_TEST_ARCHIVE for combined direct startup without export JSON")
	}
	verify := false
	c, err := preparePVFCoreCatalogs(pvfSupportedDomains, path, os.Getenv("DFO_PVF_CORE_TEST_SHA256"), "../../configs/characters.skycastle-release.json", "missing-quests.json", "missing-progression.json", "missing-world.json", pvfItemInputs{
		lotteryPolicyPath: "../../configs/pvf-lottery-policy.json", selectionBoxesPath: "missing-selection-boxes.json", selectionPolicyPath: "../../configs/pvf-selection-policy.json", verifyBaselines: &verify, minePath: "missing-mine.json", blackPurgatoryPath: "missing-black-purgatory.json", clearCubePath: "missing-cube.json", odysseyGrowthPath: "missing-growth.json", odysseyChapterPath: "missing-chapters.json", odysseyDropPath: "missing-odyssey-drop.json", odysseyCurrencyPath: "missing-coins.json", odysseyWeaponPath: "missing-weapons.json", apocalypsePath: "missing-apocalypse.json", attunementPath: "missing-attunement.json", contentPolicyPath: "../../configs/pvf-mine-policy.json", indexPath: "missing/items.index.json", fullPrefix: "missing/equipment-full", journalPath: "missing-journal.json", createCostPath: "missing-create-cost.json", learningPath: "missing-skills.json", pricesPath: "missing-prices.json", materialsPath: "missing-materials.json", boosterPath: "missing-boosters.json", tutorialPath: "missing-tutorial.json", enhancementPolicyPath: "../../configs/pvf-enhancement-policy.json",
		randomOptionPath: "missing-options.json", shieldPath: "missing-shields.json", oathPath: "missing-oath.json", vaultPath: "missing-vault.json", wearRulesPath: "../../configs/equipment-wear.current35.json", vaultPolicyPath: "../../configs/pvf-vault-policy.json", lootPath: "missing-loot.json", equipmentPath: "missing-equipment.json", questEquipmentPath: "missing-quest-equipment.json", dropPolicyPath: "../../configs/pvf-drop-policy.json",
		townPath: "missing-town.json", dungeonPath: "missing-dungeons.json", trainingDungeonPath: "missing-training.json", tutorialDungeonPath: "missing-tutorial-dungeons.json", scenePolicyPath: "../../configs/pvf-scene-policy.json",
	})
	if err != nil {
		t.Fatal(err)
	}
	defer c.equipment.Close()
	collectPVFImportMemory(c)
	verifyEquipmentRuleReuse(t, c)
	if len(c.loot.Items) != 1022 || len(c.selection.Rows) != 3174 || len(c.selection.DropPool()) != 2794 {
		t.Fatal("existing loot scope changed")
	}
	if err := c.loadEnhancements("missing-enhancements"); err != nil {
		t.Fatal(err)
	}
	if _, err := c.equipment.Definition(101001153); err != nil {
		t.Fatal(err)
	}
	if !c.town.Allows(1, 561, 234) || len(c.dungeons.Dungeons) != 3200 || len(c.trainingDungeons.Dungeons) != 4 || len(c.tutorialDungeons.Dungeons) != 15 {
		t.Fatal("scene selection changed")
	}
	if c.grief == nil || c.dazzlement == nil || c.hellMaps == nil || c.mazeRates == nil {
		t.Fatal("source overlays missing")
	}
	base, err := c.loadDungeons("missing-dungeons.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := c.attachTerminalScenes(&base, "missing-terminal.json"); err != nil {
		t.Fatal(err)
	}
	if err := c.attachTournamentMaps(&base, "missing-tournament.json"); err != nil {
		t.Fatal(err)
	}
	if len(base.TerminalScenes) != 7 || len(c.tournamentMaps.Maps) != 2 {
		t.Fatal("closing scene source scope changed")
	}

	boxes, err := c.loadSelectionBoxes("missing-selection-boxes.json")
	if err != nil || len(boxes.Boxes) != 2975 || len(boxes.Fixed) != 2 || len(boxes.Unparsed) != 1 {
		t.Fatal("selection source scope changed", err)
	}

	lottery, err := c.loadLotteryItems("missing-lottery.json", c.items.Items)
	if err != nil {
		t.Fatal(err)
	}
	if count, err := c.loadLotteryEquipment("missing-equipment-lottery.json", c.items.Items, lottery); err != nil || count != 2477 || len(lottery.byTemplate) != 2753 {
		t.Fatal("lottery scope changed", err)
	}
	restore, err := c.installAdventureRules()
	if err != nil {
		t.Fatal(err)
	}
	defer restore()
	rules, err := adventure.Current()
	if err != nil || len(rules.Experience) != 60 || len(rules.Shops) != 3 {
		t.Fatal("native embedded adventure rules missing", err)
	}
	restoreRecommended, err := c.installRecommendedRules()
	if err != nil {
		t.Fatal(err)
	}
	defer restoreRecommended()
	recommended, err := adventure.CurrentRecommendedRules()
	if err != nil || len(recommended.Ranges) == 0 {
		t.Fatal("native embedded recommended rules missing", err)
	}
	restoreSeason, err := c.installSeasonRules()
	if err != nil {
		t.Fatal(err)
	}
	defer restoreSeason()
	season, err := adventure.CurrentSeason()
	if err != nil || len(season.Levels) != 120 || len(season.Capsules) != 40 {
		t.Fatal("native embedded season rules missing", err)
	}
	t.Log("45 selectors / 52 source families prepared with all selected export JSON paths absent")
}
