package dungeon

import (
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"dfolan/internal/catalog"
	"dfolan/internal/catalog/pvf"
	"dfolan/internal/game/protocol"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"testing"
)

// Historical, immutable test-only subset. Runtime still reads the native PVF.
func tianqingDungeonFixture(t *testing.T) string {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("testdata", "tianqing-postboss.json.gz"))
	if err != nil {
		t.Fatal(err)
	}
	if got := fmt.Sprintf("%x", sha256.Sum256(raw)); got != "0b8ced5e5959eb4fbd818de31028231d014d2a2b19847e1d815c9bdcd5a0cbc9" {
		t.Fatalf("historical fixture changed: %s", got)
	}
	r, err := gzip.NewReader(bytes.NewReader(raw))
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	body, err := io.ReadAll(r)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "dungeons.json")
	if err = os.WriteFile(path, body, 0600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestTianqingPostBossPlainRoomBoundaries(t *testing.T) {
	s := &Session{
		Room: catalog.DungeonRoom{X: 1, Y: 1, Map: 10},
		Maze: catalog.DungeonMaze{Rooms: []catalog.DungeonRoom{
			{X: 2, Y: 1, Map: 11}, {X: 3, Y: 1, Map: 12},
		}},
		Visited: map[uint32][]protocol.DungeonMonster{10: nil},
	}
	if !s.adjacentUnvisitedRoom([2]byte{2, 1}) {
		t.Fatal("source-adjacent unvisited room refused")
	}
	for _, pos := range [][2]byte{{3, 1}, {1, 2}} {
		if s.adjacentUnvisitedRoom(pos) {
			t.Fatalf("non-adjacent or absent source room accepted: %v", pos)
		}
	}
	s.Visited[11] = nil
	if s.adjacentUnvisitedRoom([2]byte{2, 1}) {
		t.Fatal("visited empty room accepted")
	}
	delete(s.Visited, 11)
	s.Maze.Layers = []catalog.DungeonLayer{{Position: [2]byte{0, 0}, Maps: []uint32{20}}}
	if s.adjacentUnvisitedRoom([2]byte{2, 1}) {
		t.Fatal("plain-room exception widened to a layered maze")
	}
}

func TestTianqingFinishedLayerDoesNotReplayAfterCompletion(t *testing.T) {
	s := &Session{
		Definition: catalog.DungeonDefinition{ID: 1},
		Room:       catalog.DungeonRoom{X: 1, Y: 1, Map: 11}, Loaded: true, completed: true,
		Maze:    catalog.DungeonMaze{Layers: []catalog.DungeonLayer{{Position: [2]byte{1, 1}, Maps: []uint32{11}}}},
		Visited: map[uint32][]protocol.DungeonMonster{11: nil},
	}
	if s.layerPendingUnvisited([2]byte{1, 1}) {
		t.Fatal("visited empty layer considered pending")
	}
	if _, err := s.MoveScene(catalog.DungeonCatalog{}, protocol.DungeonRoomTransition{Dungeon: 1, Position: [2]byte{1, 1}, LayerChange: true}); err == nil {
		t.Fatal("completed source layer replay accepted")
	}
}

func TestTianqingSpecialPassiveSectionBoundaries(t *testing.T) {
	section := func(text string) pvf.Token { return pvf.Token{Type: 3, Text: text} }
	number := pvf.Token{Type: 0, Value: 123}
	for _, tc := range []struct {
		name  string
		cells []pvf.Token
		want  bool
	}{
		{"trap", []pvf.Token{section("[special passive object]"), number}, true},
		{"empty", []pvf.Token{section("[special passive object]"), section("[/special passive object]")}, false},
		{"number-outside", []pvf.Token{section("[monster]"), number}, false},
		{"closed-section", []pvf.Token{section("[special passive object]"), section("[/special passive object]"), number}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := hasSpecialPassiveObject(catalog.ScriptRecord{Cells: tc.cells}); got != tc.want {
				t.Fatalf("source section=%v want %v", got, tc.want)
			}
		})
	}
}

func TestTianqingSpecialPassiveSettlementRequiresConfirmedFight(t *testing.T) {
	c, err := catalog.LoadDungeons(tianqingDungeonFixture(t))
	if err != nil {
		t.Fatal(err)
	}
	def := c.Dungeons[88]
	maze := def.Mazes[0]
	var boss catalog.DungeonRoom
	for _, room := range maze.Rooms {
		if room.Boss && [2]byte{room.X, room.Y} == maze.Boss {
			boss = room
		}
	}
	for _, name := range []string{"not-loaded", "boss-alive", "minor-enemy-alive", "missing-source-passive", "foreign-room", "no-fightable-boss"} {
		t.Run(name, func(t *testing.T) {
			s, err := Select(c, protocol.DungeonSelection{ID: 88, Difficulty: 1, Party: 65535}, 57, nil)
			if err != nil {
				t.Fatal(err)
			}
			s, err = s.enterRoom(c, boss)
			if err != nil {
				t.Fatal(err)
			}
			s.Loaded = true
			for _, m := range s.Monsters {
				s.Dead[m.Entity] = true
			}
			if !s.BossRoomSpecialPassive || !s.hasFightableBoss() || !s.roomEnemiesDead() {
				t.Fatal("source fixture lacks expected real boss/passive/deaths")
			}
			switch name {
			case "not-loaded":
				s.Loaded = false
			case "boss-alive":
				for _, m := range s.Monsters {
					if m.Rank == 3 && !m.NonCombat && m.Team != 0 {
						delete(s.Dead, m.Entity)
					}
				}
			case "minor-enemy-alive":
				found := false
				for _, m := range s.Monsters {
					if m.Rank != 3 && !m.NonCombat && m.Team != 0 {
						delete(s.Dead, m.Entity)
						found = true
						break
					}
				}
				if !found {
					t.Fatal("fixture lost minor enemy")
				}
			case "missing-source-passive":
				s.BossRoomSpecialPassive = false
			case "foreign-room":
				s.Room.X++
			case "no-fightable-boss":
				for i := range s.Monsters {
					if s.Monsters[i].Rank == 3 {
						// An all-minor-enemy room must not use the new boss rule.
						// A display boss has its own pre-existing settlement path.
						s.Monsters[i].Rank = 2
					}
				}
			}
			s.TryComplete()
			if s.Completed() {
				t.Fatal("unconfirmed fight settled")
			}
		})
	}
}
