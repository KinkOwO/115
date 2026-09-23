package dungeon

import (
	"dfolan/internal/catalog"
	"dfolan/internal/game/protocol"
	"testing"
)

func TestLotusClosingCinematicReusesFinalLayer(t *testing.T) {
	const finalMap uint32 = 100008697
	monsters := []protocol.DungeonMonster{
		{Entity: 0x102d, Template: 70160, Team: 100},
		{Entity: 0x102e, Template: 109015585, Team: 0, Rank: 3, NonCombat: true},
		{Entity: 0x1030, Template: 75099, Team: 100, Rank: 3, NonCombat: true},
	}
	s := &Session{
		Definition: catalog.DungeonDefinition{ID: 26},
		Maze:       catalog.DungeonMaze{Index: 3, Layers: []catalog.DungeonLayer{{Position: [2]byte{3, 0}, Maps: []uint32{100008786, finalMap}}}},
		Room:       catalog.DungeonRoom{X: 3, Y: 0, Map: finalMap, Boss: true},
		Loaded:     true,
		Monsters:   monsters,
		Visited:    map[uint32][]protocol.DungeonMonster{finalMap: monsters},
		Dead:       map[uint16]bool{0x102d: true},
	}
	r := protocol.DungeonRoomTransition{
		Dungeon: 26, Position: [2]byte{3, 0}, LayerChange: true,
		Record: [18]byte{0, 0, 0, 0, 4, 5, 0xbd, 2, 0xe5, 0, 0, 0, 3, 0, 2, 0, 0, 0},
	}
	delete(s.Dead, 0x102d)
	if _, err := s.MoveScene(catalog.DungeonCatalog{}, r); err == nil {
		t.Fatal("closing scene accepted while the combat target was alive")
	}
	s.Dead[0x102d] = true
	s.TryComplete()
	if s.Completed() {
		t.Fatal("first arrival on the final map completed before the closing scene")
	}
	next, err := s.MoveScene(catalog.DungeonCatalog{}, r)
	if err != nil || next.Room.Map != finalMap || next.Loaded || !next.Dead[0x102d] {
		t.Fatalf("closing scene must revisit cached final map without respawn: next=%+v err=%v", next, err)
	}
	next.TryComplete()
	if next.Completed() {
		t.Fatal("closing scene completed before loading confirmation")
	}
	next.Loaded = true
	next.TryComplete()
	if !next.Completed() || next.CompletionTarget() != 0x1030 {
		t.Fatalf("closing scene should complete with the source hostile display boss identity: completed=%v target=%x", next.Completed(), next.CompletionTarget())
	}
	if _, err := protocol.BossCheckConfirmed(next.CompletionTarget()); err != nil {
		t.Fatalf("closing scene identity rejected by native confirmation encoder: %v", err)
	}
	for _, bad := range []protocol.DungeonRoomTransition{
		{Dungeon: 26, Position: [2]byte{3, 0}, LayerChange: true},
		{Dungeon: 26, Position: [2]byte{3, 0}, LayerChange: true, Record: [18]byte{0, 0, 0, 0, 4, 5, 0xbc, 2, 0xe5, 0, 0, 0, 3, 0, 2}},
	} {
		if _, err := s.MoveScene(catalog.DungeonCatalog{}, bad); err == nil {
			t.Fatalf("accepted unrelated terminal transition: %+v", bad)
		}
	}
}
