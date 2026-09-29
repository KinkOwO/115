package dungeon

import (
	"dfolan/internal/catalog"
	"dfolan/internal/game/protocol"
	_ "embed"
	"encoding/json"
	"fmt"
)

// Source-linked CMT -> passive-object custom action -> dungeon warp condition.
//
//go:embed script_warp_routes.json
var scriptWarpData []byte

// Source-backed forced moves outside Odyssey. These routes still require an
// exact dungeon/map source identity and the native transition record.
//go:embed forced_script_warp_routes.json
var forcedScriptWarpData []byte

func scriptWarpRoutes() ([]scriptWarpRoute, error) {
	var routes, forced []scriptWarpRoute
	if err := json.Unmarshal(scriptWarpData, &routes); err != nil {
		return nil, err
	}
	if err := json.Unmarshal(forcedScriptWarpData, &forced); err != nil {
		return nil, err
	}
	return append(routes, forced...), nil
}

type scriptWarpRoute struct {
	Source          string   `json:"source"`
	Dungeon         uint32   `json:"dungeon"`
	Maze            byte     `json:"maze"`
	From            uint32   `json:"from_map"`
	To              uint32   `json:"to_map"`
	Position        [2]byte  `json:"position"`
	Target          [2]byte  `json:"target"`
	Record          [18]byte `json:"record"`
	DungeonSHA256   string   `json:"dungeon_sha256"`
	MapSHA256       string   `json:"map_sha256"`
	RequiredKeyMaps []uint32 `json:"required_key_maps"`
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
