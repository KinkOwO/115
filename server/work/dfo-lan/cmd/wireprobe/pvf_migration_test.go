package main

import (
	"os"
	"testing"
)

func TestPVFMigrationSourceOnlyLocalArchive(t *testing.T) {
	path := os.Getenv("DFO_PVF_CORE_TEST_ARCHIVE")
	if path == "" {
		t.Skip("set DFO_PVF_CORE_TEST_ARCHIVE for combined direct startup without export JSON")
	}
	verify := false
	c, err := preparePVFCoreCatalogs(pvfNextDomains+",enhancements,random-options,shields,oath-grades,vault,loot,equipment-selection", path, os.Getenv("DFO_PVF_CORE_TEST_SHA256"), "../../configs/characters.skycastle-release.json", "missing-quests.json", "missing-progression.json", "missing-world.json", pvfItemInputs{
		verifyBaselines: &verify, indexPath: "missing/items.index.json", fullPrefix: "missing/equipment-full", journalPath: "missing-journal.json", createCostPath: "missing-create-cost.json", learningPath: "missing-skills.json", pricesPath: "missing-prices.json", materialsPath: "missing-materials.json", boosterPath: "missing-boosters.json", tutorialPath: "missing-tutorial.json", enhancementPolicyPath: "../../configs/pvf-enhancement-policy.json",
		randomOptionPath: "missing-options.json", shieldPath: "missing-shields.json", oathPath: "missing-oath.json", vaultPath: "missing-vault.json", wearRulesPath: "../../configs/equipment-wear.current35.json", vaultPolicyPath: "../../configs/pvf-vault-policy.json", lootPath: "missing-loot.json", equipmentPath: "missing-equipment.json", questEquipmentPath: "missing-quest-equipment.json", dropPolicyPath: "../../configs/pvf-drop-policy.json",
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
	t.Log("21 selectors / 26 source families prepared with all selected export JSON paths absent")
}
