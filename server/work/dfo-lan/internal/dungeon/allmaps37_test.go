package dungeon

import (
	"dfolan/internal/catalog"
	"testing"
)

// Every map in the runtime catalog must parse. Live capture 20260912T025417
// refused CMD45 with "unresolved monster spawn option [champion]": a champion
// in a room the player tried to enter stalled dungeon progression. This walks
// every map through the real parser so no spawn option is left that would
// block a room.
func TestEveryRuntimeMapParses(t *testing.T) {
	c, e := catalog.LoadDungeons("../../configs/dungeons.next28.json")
	if e != nil {
		t.Skip("runtime dungeon catalog not present:", e)
	}
	parsed, failed := 0, 0
	for mapID, script := range c.Maps {
		// Basis level varies per dungeon; the parser only needs a non-zero
		// basis to resolve relative monster levels, so use a mid value.
		if _, err := fixedMonsters(script, 40); err != nil {
			failed++
			if failed <= 20 {
				t.Errorf("map %d does not parse: %v", mapID, err)
			}
			continue
		}
		parsed++
	}
	t.Logf("parsed %d maps, %d failed", parsed, failed)
	if failed != 0 {
		t.Fatalf("%d maps still block room entry", failed)
	}
}
