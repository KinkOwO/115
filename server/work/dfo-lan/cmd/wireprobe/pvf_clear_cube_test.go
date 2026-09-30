package main

import (
	"dfolan/internal/catalog"
	"os"
	"testing"
)

func TestPVFClearCubeLocalArchive(t *testing.T) {
	path := os.Getenv("DFO_PVF_CORE_TEST_ARCHIVE")
	if path == "" {
		t.Skip("set DFO_PVF_CORE_TEST_ARCHIVE for source overlay parity")
	}
	c, err := preparePVFCoreCatalogs("clear-cube", path, os.Getenv("DFO_PVF_CORE_TEST_SHA256"), "../../configs/characters.skycastle-release.json", "", "", "", pvfItemInputs{indexPath: "../../configs/items.index.json"})
	if err != nil {
		t.Fatal(err)
	}
	base := catalog.LootCatalog{Source: c.items.Source, Items: map[uint32]catalog.LootItem{2: {ID: 2}}}
	next, err := c.withClearCube(base, "missing-cube.json")
	if err != nil {
		t.Fatal(err)
	}
	if next.Items[3037].ID != 3037 || next.Items[3037].Weight != 0 || next.Items[3037].Grade != 0 || len(base.Items) != 1 {
		t.Fatal("storage projection or original catalog mutated")
	}
	base.Source.Checksum = "wrong"
	if _, err := c.withClearCube(base, "missing-cube.json"); err == nil {
		t.Fatal("mixed source overlay accepted")
	}
}
