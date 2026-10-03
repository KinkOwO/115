package gamedata

import (
	"dfolan/internal/catalog"
	"dfolan/internal/catalog/pvf"
	"dfolan/internal/inventory"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRetiredLootAndEquipmentJSONCannotSupplyRuntimeCatalogs(t *testing.T) {
	lootChecksum := strings.Repeat("a", 64)
	legacyLoot := catalog.LootCatalog{
		Source:       pvf.ArchiveSnapshot{Checksum: lootChecksum},
		MaximumGrade: 1,
		Rules: map[string]catalog.ScriptRecord{
			"a": {}, "b": {}, "c": {}, "d": {},
		},
		Items: map[uint32]catalog.LootItem{},
	}
	lootData, err := json.Marshal(legacyLoot)
	if err != nil {
		t.Fatal(err)
	}
	lootPath := filepath.Join(t.TempDir(), "old-loot.json")
	if err := os.WriteFile(lootPath, lootData, 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := catalog.LoadLoot(lootPath); err != nil {
		t.Fatalf("test must provide a valid legacy loot catalog: %v", err)
	}
	if _, err := (&Catalogs{}).LoadLoot(lootPath); err == nil {
		t.Fatal("valid legacy loot JSON supplied runtime content without prepared PVF")
	}

	equipmentChecksum := strings.Repeat("b", 64)
	legacyEquipment := inventory.EquipmentCatalog{
		Source: pvf.ArchiveSnapshot{Checksum: equipmentChecksum},
		Rows:   []inventory.EquipmentDefinition{{ID: 1, Path: "equipment/test.equ", SHA256: strings.Repeat("c", 64)}},
	}
	equipmentData, err := json.Marshal(legacyEquipment)
	if err != nil {
		t.Fatal(err)
	}
	equipmentPath := filepath.Join(t.TempDir(), "old-equipment.json")
	if err := os.WriteFile(equipmentPath, equipmentData, 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := inventory.LoadEquipmentCatalog(equipmentPath, equipmentChecksum); err != nil {
		t.Fatalf("test must provide a valid legacy equipment catalog: %v", err)
	}
	if _, err := (&Catalogs{}).LoadEquipmentSelection(equipmentPath, equipmentChecksum); err == nil {
		t.Fatal("valid legacy equipment JSON supplied runtime content without prepared PVF")
	}
}

func TestPreparedLootAndEquipmentSelectionIgnoreLegacyPaths(t *testing.T) {
	checksum := strings.Repeat("d", 64)
	loot := &catalog.LootCatalog{
		Source:       pvf.ArchiveSnapshot{Checksum: checksum},
		MaximumGrade: 150,
		Rules:        map[string]catalog.ScriptRecord{},
		Items:        map[uint32]catalog.LootItem{},
	}
	selection := &inventory.EquipmentCatalog{Source: pvf.ArchiveSnapshot{Checksum: checksum}, Rows: []inventory.EquipmentDefinition{{ID: 1}}}
	c := &Catalogs{
		selected:  map[string]bool{"loot": true, "equipment-selection": true},
		prepared:  map[string]bool{"loot": true, "equipment-selection": true},
		Loot:      loot,
		Selection: selection,
	}
	gotLoot, err := c.LoadLoot(filepath.Join(t.TempDir(), "missing-loot.json"))
	if err != nil || gotLoot.Source.Checksum != checksum || gotLoot.Source.SaveIdentity() != loot.Source.SaveIdentity() {
		t.Fatalf("prepared native loot was not returned with save identity intact: err=%v source=%+v", err, gotLoot.Source)
	}
	gotSelection, err := c.LoadEquipmentSelection(filepath.Join(t.TempDir(), "missing-equipment.json"), checksum)
	if err != nil || gotSelection == selection || gotSelection.Source.Checksum != checksum || gotSelection.Source.SaveIdentity() != selection.Source.SaveIdentity() {
		t.Fatalf("prepared native equipment selection/source identity changed: err=%v", err)
	}
	if _, err := c.LoadEquipmentSelection("missing.json", strings.Repeat("e", 64)); err == nil {
		t.Fatal("equipment selection from a different source was accepted")
	}
}

func TestPVFLootAndEquipmentSelectionPrepareWithoutLegacyBaselines(t *testing.T) {
	path := os.Getenv("DFO_PVF_CORE_TEST_ARCHIVE")
	if path == "" {
		t.Skip("set DFO_PVF_CORE_TEST_ARCHIVE for native loot and equipment preparation")
	}
	const currentChecksum = "8b2a9f83247e000a28acd5134b616da725f46def39980b5373030e8cbc5d0934"
	if got := os.Getenv("DFO_PVF_CORE_TEST_SHA256"); got != currentChecksum {
		t.Skip("set DFO_PVF_CORE_TEST_SHA256 to the current 8b2a archive for this parity gate")
	}
	c, err := PrepareCatalogs(CatalogInputs{
		Selection: "loot,equipment-selection", ArchivePath: path, ArchiveChecksum: currentChecksum,
		DerivedCacheDir: "-", VerifyBaselines: true,
		IndexPath: "missing-items.json", DropPolicyPath: "../../configs/pvf-drop-policy.json",
		LootPath: "missing-loot-baseline.json", EquipmentPath: "missing-equipment-baseline.json",
		QuestEquipmentPath: "missing-quest-equipment-baseline.json",
	}, CatalogAdapters{})
	if err != nil {
		t.Fatal(err)
	}
	if c.Loot == nil || c.Loot.MaximumGrade != 150 || c.Loot.MonsterItemExclusions[6013] != true || c.Selection == nil {
		t.Fatalf("native source or independent drop policy changed: loot=%v selection=%v", c.Loot != nil, c.Selection != nil)
	}
	if _, err := c.LoadLoot("missing-loot-baseline.json"); err != nil {
		t.Fatal(err)
	}
	if _, err := c.LoadEquipmentSelection("missing-equipment-baseline.json", c.Selection.Source.Checksum); err != nil {
		t.Fatal(err)
	}
	t.Logf("native source=%s loot=%d equipment-selection=%d", c.SourceChecksum, len(c.Loot.Items), len(c.Selection.Rows))
}
