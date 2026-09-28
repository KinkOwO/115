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
