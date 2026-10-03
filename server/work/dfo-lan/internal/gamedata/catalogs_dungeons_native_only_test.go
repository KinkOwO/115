package gamedata

import (
	"crypto/sha256"
	"dfolan/internal/catalog"
	"dfolan/internal/catalog/pvf"
	"encoding/json"
	"fmt"
	"os"
	"testing"
)

func TestRetiredDungeonJSONCannotSupplyRuntimeContent(t *testing.T) {
	c := &Catalogs{}
	if _, err := c.LoadDungeons("old-dungeons.json"); err == nil {
		t.Fatal("dungeon JSON runtime fallback accepted")
	}
}

func TestNativeDungeonsCurrentArchiveFingerprint(t *testing.T) {
	path := os.Getenv("DFO_PVF_CORE_TEST_ARCHIVE")
	if path == "" {
		t.Skip("set DFO_PVF_CORE_TEST_ARCHIVE for complete native dungeon fingerprint")
	}
	c, err := PrepareCatalogs(CatalogInputs{Selection: "dungeons", ArchivePath: path, ArchiveChecksum: "8b2a9f83247e000a28acd5134b616da725f46def39980b5373030e8cbc5d0934", DerivedCacheDir: "-", VerifyBaselines: true, DungeonPath: "missing-dungeons.json", ScenePolicyPath: "../../configs/pvf-scene-policy.json"}, CatalogAdapters{})
	if err != nil {
		t.Fatal(err)
	}
	defer c.Dungeons.CloseMapSource()
	d, err := c.LoadDungeons("missing-dungeons.json")
	if err != nil {
		t.Fatal(err)
	}
	full, err := d.ExpandedMaps()
	if err != nil {
		t.Fatal(err)
	}
	if len(full.Dungeons) != 3200 || len(full.Maps) != 18387 || len(full.Skipped) != 1699 {
		t.Fatal("complete native dungeon scope changed")
	}
	p, err := readPVFScenePolicy("../../configs/pvf-scene-policy.json")
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range append(append([]uint32(nil), p.Training...), p.Disabled...) {
		if _, ok := full.Dungeons[id]; ok {
			t.Fatalf("excluded dungeon %d admitted", id)
		}
	}
	// Archive load timestamps/path/cache counters are not game content.
	// Retain its verified checksum while hashing every effective game field.
	full.Source = pvf.ArchiveSnapshot{Checksum: full.Source.Checksum}
	raw, err := json.Marshal(full)
	if err != nil {
		t.Fatal(err)
	}
	if got := fmt.Sprintf("%x", sha256.Sum256(raw)); got != "cfef29a48e70f1b13bcc04d4b5bdac8bcf0728f9b9313678d816662149b90427" {
		t.Fatalf("complete native dungeon content changed: %s", got)
	}
}

// Historical diagnostic normalization is retained only for its audit regression.
// The old exporter rejected these fourteen missing-basis scripts. The current
// parser already accepts them, but training is separate and ten are disabled.
// Normalize only their obsolete diagnostic text; every other diagnostic and
// every gameplay field still participates in the complete comparison.
func normalizeOldDungeonBasisDiagnostics(c catalog.DungeonCatalog, p pvfScenePolicy) catalog.DungeonCatalog {
	obsolete := map[string]bool{}
	for _, id := range append(append([]uint32(nil), p.Training...), p.Disabled...) {
		obsolete[fmt.Sprintf("dungeon %d: invalid [basis level]", id)] = true
	}
	kept := make([]string, 0, len(c.Skipped))
	for _, diagnostic := range c.Skipped {
		if !obsolete[diagnostic] {
			kept = append(kept, diagnostic)
		}
	}
	c.Skipped = kept
	return c
}
