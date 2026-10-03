package catalog_test

import (
	"dfolan/internal/catalog"
	"dfolan/internal/inventory"
	"os"
	"testing"
)

// Optional broad check explicitly fails on a missing shield side-car rather
// than reproducing startup's deliberate feature-off behavior.
func TestConfigCrossCatalogSweep(t *testing.T) {
	if os.Getenv("CONFIG_SWEEP") != "1" {
		t.Skip("set CONFIG_SWEEP=1 to check shield cross-catalog provenance")
	}
	jobs, e := catalog.LoadCharacters("../../configs/characters.generated.json")
	if e != nil {
		t.Fatal(e)
	}
	shields, e := inventory.LoadKnightShields("../../configs/equipment-knight-shield.full-candidate.json", jobs.Source.Checksum)
	if e != nil {
		t.Fatal(e)
	}
	rules, e := inventory.LoadWearRules("../../configs/equipment-wear.current35.json", jobs.Source.Checksum)
	if e != nil {
		t.Fatal(e)
	}
	if jobs.Professions[shields.Profession].Job != "[knight]" || rules.Slots["[support weapon]"] != shields.Slot() {
		t.Fatal("shield profession or slot mismatch")
	}
	full, e := inventory.OpenFullEquipmentCatalog("../inventory/testdata/equipment-flow", jobs.Source.Checksum)
	if e != nil {
		t.Fatal(e)
	}
	defer full.Close()
	levels, quests := 0, 0
	for _, r := range shields.Rows {
		d, e := full.Definition(r.Item)
		if e != nil {
			t.Fatal(e)
		}
		if d.SHA256 != r.EquSHA256 || d.Fields["[equipment type]"][0].Text != "[support weapon]" {
			t.Fatalf("shield %d provenance/kind differs", r.Item)
		}
		if r.Condition == "quest" {
			quests++
		} else {
			levels++
		}
	}
	if levels != 19 || quests != 6 {
		t.Fatalf("unexpected window counts level=%d quest=%d", levels, quests)
	}
}
