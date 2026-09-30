package main

import (
	"dfolan/internal/gamedata"
	"dfolan/internal/legion"
	"dfolan/internal/loot"
	"os"
	"testing"
)

func TestPVFAttunementLocalArchive(t *testing.T) {
	path := os.Getenv("DFO_PVF_CORE_TEST_ARCHIVE")
	if path == "" {
		t.Skip("set DFO_PVF_CORE_TEST_ARCHIVE for native reward CTP parity")
	}
	c, err := preparePVFCoreCatalogs("attunement", path, os.Getenv("DFO_PVF_CORE_TEST_SHA256"), "../../configs/characters.skycastle-release.json", "", "", "", pvfItemInputs{indexPath: "../../configs/items.index.json", contentPolicyPath: "../../configs/pvf-content-policy.json"})
	if err != nil {
		t.Fatal(err)
	}
	a, err := c.loadAttunement("missing.json")
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := a.ApplyRebalance(loot.Rebalance{FixedTiltPercent: 25, OmenHalveIdle: true}); err != nil {
		t.Fatal(err)
	}
	b, err := c.loadAttunement("missing.json")
	if err != nil {
		t.Fatal(err)
	}
	if diff := gamedata.Compare(c.attunement, b, 1); diff.Count != 0 {
		t.Fatal("rebalance mutated native source", diff)
	}
	if len(c.attunement.Dungeons()) != 4 || c.attunement.Coupons() != 5 {
		t.Fatal("reward scope changed")
	}
	t.Log(c.attunement.Dungeons(), len(c.attunement.Templates()), c.attunement.Coupons())
}

func TestPVFApocalypseLocalArchive(t *testing.T) {
	path := os.Getenv("DFO_PVF_CORE_TEST_ARCHIVE")
	if path == "" {
		t.Skip("set DFO_PVF_CORE_TEST_ARCHIVE for native CTP parity")
	}
	c, err := preparePVFCoreCatalogs("apocalypse", path, os.Getenv("DFO_PVF_CORE_TEST_SHA256"), "../../configs/characters.skycastle-release.json", "", "", "", pvfItemInputs{indexPath: "../../configs/items.index.json"})
	if err != nil {
		t.Fatal(err)
	}
	x, err := c.loadApocalypse("missing.json")
	if err != nil {
		t.Fatal(err)
	}
	clock, err := legion.NewApocalypseClock(x)
	if err != nil || clock.Len() != 6 || len(x.Operations) != 4 {
		t.Fatal("apocalypse clock/operations changed", err)
	}
	t.Log(x.RecordCount, len(x.Duties.Records), clock.TotalSeconds())
}
