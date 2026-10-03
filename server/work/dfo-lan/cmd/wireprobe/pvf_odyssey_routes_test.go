package main

import (
	"dfolan/internal/character"
	"os"
	"testing"
)

func TestPVFOdysseyRoutesLocalArchive(t *testing.T) {
	path := os.Getenv("DFO_PVF_CORE_TEST_ARCHIVE")
	if path == "" {
		t.Skip("set DFO_PVF_CORE_TEST_ARCHIVE for complete journal route parity")
	}
	c, err := preparePVFCoreCatalogs("odyssey-routes", path, os.Getenv("DFO_PVF_CORE_TEST_SHA256"), "../../configs/characters.skycastle-release.json", "", "", "", pvfItemInputs{})
	if err != nil {
		t.Fatal(err)
	}
	var service character.ProgressionService
	if err := c.bindOdysseyRoutes(&service); err != nil {
		t.Fatal(err)
	}
	if err := verifyPVFCatalog(c.odysseyRoutes, service.JournalRoutes); err != nil {
		t.Fatal(err)
	}
	if len(service.JournalRoutes.Nodes) != 29 {
		t.Fatal("journal scope changed")
	}
	before := c.odysseyRoutes.Nodes[0].Dungeons[0]
	service.JournalRoutes.Nodes[0].Dungeons[0]++
	if c.odysseyRoutes.Nodes[0].Dungeons[0] != before {
		t.Fatal("runtime mutation reached prepared source")
	}
	t.Log("native nodes", len(c.odysseyRoutes.Nodes), "source SHA256", c.odysseyRoutes.SHA256)
}
