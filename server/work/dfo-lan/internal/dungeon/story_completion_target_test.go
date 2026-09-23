package dungeon

import (
	"dfolan/internal/catalog"
	"dfolan/internal/game/protocol"
	"os"
	"testing"
)

// catalogPath is the shipped dungeon catalog, read relative to this package so
// the suite stays runnable on a bare checkout.
const catalogPath = "../../configs/dungeons.full.json"

// TestStoryLayerCompletionOnRealCastellanChamber pins the shipped shape of the
// run this fallback exists for: dungeon 15 maze 6 (quest 3191, palaceofload).
// Its layer sequence 100008695 -> 100008694 -> 100008684 -> 100008683 is walked
// without a fight, and the last map's only rank-3 actor is a [displayhuntdummy]
// boss - Rank 3 but spawned NonCombat - so the client never raises a BOSS_CHECK
// and no completion target is ever requested.
//
// The run must therefore complete on arrival at the last map and must not
// complete on any earlier map of the sequence, or the client is left with a
// dark clear button and no settlement. The reported target has to be encodable
// too: the wire encoder rejects 0, and a rejected payload drops the whole
// completion batch with the clear-enable in it.
func TestStoryLayerCompletionOnRealCastellanChamber(t *testing.T) {
	if _, err := os.Stat(catalogPath); err != nil {
		t.Skipf("dungeon catalog unavailable: %v", err)
	}
	c, err := catalog.LoadDungeons(catalogPath)
	if err != nil {
		t.Fatalf("load catalog: %v", err)
	}
	const quest = 3191
	s, err := Select(c, protocol.DungeonSelection{ID: 15, Difficulty: 2, Party: 65535, Quest: quest}, 115, map[uint16]bool{quest: true})
	if err != nil {
		t.Fatalf("select dungeon 15 quest %d: %v", quest, err)
	}
	if s.Maze.Index != 6 {
		t.Fatalf("quest %d no longer resolves to maze 6: %d", quest, s.Maze.Index)
	}
	const finalMap = 100008683
	if len(s.Maze.Layers) != 1 {
		t.Fatalf("expected a single layer entry, got %+v", s.Maze.Layers)
	}
	layer := s.Maze.Layers[0]
	if layer.Maps[len(layer.Maps)-1] != finalMap {
		t.Fatalf("final layer map is no longer %d: %v", finalMap, layer.Maps)
	}

	for i, mapID := range layer.Maps {
		last := i == len(layer.Maps)-1
		room := catalog.DungeonRoom{X: layer.Position[0], Y: layer.Position[1], Map: mapID, Boss: last}
		run, err := s.enterRoom(c, room)
		if err != nil {
			t.Fatalf("enter layer map %d: %v", mapID, err)
		}
		// Walk it the way a live client does: everything killable dies, the
		// map finishes loading, nothing else happens.
		for _, m := range run.Monsters {
			if !m.NonCombat {
				run.Dead[m.Entity] = true
			}
		}
		run.Loaded = true
		if !run.RoomCleared() {
			t.Fatalf("layer map %d refuses to clear, so the sequence cannot be walked", mapID)
		}
		run.TryComplete()
		if run.Completed() != last {
			t.Fatalf("layer map %d: completed=%v, want %v", mapID, run.Completed(), last)
		}
		if !last {
			continue
		}
		// The crux: the boss the client would check for is a non-combat dummy,
		// so no requested identity can ever exist for this run.
		if run.hasKillableBoss() {
			t.Fatal("final map now carries a killable boss; the fallback is obsolete")
		}
		dummy := false
		for _, m := range run.Monsters {
			if m.Rank == 3 && m.NonCombat {
				dummy = true
			}
		}
		if !dummy {
			t.Fatal("final map no longer carries a non-combat rank-3 display dummy")
		}
		if run.completionTarget != 0 {
			t.Fatalf("precondition: expected no requested boss identity, got %d", run.completionTarget)
		}
		target := run.CompletionTarget()
		if target == 0 || target == 65535 {
			t.Fatalf("completion target unusable for the wire payload: %d", target)
		}
		// The payload must actually encode: this is the assertion the fix exists for.
		if _, err := protocol.BossCheckConfirmed(target); err != nil {
			t.Fatalf("BossCheckConfirmed(%d) refused: %v", target, err)
		}
	}
}
