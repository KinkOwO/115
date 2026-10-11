package dungeon

import (
	"bytes"
	"dfolan/internal/catalog"
	"dfolan/internal/catalog/pvf"
	"dfolan/internal/game/protocol"
	"testing"
	"time"
)

func TestMoonRevisitUsesPreMoveVisitedAndNoMonsterRows(t *testing.T) {
	now := time.Unix(1800000000, 0)
	rooms := []catalog.DungeonRoom{{X: 0, Y: 0, Map: 1}, {X: 0, Y: 1, Map: 2}, {X: 0, Y: 2, Map: 3}}
	c := catalog.DungeonCatalog{Source: pvf.ArchiveSnapshot{Checksum: "source"}, Maps: map[uint32]catalog.ScriptRecord{1: {}, 2: {}, 3: {}}}
	r := &MoonSoloOwner{session: &Session{RunID: "run", Loaded: true, MoonGridReady: true, Definition: catalog.DungeonDefinition{ID: 100004136, BasisLevel: 115}, Maze: catalog.DungeonMaze{Rooms: rooms}, Room: rooms[0], Visited: map[uint32][]protocol.DungeonMonster{1: nil}, Dead: map[uint16]bool{}}, source: "source", room: 1, phase: moonSoloRunning, timeout: time.Minute}
	for i, target := range [][2]byte{{0, 1}, {0, 2}, {0, 1}, {0, 0}} {
		if err := r.Move(c, protocol.DungeonRoomTransition{Position: target}, uint32(i), now); err != nil {
			t.Fatal(err)
		}
		got, err := r.StartMap()
		if err != nil {
			t.Fatal(err)
		}
		reused := i >= 2
		if r.reuseRoom != reused {
			t.Fatalf("move %d reuse=%v", i, r.reuseRoom)
		}
		if reused && (len(got) != 34 || !bytes.Equal(got[31:], []byte{0, 0, 255})) {
			t.Fatalf("cached room reinitializes actors: %x", got)
		}
		if !reused && len(got) == 34 {
			t.Fatal("new room incorrectly reused")
		}
		if err := r.Loaded(now); err != nil {
			t.Fatal(err)
		}
	}
	// Previously visited living rows must not be serialized a second time.
	r.session.Monsters = []protocol.DungeonMonster{{Entity: 9, Template: 101}}
	if _, err := r.StartMap(); err != nil {
		t.Fatal("reuse carried monster rows", err)
	}
}
