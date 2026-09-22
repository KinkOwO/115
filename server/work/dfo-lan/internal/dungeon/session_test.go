package dungeon

import (
	"dfolan/internal/catalog"
	"dfolan/internal/catalog/pvf"
	"dfolan/internal/game/protocol"
	"testing"
)

func TestMirkwoodSourceQuestRoute(t *testing.T) {
	c, e := catalog.LoadDungeons("../../configs/dungeons.generated.json")
	if e != nil {
		t.Fatal(e)
	}
	r := protocol.DungeonSelection{ID: 3, Party: 65535, Quest: 3145}
	s, e := Select(c, r, 1, map[uint16]bool{3145: true})
	if e != nil {
		t.Fatal(e)
	}
	if s.Maze.Index != 1 || len(s.Maze.Rooms) != 5 || s.Room.Map != 76121 || len(s.Monsters) != 4 {
		t.Fatalf("wrong source quest route: %+v", s.Maze)
	}
	for i, m := range s.Monsters {
		if m.SourceIndex != uint32(i) || m.Template != 109014858 || m.Level != 3 {
			t.Fatalf("source spawn changed: %+v", m)
		}
	}
	boss := false
	for _, room := range s.Maze.Rooms {
		boss = boss || room.Boss && room.Map == 76126
	}
	if !boss {
		t.Fatal("quest clear target missing")
	}
	if _, e = Select(c, r, 1, nil); e == nil {
		t.Fatal("accepted another character's quest")
	}
	if _, e = Select(c, r, 0, map[uint16]bool{3145: true}); e == nil {
		t.Fatal("accepted below source level")
	}
	r.Mode = 1
	if _, e = Select(c, r, 1, map[uint16]bool{3145: true}); e == nil {
		t.Fatal("accepted unsupported mode")
	}
}

func TestFriendlyAPCCarriesWithDynamicNativeSource(t *testing.T) {
	apcMap := catalog.ScriptRecord{Cells: []pvf.Token{
		{Type: 3, Text: "[ai character]"},
		{Type: 0, Value: 6517}, {Type: 0, Value: 100}, {Type: 0, Value: 200}, {Type: 0, Value: 0},
		{Type: 6, Text: "[character]"}, {Type: 6, Text: "[normal]"},
		{Type: 0, Value: 0}, {Type: 0, Value: 0},
		{Type: 3, Text: "[/ai character]"},
	}}
	emptyMap := catalog.ScriptRecord{}
	maze := catalog.DungeonMaze{Index: 0, Start: [2]byte{0, 0}, Rooms: []catalog.DungeonRoom{
		{X: 0, Y: 0, Map: 1}, {X: 1, Y: 0, Map: 2},
	}}
	d := catalog.DungeonDefinition{ID: 99, BasisLevel: 10, Mazes: []catalog.DungeonMaze{maze}}
	c := catalog.DungeonCatalog{Dungeons: map[uint32]catalog.DungeonDefinition{99: d}, Maps: map[uint32]catalog.ScriptRecord{1: apcMap, 2: emptyMap}}
	s, err := newSession(c, d, maze)
	if err != nil {
		t.Fatal(err)
	}
	if len(s.Monsters) != 1 || len(s.companions) != 1 || s.Monsters[0].SourceIndex != 0 {
		t.Fatalf("native companion was not harvested: %+v", s.Monsters)
	}
	s.Loaded = true
	next, err := s.Move(c, [2]byte{1, 0})
	if err != nil {
		t.Fatal(err)
	}
	if len(next.Monsters) != 1 {
		t.Fatalf("carried companions=%d", len(next.Monsters))
	}
	got := next.Monsters[0]
	if !got.APC || !got.NonCombat || got.Team != 0 || got.Template != 6517 || got.SourceIndex != dynamicAPCSourceIndex || got.Entity == s.Monsters[0].Entity {
		t.Fatalf("bad carried companion: %+v", got)
	}
	if _, err = protocol.StartMap(protocol.StartMapState{Map: next.Room.Map, Monsters: next.Monsters}); err != nil {
		t.Fatalf("dynamic APC row rejected: %v", err)
	}
}

func TestRoomClearOwnershipAndBacktracking(t *testing.T) {
	c, e := catalog.LoadDungeons("../../configs/dungeons.generated.json")
	if e != nil {
		t.Fatal(e)
	}
	s, e := Select(c, protocol.DungeonSelection{ID: 3, Party: 65535, Quest: 3145}, 1, map[uint16]bool{3145: true})
	if e != nil {
		t.Fatal(e)
	}
	if _, e = s.ConfirmDeath(4096, 3, 3); e == nil {
		t.Fatal("kill before room load accepted")
	}
	s.Loaded = true
	if _, e = s.Move(c, [2]byte{1, 1}); e == nil {
		t.Fatal("live room opened")
	}
	if _, e = s.ConfirmDeath(4096, 4, 3); e == nil {
		t.Fatal("foreign actor killed monster")
	}
	if _, e = s.ConfirmDeath(100, 3, 3); e == nil {
		t.Fatal("unknown monster accepted")
	}
	for _, m := range s.Monsters {
		fresh, e := s.ConfirmDeath(uint32(m.Entity), 3, 3)
		if e != nil || !fresh {
			t.Fatal(e)
		}
		fresh, e = s.ConfirmDeath(uint32(m.Entity), 3, 3)
		if e != nil || fresh {
			t.Fatal("duplicate kill counted")
		}
	}
	if _, e = s.Move(c, [2]byte{3, 0}); e == nil {
		t.Fatal("nonadjacent move accepted")
	}
	next, e := s.Move(c, [2]byte{1, 1})
	if e != nil {
		t.Fatal(e)
	}
	if next.Room.Map != 76123 || next.Loaded || next.RunID != s.RunID {
		t.Fatal("run/map state changed incorrectly")
	}
	next.Loaded = true
	chained := uint16(0)
	for _, m := range next.Monsters {
		if m.NonCombat {
			fresh, e := next.ConfirmDeath(uint32(m.Entity), 65535, 3)
			if e != nil || !fresh {
				t.Fatal("source cinematic sentinel", e)
			}
			continue
		}
		// A killer that names some other actor stays foreign and refused.
		if _, e := next.ConfirmDeath(uint32(m.Entity), 9, 3); e == nil {
			t.Fatal("foreign named killer accepted")
		}
		// A boss clearing its own room reports killerFFFF for the ordinary
		// monsters that go with it (live capture 20260912T001341). That
		// death has to be confirmed or the client never removes them and the
		// gate stays shut, but it is unowned: no loot, no experience.
		if chained == 0 {
			fresh, e := next.ConfirmDeath(uint32(m.Entity), 65535, 3)
			if e != nil || !fresh || !next.Unowned[m.Entity] {
				t.Fatal("boss chain death refused", e)
			}
			chained = m.Entity
		}
	}
	if chained == 0 {
		t.Fatal("room had no combat monster")
	}
	for _, m := range next.Monsters {
		if m.Entity < 4100 {
			t.Fatal("reused entity from prior room")
		}
		if _, e = next.ConfirmDeath(uint32(m.Entity), 3, 3); e != nil {
			t.Fatal(e)
		}
		if m.Entity != chained && !m.NonCombat && next.Unowned[m.Entity] {
			t.Fatal("owned kill marked unowned")
		}
	}
	back, e := next.Move(c, [2]byte{0, 1})
	if e != nil {
		t.Fatal(e)
	}
	if len(back.LivingMonsters()) != 0 {
		t.Fatal("backtracking respawned defeated monsters")
	}
}

func TestCompleteSourceRoutePreservesCinematicActors(t *testing.T) {
	c, err := catalog.LoadDungeons("../../configs/dungeons.generated.json")
	if err != nil {
		t.Fatal(err)
	}
	s, err := Select(c, protocol.DungeonSelection{ID: 3, Party: 65535, Quest: 3145}, 1, map[uint16]bool{3145: true})
	if err != nil {
		t.Fatal(err)
	}
	rooms := []struct {
		xy              [2]byte
		id              uint32
		actors, enemies int
	}{
		{[2]byte{0, 1}, 76121, 4, 4}, {[2]byte{1, 1}, 76123, 5, 4},
		{[2]byte{1, 0}, 76124, 6, 6}, {[2]byte{2, 0}, 76125, 4, 4}, {[2]byte{3, 0}, 76126, 8, 5},
	}
	seen := map[uint16]bool{}
	for i, room := range rooms {
		if i > 0 {
			s, err = s.Move(c, room.xy)
			if err != nil {
				t.Fatalf("map %d: %v", room.id, err)
			}
		}
		if s.Room.Map != room.id || len(s.Monsters) != room.actors || s.RoomCleared() {
			t.Fatalf("incorrect unloaded room %d", room.id)
		}
		s.Loaded = true
		enemies := 0
		for index, m := range s.Monsters {
			if room.id == 76123 && index == 0 && (m.Team != 0 || !m.NonCombat) {
				t.Fatal("cinematic friend lost its source team")
			}
			if !m.NonCombat && m.Team != 100 {
				t.Fatal("enemy affiliation changed")
			}
			if seen[m.Entity] || m.SourceIndex != uint32(index) {
				t.Fatal("source index changed or runtime identity reused")
			}
			seen[m.Entity] = true
			if m.NonCombat {
				continue
			}
			enemies++
			if s.RoomCleared() {
				t.Fatal("cleared before last enemy confirmation")
			}
			if _, err = s.ConfirmDeath(uint32(m.Entity), 503, 503); err != nil {
				t.Fatal(err)
			}
		}
		if enemies != room.enemies || !s.RoomCleared() {
			t.Fatalf("map %d enemies=%d clear=%v", room.id, enemies, s.RoomCleared())
		}
	}
	if s.Monsters[0].Template != 107000903 || s.Monsters[0].Rank != 3 || s.Monsters[0].NonCombat {
		t.Fatal("source boss lost its rank or combat role")
	}
	if s.Monsters[2].Template != 75099 || s.Monsters[2].Rank != 3 || !s.Monsters[2].NonCombat {
		t.Fatal("source display dummy must remain spawned but not block clear")
	}
}
