package gamedata

import (
	"dfolan/internal/catalog"
	"dfolan/internal/testfixture"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const currentItemContentPVF = "8b2a9f83247e000a28acd5134b616da725f46def39980b5373030e8cbc5d0934"

func TestItemContentLoadersFailClosedWithoutNativePreparation(t *testing.T) {
	c := &Catalogs{}
	if _, err := c.LoadItemMaterials("configs/item-materials.json"); err == nil || !strings.Contains(err.Error(), "native PVF materials") {
		t.Fatalf("materials loader fell back to JSON: %v", err)
	}
	if _, err := c.LoadItemPeriods("configs/item-period-tags.json", ""); err == nil || !strings.Contains(err.Error(), "native PVF periods") {
		t.Fatalf("period loader fell back to JSON: %v", err)
	}
	if _, err := c.LoadSkinStorage("configs/skin-storage-items.json", ""); err == nil || !strings.Contains(err.Error(), "native PVF skins") {
		t.Fatalf("skin loader fell back to JSON: %v", err)
	}
	c.selected = map[string]bool{"materials": true, "periods": true, "skins": true}
	if _, err := c.LoadItemMaterials("missing"); err == nil || !strings.Contains(err.Error(), "selected PVF materials projection is not prepared") {
		t.Fatalf("selected missing materials domain did not fail closed: %v", err)
	}
	if _, err := c.LoadItemPeriods("missing", ""); err == nil || !strings.Contains(err.Error(), "selected PVF periods projection is not prepared") {
		t.Fatalf("selected missing periods domain did not fail closed: %v", err)
	}
	if _, err := c.LoadSkinStorage("missing", ""); err == nil || !strings.Contains(err.Error(), "selected PVF skins projection is not prepared") {
		t.Fatalf("selected missing skins domain did not fail closed: %v", err)
	}
}

// This deliberately gated migration witness compares the complete old typed
// projections to the current PVF after changing only each temporary fixture's
// provenance checksum. The checked-in historical snapshots remain byte-exact.
func TestCurrentPVFItemContentHistoricalFieldParity(t *testing.T) {
	archive := os.Getenv("DFO_PVF_CORE_TEST_ARCHIVE")
	if archive == "" {
		t.Skip("set DFO_PVF_CORE_TEST_ARCHIVE for current item-content migration parity")
	}
	if got := strings.ToLower(os.Getenv("DFO_PVF_CORE_TEST_SHA256")); got != currentItemContentPVF {
		t.Fatalf("migration witness must run against current PVF %s, got %q", currentItemContentPVF, got)
	}
	source, err := Open(Options{Mode: PVF, ArchivePath: filepath.Clean(archive), ExpectedChecksum: currentItemContentPVF, DerivedCacheDir: testPVFCacheDir()})
	if err != nil {
		t.Fatal(err)
	}
	defer source.Close()
	checksum := source.Snapshot().Checksum
	if checksum != currentItemContentPVF {
		t.Fatalf("current PVF checksum changed: %s", checksum)
	}
	direct, err := source.ItemCatalogs(catalog.ItemBasicOptions{Periods: true, Materials: true, Skins: true}, false, "", false)
	if err != nil {
		t.Fatal(err)
	}
	if direct.Basics.Periods == nil || direct.Basics.Materials == nil || direct.Basics.Skins == nil {
		t.Fatal("joint native item projections are incomplete")
	}

	materialsPath := testfixture.ItemContentPath(t, "materials", checksum)
	legacyMaterials, err := catalog.LoadItemMaterials(materialsPath)
	if err != nil {
		t.Fatal(err)
	}
	if diff := Compare(legacyMaterials, direct.Basics.Materials, 1); diff.Count != 0 {
		t.Fatalf("materials typed fields differ from current PVF: %+v", diff)
	}
	if len(direct.Basics.Materials.Items) != 14211 {
		t.Fatalf("materials count changed: %d", len(direct.Basics.Materials.Items))
	}
	if got, ok := direct.Basics.Materials.Materials(10345008); !ok || len(got) != 2 || got[0] != (catalog.ItemMaterialCost{Template: 10400396, Count: 500}) || got[1] != (catalog.ItemMaterialCost{Template: 10403609, Count: 5}) {
		t.Fatalf("live shop material recipe changed: %+v found=%t", got, ok)
	}
	if got, ok := direct.Basics.Materials.Materials(3242); !ok || len(got) != 1 || got[0] != (catalog.ItemMaterialCost{Template: 3037, Count: 1000}) {
		t.Fatalf("material payment recipe for 3242 changed: %+v found=%t", got, ok)
	}

	periodsPath := testfixture.ItemContentPath(t, "periods", checksum)
	periodBytes, err := os.ReadFile(periodsPath)
	if err != nil {
		t.Fatal(err)
	}
	var legacyPeriods catalog.ItemPeriodCatalog
	if err := json.Unmarshal(periodBytes, &legacyPeriods); err != nil {
		t.Fatal(err)
	}
	if _, err := catalog.LoadItemPeriods(periodsPath, checksum); err != nil {
		t.Fatal(err)
	}
	if diff := Compare(legacyPeriods, *direct.Basics.Periods, 1); diff.Count != 0 {
		t.Fatalf("periods typed fields differ from current PVF: %+v", diff)
	}
	if len(direct.Basics.Periods.Templates) != 124610 {
		t.Fatalf("period template count changed: %d", len(direct.Basics.Periods.Templates))
	}

	skinsPath := testfixture.ItemContentPath(t, "skins", checksum)
	skinBytes, err := os.ReadFile(skinsPath)
	if err != nil {
		t.Fatal(err)
	}
	var legacySkins catalog.SkinStorageCatalog
	if err := json.Unmarshal(skinBytes, &legacySkins); err != nil {
		t.Fatal(err)
	}
	resolved, err := catalog.LoadSkinStorage(skinsPath, checksum)
	if err != nil {
		t.Fatal(err)
	}
	if diff := Compare(legacySkins, *direct.Basics.Skins, 1); diff.Count != 0 {
		t.Fatalf("skin typed fields differ from current PVF: %+v", diff)
	}
	if len(resolved) != 1733 || len(legacySkins.MissingSkins) != 127 || len(direct.Basics.Skins.MissingSkins) != 127 {
		t.Fatalf("skin resolution boundary changed: resolved=%d old-missing=%d native-missing=%d", len(resolved), len(legacySkins.MissingSkins), len(direct.Basics.Skins.MissingSkins))
	}
	t.Logf("current PVF %s: materials=%d periods=%d skins=%d unresolved-skins=%d; all typed fields match after provenance-only fixture rewrite", checksum, len(direct.Basics.Materials.Items), len(direct.Basics.Periods.Templates), len(resolved), len(direct.Basics.Skins.MissingSkins))
}
