package main

import (
	"dfolan/internal/inventory"
	"os"
	"testing"
)

func TestPVFEquipmentRulesLocalArchive(t *testing.T) {
	path := os.Getenv("DFO_PVF_CORE_TEST_ARCHIVE")
	if path == "" {
		t.Skip("set DFO_PVF_CORE_TEST_ARCHIVE for four equipment/vault rule families")
	}
	c, err := preparePVFCoreCatalogs("random-options,shields,oath-grades,vault", path, os.Getenv("DFO_PVF_CORE_TEST_SHA256"), "../../configs/characters.skycastle-release.json", "", "", "", pvfItemInputs{indexPath: "../../configs/items.index.json", wearRulesPath: "../../configs/equipment-wear.current35.json", vaultPolicyPath: "../../configs/pvf-vault-policy.json"})
	if err != nil {
		t.Fatal(err)
	}
	verifyEquipmentRuleReuse(t, c)
}

func TestPVFEquipmentRulesSourceOnlyLocalArchive(t *testing.T) {
	path := os.Getenv("DFO_PVF_CORE_TEST_ARCHIVE")
	if path == "" {
		t.Skip("set DFO_PVF_CORE_TEST_ARCHIVE for direct preparation without JSON baselines")
	}
	verify := false
	c, err := preparePVFCoreCatalogs("random-options,shields,oath-grades,vault", path, os.Getenv("DFO_PVF_CORE_TEST_SHA256"), "../../configs/characters.skycastle-release.json", "", "", "", pvfItemInputs{verifyBaselines: &verify, indexPath: "missing/items.json", randomOptionPath: "missing-options.json", shieldPath: "missing-shields.json", oathPath: "missing-oath.json", vaultPath: "missing-vault.json", wearRulesPath: "../../configs/equipment-wear.current35.json", vaultPolicyPath: "../../configs/pvf-vault-policy.json"})
	if err != nil {
		t.Fatal(err)
	}
	verifyEquipmentRuleReuse(t, c)
}

func verifyEquipmentRuleReuse(t *testing.T, c pvfCoreCatalogs) {
	t.Helper()
	source := c.items.Source.Checksum
	if x, err := c.loadRandomOptions("missing.json", source); err != nil || x.GroupCount() == 0 {
		t.Fatal("random options not reused", err)
	}
	if _, err := c.loadRandomOptions("missing.json", "wrong"); err == nil {
		t.Fatal("wrong random option source accepted")
	}
	shields, err := c.loadShields("missing.json", source)
	if err != nil || len(shields.Rows) != 25 {
		t.Fatal("shields not reused", err)
	}
	for _, row := range shields.Rows {
		if row.Condition == "quest" {
			if ok, _ := shields.Allowed(row.Item, 115); ok {
				t.Fatal("unverified quest shield enabled")
			}
		}
	}
	if oath, err := c.loadOathGrades("missing.json"); err != nil || oath.Len() == 0 {
		t.Fatal("oath table not reused", err)
	}
	vault, err := c.loadVaultRules("missing.json")
	if err != nil {
		t.Fatal(err)
	}
	if vault.SourceSHA256 != "fda6c33f2a155124a4262f3e7cd6f7a18d4e2191e5f147aec5754716afe5aa7c" || vault.InitialSlots != 8 || vault.InitialSecondarySlots != 24 || vault.Account.RequiredLevel != 60 || len(vault.Account.Upgrades) != 40 {
		t.Fatal("vault capacity/save or source table changed")
	}
	if limit, err := vault.Account.GoldLimit(64); err != nil || limit != 500000000 {
		t.Fatal(limit, err)
	}
	primer, oath := c.oath.Grades([]inventory.BagEquipment{})
	if primer != 40 || oath != 40 {
		t.Fatal("diagnostic grades default changed")
	}
	t.Logf("four source projections and runtime indexes ready; groups=%d shields=%d oath=%d vault=%d", c.randomOptions.GroupCount(), len(c.shields.Rows), c.oath.Len(), len(vault.Account.Upgrades))
}
