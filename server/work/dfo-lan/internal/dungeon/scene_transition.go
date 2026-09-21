package dungeon

import (
	"dfolan/internal/catalog"
	"dfolan/internal/game/protocol"
	"fmt"
)

func (s *Session) MoveScene(c catalog.DungeonCatalog, r protocol.DungeonRoomTransition) (*Session, error) {
	if s == nil || !s.Loaded || !r.LayerChange || s.Completed() {
		return nil, fmt.Errorf("scene transition requires owned cleared loaded room")
	}
	if r.Dungeon != s.Definition.ID || r.Position != [2]byte{s.Room.X, s.Room.Y} {
		return nil, fmt.Errorf("scene transition dungeon/position mismatch")
	}
	if s.Definition.Odyssey && len(c.SceneRoutes) > 0 {
		if !s.RoomCleared() {
			return nil, fmt.Errorf("scene transition requires owned cleared loaded room")
		}
		if !s.scriptStageReady() {
			return nil, fmt.Errorf("scene requires preceding source script warp")
		}
		for _, route := range c.SceneRoutes {
			if route.Dungeon != s.Definition.ID || route.Maze != s.Maze.Index || route.From != s.Room.Map || route.Position != r.Position {
				continue
			}
			if route.Source != c.Source.Checksum || route.DungeonSHA256 != s.Definition.Script.SHA256 || route.MapSHA256 != c.Maps[s.Room.Map].SHA256 || !catalog.SceneRouteInMaze(s.Definition, route) {
				return nil, fmt.Errorf("scene transition source mismatch")
			}
			if r.Record != route.Record {
				continue
			}
			_, visited := s.Visited[route.To]
			if visited {
				return nil, fmt.Errorf("scene layer missing or already visited")
			}
			room := s.Room
			room.Map = route.To
			return s.enterRoom(c, room)
		}
		return nil, fmt.Errorf("no next source scene")
	}

	for _, layer := range s.Maze.Layers {
		if layer.Position != r.Position {
			continue
		}
		currentIdx := -1
		for i, mapID := range layer.Maps {
			if mapID == s.Room.Map {
				currentIdx = i
				break
			}
		}
		nextIdx := currentIdx + 1
		if nextIdx >= len(layer.Maps) {
			return nil, fmt.Errorf("no next layer map")
		}
		nextMap := layer.Maps[nextIdx]
		if _, ok := c.Maps[nextMap]; !ok {
			return nil, fmt.Errorf("layer target map not imported %d", nextMap)
		}
		room := s.Room
		room.Map = nextMap
		return s.enterRoom(c, room)
	}

	return nil, fmt.Errorf("no layer configured for room position")
}
