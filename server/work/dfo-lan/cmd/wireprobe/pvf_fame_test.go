package main

import (
	"dfolan/internal/character"
	"os"
	"testing"
)

func TestPVFFameLocalArchive(t *testing.T) {
	path := os.Getenv("DFO_PVF_CORE_TEST_ARCHIVE")
	if path == "" {
		t.Skip("set DFO_PVF_CORE_TEST_ARCHIVE for complete fame rule parity")
	}
	c, err := preparePVFCoreCatalogs("fame", path, os.Getenv("DFO_PVF_CORE_TEST_SHA256"), "../../configs/characters.skycastle-release.json", "", "", "", pvfItemInputs{indexPath: "../../configs/items.index.json"})
	if err != nil {
		t.Fatal(err)
	}
	restore, err := c.installFameRules()
	if err != nil {
		t.Fatal(err)
	}
	defer restore()
	current, err := character.CurrentFameRules()
	if err != nil {
		t.Fatal(err)
	}
	if err := verifyPVFCatalog(c.fameRules, current); err != nil {
		t.Fatal(err)
	}
	if len(current.Tables) != 9 || len(current.Items) != 8100 || len(current.Sets) != 13 || len(current.ItemPoints) != 1054 || len(current.Sources) != 8411 {
		t.Fatal("fame source scope changed")
	}
	t.Log("native fame tables", len(current.Tables), "items", len(current.Items), "sets", len(current.Sets), "source hashes", len(current.Sources))
}
