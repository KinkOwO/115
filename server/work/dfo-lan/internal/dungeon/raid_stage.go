package dungeon

import (
	"dfolan/internal/catalog"
	"fmt"
)

// The caller binds the target to the source battlefield's normal entry grid.
// This may be nonadjacent (e.g. Sparazzi), but never leaves a live boss.
func (s *Session) MoveRaidReturn(c catalog.DungeonCatalog, target [2]byte) (*Session, error) {
	if s == nil || !s.Definition.Bakal || !s.RoomCleared() {
		return nil, fmt.Errorf("raid return requires cleared owned room")
	}
	for _, room := range s.Maze.Rooms {
		if [2]byte{room.X, room.Y} == target {
			return s.enterRoom(c, s.latestLayer(room))
		}
	}
	return nil, fmt.Errorf("raid return destination absent from source maze")
}

// A source raid phase teleport may leave its first actor alive in a custom
// die action. It is not an ordinary adjacent-room door or a completed room.
func (s *Session) MoveRaidStage(c catalog.DungeonCatalog, m catalog.BakalRaidMonster) (*Session, error) {
	if s == nil || !s.Loaded || m.SecondID == 0 || !m.HasGrid || !m.HasSecondGrid || [2]byte{s.Room.X, s.Room.Y} != m.Grid {
		return nil, fmt.Errorf("raid phase warp outside owned source actor")
	}
	owned := false
	for _, actor := range s.Monsters {
		owned = owned || actor.Template == m.ID
	}
	if !owned {
		return nil, fmt.Errorf("raid phase first actor was never spawned")
	}
	for _, room := range s.Maze.Rooms {
		if [2]byte{room.X, room.Y} == m.SecondGrid {
			return s.enterRoom(c, s.latestLayer(room))
		}
	}
	return nil, fmt.Errorf("raid phase destination absent from native maze")
}
