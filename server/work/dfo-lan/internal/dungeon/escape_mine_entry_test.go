package dungeon

import (
	"dfolan/internal/catalog"
	"dfolan/internal/game/protocol"
	"path/filepath"
	"testing"
)

func TestEscapeMineQuest3354EntersSourceMaze(t *testing.T) {
	c, err := catalog.LoadDungeons(filepath.Join("..", "..", "configs", "dungeons.full.json"))
	if err != nil {
		t.Fatal(err)
	}
	r := protocol.DungeonSelection{ID: 53, Difficulty: 1, Party: 65535, Quest: 3354}
	s, err := Select(c, r, 49, map[uint16]bool{3354: true})
	if err != nil {
		t.Fatal(err)
	}
	if s.Maze.Index != 4 || s.Maze.Size != [2]byte{4, 4} || s.Room.Map != 76407 || len(s.Maze.Rooms) != 7 || len(s.Maze.Pending) != 0 {
		t.Fatalf("quest 3354 source maze unresolved: %+v", s.Maze)
	}
	for _, room := range s.Maze.Rooms {
		if _, ok := c.Maps[room.Map]; !ok {
			t.Fatalf("quest 3354 map %d missing from catalog", room.Map)
		}
	}
}
