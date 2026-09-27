package dungeon

import (
	"dfolan/internal/catalog"
	"dfolan/internal/game/protocol"
	"encoding/binary"
	"fmt"
)

// The closing q3215_14949.cmt [CHANGE MAP] changes the player's position
// within the final cached layer. Its native CMD45 record carries the scene's
// X/Y bounds; NOTI29 mode 0 with layer flag 1 retains the last layer in the
// client (1452b77ee..1452b788b). This is not a request for another map.
func (s *Session) lotusClosingRevisit(r protocol.DungeonRoomTransition) bool {
	if s.Definition.ID != 26 || s.Maze.Index != 3 || s.Room.Map != 100008697 || !s.RoomCleared() {
		return false
	}
	record := r.Record
	if record[0] != 0 || record[1] != 0 || record[2] != 0 || record[3] != 0 || record[4] != 4 || record[5] != 5 ||
		record[10] != 0 || record[11] != 0 || record[12] != 3 || record[13] != 0 || record[14] != 2 || record[15] != 0 || record[16] != 0 || record[17] != 0 {
		return false
	}
	x := binary.LittleEndian.Uint16(record[6:8])
	y := binary.LittleEndian.Uint16(record[8:10])
	return x >= 701 && x <= 704 && y >= 229 && y <= 231
}

// A terminal cinematic without a boss identity may revisit its cached layer
// after clearing the quest's source objective room. Match the exact source
// CMT landing area imported for this maze; cinematic actors in the last map
// are not a room-clear precondition.
func (s *Session) terminalSceneRevisit(c catalog.DungeonCatalog, r protocol.DungeonRoomTransition) bool {
	if s.completionTarget != 0 || s.hasFightableBoss() || s.reportableDisplayBoss() != 0 ||
		r.Record[0] != 0 || r.Record[1] != 0 || r.Record[2] != 0 ||
		r.Record[3] != 0 || r.Record[4] != 4 || r.Record[5] != 5 {
		return false
	}
	x := binary.LittleEndian.Uint16(r.Record[6:8])
	y := binary.LittleEndian.Uint16(r.Record[8:10])
	for _, scene := range c.TerminalScenes {
		if scene.Source != c.Source.Checksum || scene.Dungeon != s.Definition.ID || scene.Maze != s.Maze.Index ||
			scene.Quest != s.Maze.Quest || scene.Position != r.Position || scene.FinalMap != s.Room.Map ||
			scene.DungeonSHA256 != s.Definition.Script.SHA256 || scene.MapSHA256 != c.Maps[s.Room.Map].SHA256 ||
			x < scene.XMin || x > scene.XMax || y < scene.YMin || y > scene.YMax {
			continue
		}
		objective, seen := s.Visited[scene.ObjectiveMap]
		if !seen {
			return false
		}
		for _, m := range objective {
			if m.Team != 0 && !m.NonCombat && !s.Dead[m.Entity] {
				return false
			}
		}
		return true
	}
	return false
}

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
			if currentIdx == len(layer.Maps)-1 && s.lotusClosingRevisit(r) {
				next, err := s.enterRoom(c, s.Room)
				if err != nil {
					return nil, err
				}
				next.lotusClosingReached = true
				return next, nil
			}
			if currentIdx == len(layer.Maps)-1 && s.terminalSceneRevisit(c, r) {
				next, err := s.enterRoom(c, s.Room)
				if err != nil {
					return nil, err
				}
				next.terminalSceneClosingReached = true
				return next, nil
			}
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
