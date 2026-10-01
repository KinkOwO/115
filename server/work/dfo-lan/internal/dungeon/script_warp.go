package dungeon

import (
	"dfolan/internal/catalog"
	"dfolan/internal/game/protocol"
	_ "embed"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sync/atomic"
)

// Source-linked CMT -> passive-object custom action -> dungeon warp condition.
//
//go:embed script_warp_routes.json
var scriptWarpData []byte

// Source-backed forced moves outside Odyssey. These routes still require an
// exact dungeon/map source identity and the native transition record.
//
//go:embed forced_script_warp_routes.json
var forcedScriptWarpData []byte

func scriptWarpRoutes() ([]scriptWarpRoute, error) {
	if installed := installedScriptWarps.Load(); installed != nil {
		return installed.routes, nil
	}
	return EmbeddedScriptWarpRoutes()
}

func EmbeddedScriptWarpRoutes() ([]catalog.ScriptWarpRoute, error) {
	var routes, forced []scriptWarpRoute
	if err := json.Unmarshal(scriptWarpData, &routes); err != nil {
		return nil, err
	}
	if err := json.Unmarshal(forcedScriptWarpData, &forced); err != nil {
		return nil, err
	}
	return append(routes, forced...), nil
}

type scriptWarpRoute = catalog.ScriptWarpRoute

type scriptWarpSnapshot struct{ routes []scriptWarpRoute }

var installedScriptWarps atomic.Pointer[scriptWarpSnapshot]

func InstallScriptWarpRoutes(routes []catalog.ScriptWarpRoute) (func(), error) {
	if len(routes) == 0 {
		return nil, fmt.Errorf("empty native script warp routes")
	}
	owned := append([]scriptWarpRoute(nil), routes...)
	seen := map[[3]uint32]bool{}
	for i, r := range owned {
		key := [3]uint32{r.Dungeon, uint32(r.Maze), r.From}
		validHash := func(s string) bool { b, e := hex.DecodeString(s); return e == nil && len(b) == 32 }
		if seen[key] || !validHash(r.Source) || r.Source != owned[0].Source || r.Dungeon == 0 || r.From == 0 || r.To == 0 || r.From == r.To || !validHash(r.MapSHA256) || !validHash(r.DungeonSHA256) || !validHash(r.ActionSHA256) || r.ActionPath == "" || r.Record[0] != 1 || r.Record[1] != 0 || r.Record[2] != 0 || r.Record[3] != 0 || r.Record[4] != 5 || r.Record[5] != 5 || (r.CinematicPath != "" && (!validHash(r.CinematicSHA256) || !validHash(r.ObjectSHA256))) {
			return nil, fmt.Errorf("invalid native script warp %v", key)
		}
		seen[key] = true
		owned[i].RequiredKeyMaps = append([]uint32{}, r.RequiredKeyMaps...)
	}
	previous := installedScriptWarps.Swap(&scriptWarpSnapshot{owned})
	return func() { installedScriptWarps.Store(previous) }, nil
}

func (s *Session) MoveScript(c catalog.DungeonCatalog, r protocol.DungeonRoomTransition) (*Session, error) {
	if s == nil || !s.Loaded || s.Completed() || s.completionTarget != 0 || r.LayerChange || r.Dungeon != s.Definition.ID {
		return nil, fmt.Errorf("script warp requires owned loaded room")
	}
	routes, err := scriptWarpRoutes()
	if err != nil {
		return nil, err
	}
	for _, route := range routes {
		if route.Dungeon != s.Definition.ID || route.Maze != s.Maze.Index || route.From != s.Room.Map || route.Position != [2]byte{s.Room.X, s.Room.Y} || route.Target != r.Position || route.Record != r.Record {
			continue
		}
		if route.Source != c.Source.Checksum || route.DungeonSHA256 != s.Definition.Script.SHA256 || route.MapSHA256 != c.Maps[s.Room.Map].SHA256 {
			return nil, fmt.Errorf("script warp source mismatch")
		}
		// Forced cinematic moves may leave live scripted actors, but never bypass
		// an unfinished key boss encountered earlier in this same run.
		for _, id := range route.RequiredKeyMaps {
			monsters, visited := s.Visited[id]
			if !visited {
				return nil, fmt.Errorf("script warp key room not visited")
			}
			for _, m := range monsters {
				if m.Rank == 3 && !s.Dead[m.Entity] {
					return nil, fmt.Errorf("script warp key boss still alive")
				}
			}
		}
		for _, room := range s.Maze.Rooms {
			if room.Map == route.To && [2]byte{room.X, room.Y} == r.Position {
				next, err := s.enterRoom(c, s.latestLayer(room))
				if err != nil {
					return nil, err
				}
				next.ScriptWarps = map[uint32]bool{}
				for id, done := range s.ScriptWarps {
					next.ScriptWarps[id] = done
				}
				next.ScriptWarps[route.From] = true
				return next, nil
			}
		}
		return nil, fmt.Errorf("script warp target absent from source maze")
	}
	return nil, fmt.Errorf("no matching source script warp")
}

func (s *Session) warpKeyRoom() bool {
	routes, err := scriptWarpRoutes()
	if err != nil {
		return false
	}
	for _, r := range routes {
		if r.Dungeon != s.Definition.ID || r.Maze != s.Maze.Index {
			continue
		}
		for _, id := range r.RequiredKeyMaps {
			if id == s.Room.Map {
				return true
			}
		}
	}
	return false
}

func (s *Session) scriptStageReady() bool {
	routes, err := scriptWarpRoutes()
	if err != nil {
		return false
	}
	for _, r := range routes {
		if r.Dungeon == s.Definition.ID && r.Maze == s.Maze.Index && r.To == s.Room.Map && len(r.RequiredKeyMaps) > 0 && !s.ScriptWarps[r.From] {
			return false
		}
	}
	return true
}

func (s *Session) latestLayer(room catalog.DungeonRoom) catalog.DungeonRoom {
	if resumed, ok := s.sceneBaseRooms[[2]byte{room.X, room.Y}]; ok {
		room.Map = resumed
		return room
	}
	for _, layer := range s.Maze.Layers {
		if layer.Position == [2]byte{room.X, room.Y} {
			for _, id := range layer.Maps {
				if _, visited := s.Visited[id]; visited {
					room.Map = id
				}
			}
		}
	}
	return room
}
