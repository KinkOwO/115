package dungeon

import (
	"testing"

	"dfolan/internal/catalog"
	"dfolan/internal/game/protocol"
)

// The captured native CMD45 after the mid-dungeon cinematic boss names the
// current layer cell (2,0). The previous room is already visited; the next
// source maze room (3,0) is not.
func TestCrescentMoonLayerExitUsesUnvisitedNeighbor(t *testing.T) {
	c, err := catalog.LoadDungeons("../../configs/dungeons.full.json")
	if err != nil {
		t.Fatal(err)
	}
	d, ok := c.Dungeons[100004779]
	if !ok {
		t.Fatal("Crescent Moon Lake source dungeon missing")
	}
	maze := d.Mazes[0]
	s := &Session{
		Definition: d,
		Maze:       maze,
		Room:       catalog.DungeonRoom{X: 2, Y: 0, Map: 100008969},
		Loaded:     true,
		Visited: map[uint32][]protocol.DungeonMonster{
			100015645: nil,
			100015646: nil,
			100015647: nil,
			100008969: nil,
		},
		Dead: map[uint16]bool{},
	}
	next, err := s.MoveScene(c, protocol.DungeonRoomTransition{
		Dungeon:     100004779,
		Position:    [2]byte{2, 0},
		LayerChange: true,
		Record:      [18]byte{0, 0, 0, 0, 4, 5, 101, 2, 61, 1},
	})
	if err != nil {
		t.Fatal(err)
	}
	if next.Room.X != 3 || next.Room.Y != 0 || next.Room.Map != 100015648 {
		t.Fatalf("mid-dungeon layer returned to a visited room: %+v", next.Room)
	}
}
