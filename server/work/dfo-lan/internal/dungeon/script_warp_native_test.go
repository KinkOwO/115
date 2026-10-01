package dungeon

import (
	"dfolan/internal/catalog"
	"dfolan/internal/game/protocol"
	"reflect"
	"testing"
)

func TestScriptWarpInstalledWithoutEmbeddedJSON(t *testing.T) {
	routes, err := EmbeddedScriptWarpRoutes()
	if err != nil {
		t.Fatal(err)
	}
	want := append([]scriptWarpRoute(nil), routes...)
	for i := range want {
		want[i].RequiredKeyMaps = append([]uint32{}, routes[i].RequiredKeyMaps...)
	}
	restore, err := InstallScriptWarpRoutes(routes)
	if err != nil {
		t.Fatal(err)
	}
	defer restore()
	old, forced := scriptWarpData, forcedScriptWarpData
	scriptWarpData, forcedScriptWarpData = nil, nil
	defer func() { scriptWarpData, forcedScriptWarpData = old, forced }()
	got, err := scriptWarpRoutes()
	if err != nil || !reflect.DeepEqual(want, got) {
		t.Fatal("native snapshot depends on embedded JSON", err)
	}
	for i := range routes {
		for j := range routes[i].RequiredKeyMaps {
			routes[i].RequiredKeyMaps[j] = 0
		}
		routes[i].From = 0
	}
	got, err = scriptWarpRoutes()
	if err != nil || !reflect.DeepEqual(want, got) {
		t.Fatal("native snapshot shares mutable scope", err)
	}
	if _, err := InstallScriptWarpRoutes(routes); err == nil {
		t.Fatal("invalid routes installed")
	}
	if _, err := InstallScriptWarpRoutes(nil); err == nil {
		t.Fatal("empty routes installed")
	}
	c := odysseyScenes(t)
	for _, r := range got[:11] {
		d := c.Dungeons[r.Dungeon]
		s, e := newSession(c, d, d.Mazes[r.Maze])
		if e != nil {
			t.Fatal(e)
		}
		s, e = s.enterRoom(c, catalog.DungeonRoom{X: r.Position[0], Y: r.Position[1], Map: r.From})
		if e != nil {
			t.Fatal(e)
		}
		s.Loaded = true
		for _, id := range r.RequiredKeyMaps {
			monsters, e := fixedMonsters(c.Maps[id], d.BasisLevel)
			if e != nil {
				t.Fatal(e)
			}
			for i := range monsters {
				monsters[i].Entity = uint16(60000 + i)
				s.Dead[monsters[i].Entity] = true
			}
			s.Visited[id] = monsters
		}
		next, e := s.MoveScript(c, protocol.DungeonRoomTransition{Dungeon: r.Dungeon, Position: r.Target, Record: r.Record})
		if e != nil || next.Room.Map != r.To {
			t.Fatal("installed source move failed", r, e)
		}
	}
	TestRestingPlaceChaseForcedMove(t)
}
