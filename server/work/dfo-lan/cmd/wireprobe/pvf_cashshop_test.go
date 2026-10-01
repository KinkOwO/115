package main

import (
	"os"
	"testing"
)

func TestPVFCashShopLocalArchive(t *testing.T) {
	path := os.Getenv("DFO_PVF_CORE_TEST_ARCHIVE")
	if path == "" {
		t.Skip("set DFO_PVF_CORE_TEST_ARCHIVE for complete cashshop parity")
	}
	c, err := preparePVFCoreCatalogs("cashshop", path, os.Getenv("DFO_PVF_CORE_TEST_SHA256"), "../../configs/characters.skycastle-release.json", "", "", "", pvfItemInputs{cashshopPath: "../../configs/shop-vault-release.json", cashshopRelease: true})
	if err != nil {
		t.Fatal(err)
	}
	p, err := c.loadCashShop("missing-cashshop.json", c.cashshop.Config.Source.Checksum, true)
	if err != nil || p.EnabledCount() == 0 {
		t.Fatal("prepared shop not reused", err)
	}
	if _, err := c.loadCashShop("missing-cashshop.json", "foreign", true); err == nil {
		t.Fatal("foreign save source accepted")
	}
	if _, err := c.loadCashShop("missing-cashshop.json", p.Config.Source.Checksum, false); err == nil {
		t.Fatal("release behavior silently changed")
	}
	t.Logf("complete native rows=%d products=%d policies=%d priceSHA=%s indexSHA=%s equipmentSHA=%s", len(p.Config.Entries), p.EnabledCount(), len(p.Config.Policies), p.Config.PriceScriptHash, p.Config.IndexHash, p.Config.EquipmentIndexHash)
}
