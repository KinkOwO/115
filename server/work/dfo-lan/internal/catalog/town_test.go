package catalog

import (
	"dfolan/internal/catalog/pvf"
	"testing"
)

func TestTownSourceEntryBounds(t *testing.T) {
	c := TownArea{MinimumLevel: 1, Walkable: [][4]int32{{10, 196, 2670, 142}}}
	for _, tc := range []struct {
		level   byte
		x, y    uint16
		allowed bool
	}{{1, 561, 234, true}, {0, 561, 234, false}, {1, 9, 234, false}, {1, 2680, 234, false}, {1, 561, 338, false}, {1, 2679, 337, true}} {
		if got := c.Allows(tc.level, tc.x, tc.y); got != tc.allowed {
			t.Fatalf("level=%d x=%d y=%d: %v", tc.level, tc.x, tc.y, got)
		}
	}
}

func TestWorldAreaDungeonGatePriority(t *testing.T) {
	// Verify that [dungeon gate] is not overwritten by subsequent [normal] in area definition
	tokens := []pvf.Token{
		{Type: 3, Text: "[area]"},
		{Type: 0, Value: 0},
		{Type: 6, Text: "map/test.map"},
		{Type: 6, Text: "[dungeon gate]"},
		{Type: 6, Text: "[normal]"},
		{Type: 3, Text: "[/area]"},
	}
	areas, err := parseWorldAreas(235, tokens)
	if err != nil {
		t.Fatalf("parseWorldAreas failed: %v", err)
	}
	if len(areas) != 1 {
		t.Fatalf("expected 1 area, got %d", len(areas))
	}
	if areas[0].Kind != "[dungeon gate]" {
		t.Fatalf("expected Kind to be [dungeon gate], got %q", areas[0].Kind)
	}
}
