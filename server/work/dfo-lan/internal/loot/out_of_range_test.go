package loot

import (
	"errors"
	"testing"

	"dfolan/internal/catalog"
)

func loadRuntimeLoot(t *testing.T) (catalog.LootCatalog, Rules, Tables) {
	t.Helper()
	c, e := catalog.LoadLoot("../../configs/loot.next25.json")
	if e != nil {
		t.Fatal(e)
	}
	rules, e := LoadRules("../../configs/drop.compat90.json")
	if e != nil {
		t.Fatal(e)
	}
	tables, e := Parse(c)
	if e != nil {
		t.Fatal(e)
	}
	return c, rules, tables
}

// Monsters above the catalog's import ceiling are what broke the 2026-09-26
// border-of-attunement session: every death report was refused, so the kills
// never registered and the clear could not happen. The classification is what
// makes the fix possible, so it is pinned here.
func TestLevelAboveCatalogCeilingIsACoverageGap(t *testing.T) {
	c, rules, tables := loadRuntimeLoot(t)

	// The production case, kept explicit so the incident stays readable: the
	// dungeon declares [basis level] 145 while this catalog is imported to 130.
	const incidentLevel = 145
	if uint32(incidentLevel)+3 > c.MaximumGrade {
		if _, e := RollWithBonus(c, tables, rules, nil, 1, incidentLevel, 0, 0, 0); !errors.Is(e, ErrOutOfDropRange) {
			t.Fatalf("level %d against a grade-%d catalog: got %v, want a coverage gap",
				incidentLevel, c.MaximumGrade, e)
		}
	} else {
		t.Logf("catalog ceiling is %d: level %d is now covered, the incident case no longer applies",
			c.MaximumGrade, incidentLevel)
	}

	// Whatever the ceiling is, one level past it must still classify as a gap -
	// this half cannot go stale when the catalog is re-imported.
	outOfRange := byte(c.MaximumGrade + 1)
	if outOfRange == 0 {
		t.Fatal("catalog ceiling leaves no room for an out-of-range level")
	}
	if _, e := RollWithBonus(c, tables, rules, nil, 1, outOfRange, 0, 0, 0); !errors.Is(e, ErrOutOfDropRange) {
		t.Fatalf("level %d against a grade-%d catalog: got %v, want a coverage gap",
			outOfRange, c.MaximumGrade, e)
	}
}

// Option B only downgrades coverage gaps. A defect of the model itself must keep
// failing the caller's request, otherwise a real bug would be hidden behind
// "this monster simply paid nothing".
func TestModelDefectsDoNotLookLikeCoverageGaps(t *testing.T) {
	c, rules, _ := loadRuntimeLoot(t)
	_, e := RollWithBonus(c, Tables{}, rules, nil, 1, 1, 0, 0, 0)
	if e == nil {
		t.Fatal("empty drop model accepted")
	}
	if errors.Is(e, ErrOutOfDropRange) {
		t.Fatalf("model defect classified as a coverage gap: %v", e)
	}
}
