package catalog

import "testing"

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
