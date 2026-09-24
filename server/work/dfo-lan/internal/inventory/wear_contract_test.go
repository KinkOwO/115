package inventory

import (
	"dfolan/internal/catalog"
	"testing"
)

func TestConquerorContractEquipLevelBonus(t *testing.T) {
	c, e := catalog.LoadCharacters("../../configs/characters.next25.json")
	if e != nil {
		t.Fatal(e)
	}
	eq, e := LoadEquipmentCatalog("../../configs/equipment.current35.json", c.Source.Checksum)
	if e != nil {
		t.Fatal(e)
	}

	d, err := eq.Definition(10018)
	if err != nil {
		t.Fatalf("failed to find equipment 10018: %v", err)
	}
	minLevel := d.Fields["[minimum level]"][0].Value
	if minLevel != 20 {
		t.Fatalf("expected minimum level 20, got %d", minLevel)
	}

	job := c.Professions[0].Job
	kind := d.Fields["[equipment type]"][0].Text

	// At character level 10 without Conqueror's Contract: level 10 < 20 fails
	if err := WearableBy(d.Fields, kind, job, 0, 10); err == nil {
		t.Fatal("expected level 10 to fail wearing level 20 equipment without conqueror contract")
	}

	// At character level 10 with Conqueror's Contract (+10 levels => effective level 20): level 20 >= 20 succeeds
	effectiveLevel := byte(10 + 10)
	if err := WearableBy(d.Fields, kind, job, 0, effectiveLevel); err != nil {
		t.Fatalf("expected level 10 + 10 to succeed wearing level 20 equipment with conqueror contract: %v", err)
	}

	// At character level 9 with Conqueror's Contract (+10 levels => effective level 19): level 19 < 20 fails
	effectiveLevel9 := byte(9 + 10)
	if err := WearableBy(d.Fields, kind, job, 0, effectiveLevel9); err == nil {
		t.Fatal("expected level 9 + 10 to fail wearing level 20 equipment")
	}
}
