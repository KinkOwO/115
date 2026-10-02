package gamedata

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
	c, err := prepareCatalogsForTest(t, "characters,loot,equipment-selection", archive, "", "../../configs/characters.skycastle-release.json", "", "", "", CatalogInputs{DerivedCacheDir: os.Getenv("DFO_PVF_CACHE_DIR"), IndexPath: "../../configs/items.index.json", DropPolicyPath: "../../configs/pvf-drop-policy.json", CharacterPolicyPath: "../../configs/pvf-character-policy.json"})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if c.Loot != nil {
			if err := c.Loot.CloseDetails(); err != nil {
				t.Error(err)
			}
		}
	})
	if c.Selection == nil || c.Loot == nil || c.Loot.ClearReward == nil || len(c.Selection.OrdinaryPool) == 0 {
		t.Fatal("native ordinary catalogs not prepared")
	}
	if c.Selection.Source.Checksum != c.Loot.Source.Checksum {
		t.Fatal("ordinary pool source mismatch")
	}
	if c.Loot.OrdinaryMonsterItemRate != 1000 || !c.Loot.HasMonsterItemDetails() || !c.Loot.MonsterItemExclusions[6013] {
		t.Fatal("native monster item policy/details not bound")
	}
	if c.Loot.WorldDrop == nil || len(c.Loot.WorldDrop.Levels) != 200 || c.Loot.OrdinaryWorldDropPercent != 100 {
		t.Fatal("native world drop source/policy not bound")
	}
	table, known, err := c.Loot.MonsterItemTable(70)
	if err != nil || !known || len(table.Items) != 3 {
		t.Fatalf("native monster source unusable after preparation: %+v %v", table, err)
	}
	t.Logf("native ordinary pool=%d legacy pool=%d source=%s", len(c.Selection.OrdinaryPool), len(c.Selection.DropPool()), c.SourceChecksum)
}
