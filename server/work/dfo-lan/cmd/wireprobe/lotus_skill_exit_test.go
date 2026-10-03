package main

import (
	"encoding/hex"
	"testing"

	"dfolan/internal/catalog"
	"dfolan/internal/dungeon"
	"dfolan/internal/game/protocol"
)

// Role 10, 20261002_212301: native USE_SKILL 5 in the final Lotus map
// must not synthesize a return to 53543 while its cinematic still owns actors.
func TestLotusSkillKeepsTerminalCinematicRoom(t *testing.T) {
	const base, final uint32 = 53543, 100008697
	maze := catalog.DungeonMaze{Index: 3, Quest: 3215, Boss: [2]byte{3, 0},
		Rooms:  []catalog.DungeonRoom{{X: 3, Y: 0, Map: base, Boss: true}},
		Layers: []catalog.DungeonLayer{{Position: [2]byte{3, 0}, Maps: []uint32{100008786, final}}},
	}
	actors := []protocol.DungeonMonster{
		{Entity: 0x1035, SourceIndex: 0, Template: 70160, Rank: 3, Team: 100, NonCombat: true},
		{Entity: 0x1036, SourceIndex: 1, Template: 109015585, Team: 0, NonCombat: true},
		{Entity: 0x1037, SourceIndex: 2, Template: 109015474, Team: 0, NonCombat: true},
		{Entity: 0x1038, SourceIndex: 3, Template: 75099, Rank: 3, Team: 100, NonCombat: true},
	}
	run := &dungeon.Session{Definition: catalog.DungeonDefinition{ID: 26}, Maze: maze,
		Room: catalog.DungeonRoom{X: 3, Y: 0, Map: final, Boss: true}, Loaded: true,
		Monsters: actors, Visited: map[uint32][]protocol.DungeonMonster{base: {}, final: actors},
		Dead: map[uint16]bool{},
	}
	c := catalog.DungeonCatalog{Maps: map[uint32]catalog.ScriptRecord{base: {}, final: {}}}
	w := &worldSession{dungeons: &c, activeDungeon: run}
	request, err := hex.DecodeString("000500a00f9045000c000089ee307800")
	if err != nil {
		t.Fatal(err)
	}
	next, plan, err := w.interactDoor(request)
	if err != nil || next != nil || w.activeDungeon != run || run.Room.Map != final || !run.Loaded {
		t.Fatalf("skill changed the cinematic room: next=%+v err=%v", next, err)
	}
	if len(plan) != 1 || plan[0].Kind != 1 || plan[0].ID != 38 || string(plan[0].Payload) != "\x01" {
		t.Fatalf("skill must only receive its existing successful ACK, got %+v", plan)
	}
	// The source CMT's exact closing request still owns the transition.
	r := protocol.DungeonRoomTransition{Dungeon: 26, Position: [2]byte{3, 0}, LayerChange: true,
		Record: [18]byte{0, 0, 0, 0, 4, 5, 0xbd, 2, 0xe5, 0, 0, 0, 3, 0, 2, 0, 0, 0},
	}
	next, plan, err = w.moveDungeonRoomDecoded(r)
	if err != nil || next == nil || next.Room.Map != final || next.Loaded {
		t.Fatalf("native closing transition lost: next=%+v err=%v", next, err)
	}
	if len(plan) != 2 || plan[1].ID != 29 || plan[1].Payload[2] != 1 || plan[1].Payload[31] != 0 {
		t.Fatalf("closing must reuse the final layer without creating actors, got %+v", plan)
	}
	next.Loaded = true
	next.TryComplete()
	if !next.Completed() {
		t.Fatal("native closing transition no longer completes the task dungeon")
	}
}
