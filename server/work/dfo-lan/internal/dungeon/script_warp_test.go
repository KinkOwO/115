package dungeon

import (
	"dfolan/internal/catalog"
	"dfolan/internal/game/protocol"
	"encoding/json"
	"testing"
)

func TestScriptWarpSourceRoutes(t *testing.T) {
	c := odysseyScenes(t)
	var routes []scriptWarpRoute
	if e := json.Unmarshal(scriptWarpData, &routes); e != nil {
		t.Fatal(e)
	}
	if len(routes) != 11 {
		t.Fatal(len(routes))
	}
	for _, r := range routes {
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
		request := protocol.DungeonRoomTransition{Dungeon: r.Dungeon, Position: r.Target, Record: r.Record}
		for _, id := range r.RequiredKeyMaps {
			if _, e = s.MoveScript(c, request); e == nil {
				t.Fatal("unvisited key admitted")
			}
			m, e := fixedMonsters(c.Maps[id], d.BasisLevel)
			if e != nil {
				t.Fatal(e)
			}
			for i := range m {
				m[i].Entity = uint16(60000 + i)
			}
			s.Visited[id] = m
			if _, e = s.MoveScript(c, request); e == nil {
				t.Fatal("living key boss admitted")
			}
			for _, v := range m {
				s.Dead[v.Entity] = true
			}
		}
		beforeDeaths := len(s.Dead)
		n, e := s.MoveScript(c, request)
		if e != nil || n.Room.Map != r.To {
			t.Fatal(r, e)
		}
		if len(s.Dead) != beforeDeaths || len(n.Dead) != beforeDeaths || s.ScriptWarps[r.From] || !n.ScriptWarps[r.From] {
			t.Fatal("script warp fabricated kills")
		}
		for _, change := range []func(*protocol.DungeonRoomTransition){func(v *protocol.DungeonRoomTransition) { v.Dungeon++ }, func(v *protocol.DungeonRoomTransition) { v.Position[0]++ }, func(v *protocol.DungeonRoomTransition) { v.Record[6]++ }, func(v *protocol.DungeonRoomTransition) { v.LayerChange = true }} {
			bad := request
			change(&bad)
			if _, e = s.MoveScript(c, bad); e == nil {
				t.Fatal("altered route admitted", r)
			}
		}
		if _, e = n.MoveScript(c, request); e == nil {
			t.Fatal("unloaded replay admitted")
		}
		s.Loaded = false
		if _, e = s.MoveScript(c, request); e == nil {
			t.Fatal("unloaded source admitted")
		}
		s.Loaded = true
		bad := c
		bad.Source.Checksum = "altered"
		if _, e = s.MoveScript(bad, request); e == nil {
			t.Fatal("source mismatch admitted")
		}
	}
}

func TestUnderfootKeyBossAndStageRevisit(t *testing.T) {
	c := odysseyScenes(t)
	s, e := Select(c, protocol.DungeonSelection{ID: 100004940, Difficulty: 2, Party: 65535}, 40, nil)
	if e != nil {
		t.Fatal(e)
	}
	s, e = s.enterRoom(c, catalog.DungeonRoom{X: 2, Y: 3, Map: 100016039})
	if e != nil {
		t.Fatal(e)
	}
	s.Loaded = true
	if s.RoomCleared() {
		t.Fatal("dummy flag bypassed key boss")
	}
	if _, e = s.Move(c, [2]byte{3, 3}); e == nil {
		t.Fatal("key boss room escaped before death")
	}
	for _, m := range s.Monsters {
		s.Dead[m.Entity] = true
	}
	s, e = s.Move(c, [2]byte{3, 3})
	if e != nil {
		t.Fatal(e)
	}
	s.Loaded = true
	var routes []scriptWarpRoute
	json.Unmarshal(scriptWarpData, &routes)
	var warp scriptWarpRoute
	for _, r := range routes {
		if r.From == 100016040 {
			warp = r
		}
	}
	s, e = s.MoveScript(c, protocol.DungeonRoomTransition{Dungeon: s.Definition.ID, Position: warp.Target, Record: warp.Record})
	if e != nil {
		t.Fatal(e)
	}
	s.Loaded = true
	for _, m := range s.Monsters {
		s.Dead[m.Entity] = true
	}
	for _, r := range c.SceneRoutes {
		if r.From == 100016035 {
			s, e = s.MoveScene(c, protocol.DungeonRoomTransition{Dungeon: s.Definition.ID, Position: r.Position, LayerChange: true, Record: r.Record})
			if e != nil {
				t.Fatal(e)
			}
			break
		}
	}
	if s.Room.Map != 100016041 {
		t.Fatal("missing post-elite layer")
	}
	s.Loaded = true
	for _, m := range s.Monsters {
		s.Dead[m.Entity] = true
	}
	oldRun := s.RunID
	s, e = s.Move(c, [2]byte{0, 2})
	if e != nil {
		t.Fatal(e)
	}
	s.Loaded = true
	for _, m := range s.Monsters {
		s.Dead[m.Entity] = true
	}
	s, e = s.Move(c, [2]byte{1, 2})
	if e != nil {
		t.Fatal(e)
	}
	if s.Room.Map != 100016041 || s.RunID != oldRun {
		t.Fatal("revisited introductory layer or reset run")
	}
	s.Loaded = true
	for _, r := range routes {
		if r.From == 100016041 {
			n, e := s.MoveScript(c, protocol.DungeonRoomTransition{Dungeon: s.Definition.ID, Position: r.Target, Record: r.Record})
			if e != nil || n.Room.Map != 100016042 {
				t.Fatal("boss warp", e)
			}
		}
	}
	t.Log("stage chain: key boss death -> forced return -> source next layer -> revisit retains layer -> boss room")
}
