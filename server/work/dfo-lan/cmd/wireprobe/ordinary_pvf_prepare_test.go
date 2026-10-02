package main

import (
	"os"
	"testing"
)

// Use PVF characters as the live profile does. The older local-archive test
// anchors its identity to a historical character JSON and cannot validate a
// different current archive.
func TestOrdinaryPVFPreparation(t *testing.T) {
	archive := os.Getenv("DFO_ORDINARY_PVF_PREPARE")
	if archive == "" {
		t.Skip("set DFO_ORDINARY_PVF_PREPARE for native startup/cache validation")
	}
	t.Setenv("DFO_PVF_VERIFY_BASELINES", "0")
	t.Setenv("DFO_ORDINARY_MONSTER_ITEM_DROP_PERCENT", "10")
	t.Setenv("DFO_ORDINARY_WORLD_DROP_PERCENT", "100")
	verify := false
	c, err := preparePVFCoreCatalogs("characters,loot,equipment-selection", archive, "", "../../configs/characters.skycastle-release.json", "", "", "", pvfItemInputs{verifyBaselines: &verify, derivedCacheDir: os.Getenv("DFO_PVF_CACHE_DIR"), indexPath: "../../configs/items.index.json", dropPolicyPath: "../../configs/pvf-drop-policy.json", characterPolicyPath: "../../configs/pvf-character-policy.json"})
	if err != nil {
		t.Fatal(err)
	}
	if c.selection == nil || c.loot == nil || c.loot.ClearReward == nil || len(c.selection.OrdinaryPool) == 0 {
		t.Fatal("native ordinary catalogs not prepared")
	}
	if c.selection.Source.Checksum != c.loot.Source.Checksum {
		t.Fatal("ordinary pool source mismatch")
	}
	if c.loot.OrdinaryMonsterItemRate != 1000 || !c.loot.HasMonsterItemDetails() || !c.loot.MonsterItemExclusions[6013] {
		t.Fatal("native monster item policy/details not bound")
	}
	if c.loot.WorldDrop == nil || len(c.loot.WorldDrop.Levels) != 200 || c.loot.OrdinaryWorldDropPercent != 100 {
		t.Fatal("native world drop source/policy not bound")
	}
	table, known, err := c.loot.MonsterItemTable(70)
	if err != nil || !known || len(table.Items) != 3 {
		t.Fatalf("native monster source unusable after preparation: %+v %v", table, err)
	}
	t.Logf("native ordinary pool=%d legacy pool=%d source=%s", len(c.selection.OrdinaryPool), len(c.selection.DropPool()), c.sourceChecksum)
}
