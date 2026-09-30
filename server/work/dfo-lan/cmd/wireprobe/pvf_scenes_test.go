package main

import (
	"dfolan/internal/catalog"
	"os"
	"reflect"
	"testing"
)

func TestPVFScenesLocalArchive(t *testing.T) {
	path := os.Getenv("DFO_PVF_CORE_TEST_ARCHIVE")
	if path == "" {
		t.Skip("set DFO_PVF_CORE_TEST_ARCHIVE for complete scene parity")
	}
	c, err := preparePVFCoreCatalogs("town,dungeons,training-dungeons,tutorial-dungeons,dungeon-towers,dungeon-hell,dungeon-maze", path, os.Getenv("DFO_PVF_CORE_TEST_SHA256"), "../../configs/characters.skycastle-release.json", "", "", "", pvfItemInputs{indexPath: "../../configs/items.index.json", scenePolicyPath: "../../configs/pvf-scene-policy.json"})
	if err != nil {
		t.Fatal(err)
	}
	if !c.town.Allows(1, 561, 234) || len(c.trainingDungeons.Dungeons) != 4 {
		t.Fatal("entry/training compatibility changed")
	}
	if _, err := c.loadTown("missing.json"); err != nil {
		t.Fatal(err)
	}
	if _, err := c.loadDungeons("missing.json"); err != nil {
		t.Fatal(err)
	}
	base, err := c.loadDungeons("missing.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := c.attachTowerGrief(&base, "missing.json"); err != nil {
		t.Fatal(err)
	}
	if err := c.attachTowerDazzlement(&base, "missing.json"); err != nil {
		t.Fatal(err)
	}
	if err := c.attachHellMaps(&base, "missing.json"); err != nil {
		t.Fatal(err)
	}
	if err := c.attachMazeRates(&base, "missing.json"); err != nil {
		t.Fatal(err)
	}
	for _, layer := range c.grief.Layers {
		if base.Dungeons[layer.Dungeon].TowerGriefFloor == 0 || c.dungeons.Dungeons[layer.Dungeon].TowerGriefFloor != 0 {
			t.Fatal("tower attachment mutated prepared base")
		}
	}
	t.Log(len(c.dungeons.Dungeons), len(c.dungeons.Maps), len(c.tutorialDungeons.Dungeons))
}

func TestPVFOldDungeonBasisDiagnosticsAreNarrow(t *testing.T) {
	c := catalog.DungeonCatalog{Skipped: []string{"dungeon 5000: invalid [basis level]", "dungeon 5000: map missing", "dungeon 99: invalid [basis level]"}}
	got := normalizeOldDungeonBasisDiagnostics(c, pvfScenePolicy{Training: []uint32{5000}})
	if !reflect.DeepEqual(got.Skipped, c.Skipped[1:]) || len(c.Skipped) != 3 {
		t.Fatal("unexpected diagnostics suppressed or audit mutated its input")
	}
}
