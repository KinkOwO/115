package dungeon

import (
	"dfolan/internal/catalog"
	"dfolan/internal/catalog/pvf"
	"dfolan/internal/game/protocol"
	"testing"
)

// The current nemaug.dgn declares this flag and a 3x3 maze starting at1,1.
// Live CMD45 requests at21:10:28..37 target all four adjacent rooms while
// Numak is alive. Movement must retain the encounter, not fabricate a clear.
func TestIspinsSourceMoveWithLivingBoss(t *testing.T) {
	cells := []pvf.Token{
		{Type: 3, Text: "[minimum required level]"}, {Type: 0, Value: 110},
		{Type: 3, Text: "[basis level]"}, {Type: 0, Value: 140},
		{Type: 3, Text: "[move map even enemy]"}, {Type: 0, Value: 1},
		{Type: 3, Text: "[maze info]"},
		{Type: 3, Text: "[size]"}, {Type: 0, Value: 1}, {Type: 0, Value: 1},
		{Type: 3, Text: "[map specification]"}, {Type: 6, Text: "boss"}, {Type: 0, Value: 0}, {Type: 0, Value: 0}, {Type: 0, Value: 100006472}, {Type: 3, Text: "[/map specification]"},
		{Type: 3, Text: "[start map]"}, {Type: 0, Value: 0}, {Type: 0, Value: 0}, {Type: 3, Text: "[/start map]"},
		{Type: 3, Text: "[boss map]"}, {Type: 0, Value: 0}, {Type: 0, Value: 0}, {Type: 3, Text: "[/boss map]"},
	}
	def, err := catalog.ParseDungeon(100002987, catalog.ScriptRecord{Cells: cells})
	if err != nil {
		t.Fatal(err)
	}
	maze := catalog.DungeonMaze{Start: [2]byte{1, 1}, Boss: [2]byte{0, 0}}
	c := catalog.DungeonCatalog{Maps: map[uint32]catalog.ScriptRecord{}}
	for y := byte(0); y < 3; y++ {
		for x := byte(0); x < 3; x++ {
			id := uint32(100006472) + uint32(y)*3 + uint32(x)
			maze.Rooms = append(maze.Rooms, catalog.DungeonRoom{X: x, Y: y, Map: id, Boss: x == 0 && y == 0})
			c.Maps[id] = catalog.ScriptRecord{}
		}
	}
	s, err := newSession(c, def, maze)
	if err != nil {
		t.Fatal(err)
	}
	s.Loaded = true
	s.Monsters = []protocol.DungeonMonster{{Entity: 4096, Template: 109014133, Rank: 3}}
	s.Visited[s.Room.Map] = s.Monsters
	for _, target := range [][2]byte{{0, 1}, {1, 2}, {2, 1}, {1, 0}} {
		if s.RoomCleared() {
			t.Fatal("movement permission was mistaken for a room clear")
		}
		next, err := s.Move(c, target)
		if err != nil {
			t.Fatalf("live door %v refused: %v", target, err)
		}
		if next.Loaded || next.RunID != s.RunID || next.Completed() || len(next.Dead) != 0 {
			t.Fatal("door fabricated completion or changed encounter ownership")
		}
		next.Loaded = true
		back, err := next.Move(c, [2]byte{1, 1})
		if err != nil || len(back.LivingMonsters()) != 1 || back.Monsters[0].Entity != 4096 {
			t.Fatalf("return lost original living boss: %v", err)
		}
	}
	s.Definition.MoveMapEvenEnemy = false
	if _, err := s.Move(c, [2]byte{0, 1}); err == nil {
		t.Fatal("ordinary dungeon admitted movement with living enemies")
	}
	s.Definition.MoveMapEvenEnemy = true
	s.Loaded = false
	if _, err := s.Move(c, [2]byte{0, 1}); err == nil {
		t.Fatal("unloaded room admitted")
	}
	s.Loaded = true
	for _, target := range [][2]byte{{0, 0}, {3, 1}, {1, 1}} {
		if _, err := s.Move(c, target); err == nil {
			t.Fatalf("invalid target %v admitted", target)
		}
	}
}
