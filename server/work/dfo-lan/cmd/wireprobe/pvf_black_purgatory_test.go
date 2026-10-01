package main

import (
	"dfolan/internal/loot"
	"os"
	"testing"
)

func TestPVFBlackPurgatoryLocalArchive(t *testing.T) {
	path := os.Getenv("DFO_PVF_CORE_TEST_ARCHIVE")
	if path == "" {
		t.Skip("set DFO_PVF_CORE_TEST_ARCHIVE for source reward scope parity")
	}
	c, err := preparePVFCoreCatalogs("black-purgatory", path, os.Getenv("DFO_PVF_CORE_TEST_SHA256"), "../../configs/characters.skycastle-release.json", "", "", "", pvfItemInputs{indexPath: "../../configs/items.index.json", contentPolicyPath: "../../configs/pvf-reward-policy.json"})
	if err != nil {
		t.Fatal(err)
	}
	boxes := boosterBoxSource{catalog: &BoosterCatalog{Definitions: c.boosters, Items: c.items.Items}}
	r, err := c.loadBlackPurgatory("missing-rewards.json", boxes, blackPurgatoryLookup(c.items))
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Cards) != 5 || len(r.VIPSourceOnly) != 1 || len(r.Boss.Groups["epic"].Candidates) != 208 || len(r.Boss.Groups["mythic"].Candidates) != 35 || len(r.Boss.Groups["corrupt_product"].Candidates) != 135 || r.Boss.Rates["epic"] != 100000 || r.Boss.Rates["mythic"] != 1000 || r.Boss.Rates["corrupt_product"] != 10000 {
		t.Fatal("Black Purgatory scope/probability changed")
	}
	legacy := *r
	legacy.ClientSource = "unknown"
	if err := auditBlackPurgatory(legacy, *r); err == nil {
		t.Fatal("unknown provenance accepted")
	}
	legacy = *r
	legacy.Source = "wrong"
	if err := auditBlackPurgatory(legacy, *r); err == nil {
		t.Fatal("mixed character source accepted")
	}
	base := loot.BlackPurgatoryRewards{}
	if err := auditBlackPurgatory(base, *r); err == nil {
		t.Fatal("missing script identity accepted")
	}
}
