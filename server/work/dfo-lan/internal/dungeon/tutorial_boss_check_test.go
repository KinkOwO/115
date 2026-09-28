package dungeon

import (
	"testing"

	"dfolan/internal/catalog"
	"dfolan/internal/game/protocol"
)

func TestTutorialBossCheckUsesSourceTerminalCoordinate(t *testing.T) {
	run := &Session{
		Definition: catalog.DungeonDefinition{ID: 7112, Tutorial: true},
		Maze:       catalog.DungeonMaze{Boss: [2]byte{4, 0}},
		Room:       catalog.DungeonRoom{X: 4, Y: 0, Map: 53126},
		Monsters:   []protocol.DungeonMonster{{Entity: 0x100e, Rank: 3, Team: 100}},
		Loaded:     true,
		Dead:       map[uint16]bool{},
	}

	if err := run.BossCheck(protocol.BossCheckRequest{Actor: 8, Target: 0x100e}, 8); err != nil {
		t.Fatalf("source terminal boss was rejected without the room marker: %v", err)
	}
	if run.Completed() {
		t.Fatal("tutorial completed before the source boss death was confirmed")
	}
}

func TestTutorialBossCheckStillRequiresTerminalCoordinate(t *testing.T) {
	run := &Session{
		Definition: catalog.DungeonDefinition{ID: 7112, Tutorial: true},
		Maze:       catalog.DungeonMaze{Boss: [2]byte{4, 0}},
		Room:       catalog.DungeonRoom{X: 3, Y: 0, Map: 53125},
		Monsters:   []protocol.DungeonMonster{{Entity: 0x100e, Rank: 3, Team: 100}},
		Loaded:     true,
		Dead:       map[uint16]bool{},
	}

	if err := run.BossCheck(protocol.BossCheckRequest{Actor: 8, Target: 0x100e}, 8); err == nil {
		t.Fatal("rank-3 monster outside the tutorial terminal coordinate was accepted")
	}
}

func TestOrdinaryBossCheckStillRequiresRoomMarker(t *testing.T) {
	run := &Session{
		Definition: catalog.DungeonDefinition{ID: 7},
		Maze:       catalog.DungeonMaze{Boss: [2]byte{4, 0}},
		Room:       catalog.DungeonRoom{X: 4, Y: 0, Map: 76166},
		Monsters:   []protocol.DungeonMonster{{Entity: 0x100e, Rank: 3, Team: 100}},
		Loaded:     true,
		Dead:       map[uint16]bool{},
	}

	if err := run.BossCheck(protocol.BossCheckRequest{Actor: 8, Target: 0x100e}, 8); err == nil {
		t.Fatal("ordinary room without the source boss marker was accepted")
	}
}

func TestTutorialWithoutBossCompletesOnClearedTerminalMap(t *testing.T) {
	run := &Session{
		Definition: catalog.DungeonDefinition{ID: 7115, Tutorial: true},
		Maze: catalog.DungeonMaze{
			Boss:  [2]byte{3, 0},
			Rooms: []catalog.DungeonRoom{{X: 3, Y: 0, Map: 53130, Boss: true}},
		},
		Room:     catalog.DungeonRoom{X: 3, Y: 0, Map: 53130, Boss: true},
		Monsters: []protocol.DungeonMonster{{Entity: 0x100e, Rank: 0, Team: 100}},
		Loaded:   true,
		Dead:     map[uint16]bool{},
	}

	run.TryComplete()
	if run.Completed() {
		t.Fatal("tutorial completed while a terminal-room enemy was alive")
	}
	if _, err := run.ConfirmDeath(0x100e, 8, 8); err != nil {
		t.Fatal(err)
	}
	if !run.Completed() {
		t.Fatal("cleared tutorial terminal room did not complete without a boss check")
	}
	if run.CompletionNeedsBossCheck() || run.CompletionTarget() != 0 {
		t.Fatal("boss-less tutorial completion invented a boss identity")
	}
}

func TestTutorialRoomBeforeTerminalDoesNotCompleteOnClear(t *testing.T) {
	run := &Session{
		Definition: catalog.DungeonDefinition{ID: 100003327, Tutorial: true},
		Maze: catalog.DungeonMaze{
			Boss:  [2]byte{4, 0},
			Rooms: []catalog.DungeonRoom{{X: 1, Y: 0, Map: 100008880}},
		},
		Room:   catalog.DungeonRoom{X: 1, Y: 0, Map: 100008880},
		Loaded: true,
		Dead:   map[uint16]bool{},
	}
	run.TryComplete()
	if run.Completed() {
		t.Fatal("tutorial completed outside its terminal coordinate")
	}
}

func TestSwordmanTutorialCompletesOnItsClearedTerminalMap(t *testing.T) {
	c, err := catalog.LoadDungeons("../../configs/tutorial-dungeons.current36.json")
	if err != nil {
		t.Fatal(err)
	}
	d := c.Dungeons[7115]
	maze := d.Mazes[0]
	var room catalog.DungeonRoom
	for _, candidate := range maze.Rooms {
		if [2]byte{candidate.X, candidate.Y} == maze.Boss {
			room = candidate
			break
		}
	}
	if room.Map != 53130 {
		t.Fatalf("swordman tutorial terminal map = %d, want 53130", room.Map)
	}
	monsters, err := fixedMonsters(c.Maps[room.Map], d.BasisLevel)
	if err != nil {
		t.Fatal(err)
	}
	run := &Session{
		Definition: d,
		Maze:       maze,
		Room:       room,
		Monsters:   monsters,
		Dead:       map[uint16]bool{},
		Loaded:     true,
	}
	enemies := 0
	for _, monster := range monsters {
		if monster.Team == 0 || monster.NonCombat {
			continue
		}
		enemies++
		if _, err := run.ConfirmDeath(uint32(monster.Entity), 8, 8); err != nil {
			t.Fatal(err)
		}
	}
	if enemies == 0 {
		t.Fatal("swordman terminal map has no source enemies to clear")
	}
	if !run.Completed() {
		t.Fatal("cleared swordman tutorial terminal map did not complete without a boss identity")
	}
	if run.CompletionNeedsBossCheck() || run.CompletionTarget() != 0 {
		t.Fatal("boss-less swordman tutorial completion requested a boss check")
	}
}
