package dungeon

import (
	"dfolan/internal/catalog"
	"dfolan/internal/game/protocol"
	"encoding/hex"
	"testing"
)

func TestRoarRavineLiveSelectionResolvesAcceptedQuest(t *testing.T) {
	c := catalog.LoadNativeFullDungeons(t)
	p, err := hex.DecodeString("6789d7170000000000ffff000000000000000000000000000000000000000000")
	if err != nil {
		t.Fatal(err)
	}
	r, err := protocol.DecodeDungeonSelection(p)
	if err != nil {
		t.Fatal(err)
	}
	if r.ID != 400001383 || r.Quest != 0 {
		t.Fatalf("live request decoded as %+v", r)
	}
	s, err := Select(c, r, 100, map[uint16]bool{12430: true})
	if err != nil {
		t.Fatal(err)
	}
	if s.Maze.Quest != 12430 || s.Maze.Index != 0 || s.Room.Map != 400004000 || s.Maze.Boss != [2]byte{1, 0} || len(s.Maze.Rooms) != 2 {
		t.Fatalf("wrong source route: maze=%+v room=%+v", s.Maze, s.Room)
	}
	for _, room := range s.Maze.Rooms {
		script, ok := c.Maps[room.Map]
		if !ok {
			t.Fatalf("source map %d missing", room.Map)
		}
		monsters, err := fixedMonsters(script, s.Definition.BasisLevel)
		if err != nil {
			t.Fatalf("source map %d: %v", room.Map, err)
		}
		if _, err := protocol.StartMap(protocol.StartMapState{Position: [2]byte{room.X, room.Y}, Seed: 1, Map: room.Map, Monsters: monsters}); err != nil {
			t.Fatalf("source map %d cannot be encoded: %v", room.Map, err)
		}
	}
	for _, accepted := range []map[uint16]bool{nil, {12430: false}, {12431: true}} {
		if _, err := Select(c, r, 100, accepted); err == nil {
			t.Fatal("unaccepted quest-only route allowed")
		}
	}
	if _, err := Select(c, r, 99, map[uint16]bool{12430: true}); err == nil {
		t.Fatal("under-level quest-only route allowed")
	}
}

func TestQuestOnlySelectionPreservesRouteBoundaries(t *testing.T) {
	maze := func(index byte, quest uint16, id uint32) catalog.DungeonMaze {
		return catalog.DungeonMaze{Index: index, Quest: quest, Rooms: []catalog.DungeonRoom{room(0, 0, id)}}
	}
	accepted := map[uint16]bool{42: true, 43: true}
	t.Run("ordinary maze keeps priority", func(t *testing.T) {
		s, err := Select(testCatalog(maze(0, 42, 101), maze(1, 0, 100)), protocolSelection(7, 0), 10, accepted)
		if err != nil || s.Room.Map != 100 || s.Maze.Quest != 0 {
			t.Fatalf("ordinary selection changed: session=%+v err=%v", s, err)
		}
	})
	t.Run("pending ordinary maze is not a quest fallback", func(t *testing.T) {
		pending := maze(0, 0, 100)
		pending.Pending = []string{"unsupported room specification"}
		if _, err := Select(testCatalog(pending, maze(1, 42, 101)), protocolSelection(7, 0), 10, accepted); err == nil {
			t.Fatal("unresolved ordinary route silently replaced")
		}
	})
	t.Run("distinct accepted quests are ambiguous", func(t *testing.T) {
		if _, err := Select(testCatalog(maze(0, 42, 100), maze(1, 43, 101)), protocolSelection(7, 0), 10, accepted); err == nil {
			t.Fatal("ambiguous story route selected")
		}
	})
	t.Run("duplicate mazes for one quest keep selection rules", func(t *testing.T) {
		s, err := Select(testCatalog(maze(1, 42, 101), maze(0, 42, 100)), protocolSelection(7, 0), 10, accepted)
		if err != nil || s.Maze.Index != 0 || s.Maze.Quest != 42 {
			t.Fatalf("duplicate selection changed: session=%+v err=%v", s, err)
		}
	})
	t.Run("explicit quest is never replaced", func(t *testing.T) {
		c := testCatalog(maze(0, 42, 100))
		if _, err := Select(c, protocolSelection(7, 43), 10, accepted); err == nil {
			t.Fatal("explicit unmatched quest replaced")
		}
		if _, err := Select(c, protocolSelection(7, 42), 10, nil); err == nil {
			t.Fatal("explicit unaccepted quest allowed")
		}
	})
}
