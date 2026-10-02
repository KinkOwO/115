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
	t.Logf("native ordinary pool=%d legacy pool=%d source=%s", len(c.selection.OrdinaryPool), len(c.selection.DropPool()), c.sourceChecksum)
}
