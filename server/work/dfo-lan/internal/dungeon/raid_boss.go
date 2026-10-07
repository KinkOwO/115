package dungeon

import (
	"dfolan/internal/catalog"
	"dfolan/internal/game/protocol"
	"fmt"
)

// AddRaidBoss registers the source raid placement before sending N2194.
// Handover raid_boss.go:22/35/39: no duplicate template, update visited and
// entity allocation, then let the ordinary confirmed-death path own it.
func (s *Session) AddRaidBoss(template uint32, x, y int32) (protocol.UnassignedMonster115, bool, error) {
	var row protocol.UnassignedMonster115
	if s == nil || !s.Loaded || template == 0 || s.NextEntity == 65535 || s.Definition.BasisLevel > 255 {
		return row, false, fmt.Errorf("invalid owned raid boss placement")
	}
	for _, m := range s.Monsters {
		if m.Template == template && !s.Dead[m.Entity] {
			return row, false, nil
		}
	}
	row = protocol.UnassignedMonster115{Grid: [2]byte{s.Room.X, s.Room.Y}, Entity: s.NextEntity, Template: template, Rank: 3, X: x, Y: y}
	if _, err := protocol.UnassignedMonsterAdd115([]protocol.UnassignedMonster115{row}); err != nil {
		return row, false, err
	}
	s.NextEntity++
	s.Monsters = append(s.Monsters, protocol.DungeonMonster{Entity: row.Entity, Template: template, Level: byte(s.Definition.BasisLevel), Rank: 3, Team: 100, SourceIndex: 10000})
	if s.Visited == nil {
		s.Visited = map[uint32][]protocol.DungeonMonster{}
	}
	s.Visited[s.Room.Map] = append([]protocol.DungeonMonster(nil), s.Monsters...)
	s.ArenaBoss = true
	s.completed = false
	return row, true, nil
}

// The caller must authorize the stage trigger from native raid rules.
func (s *Session) RaidStageWarp(c catalog.DungeonCatalog, target [2]byte) (*Session, error) {
	if s == nil || !s.RaidManaged || !s.Loaded {
		return nil, fmt.Errorf("raid stage warp outside loaded owned combat")
	}
	for _, room := range s.Maze.Rooms {
		if [2]byte{room.X, room.Y} == target {
			next, err := s.enterRoom(c, room)
			if err != nil {
				return nil, err
			}
			next.Loaded = false
			next.completed = false
			next.completionTarget = 0
			return next, nil
		}
	}
	return nil, fmt.Errorf("raid stage warp target absent from native maze")
}

// Handover raid_stage.go:10..19 selects the owned maze target and its latest
// visited layer after a cleared room, instead of applying adjacent-door Move.
func (s *Session) MoveRaidReturn(c catalog.DungeonCatalog, target [2]byte) (*Session, error) {
	if s == nil || !s.RaidManaged || !s.Loaded || !s.RoomCleared() {
		return nil, fmt.Errorf("raid return requires cleared owned loaded room")
	}
	for _, room := range s.Maze.Rooms {
		if [2]byte{room.X, room.Y} == target {
			return s.enterRoom(c, s.latestLayer(room))
		}
	}
	return nil, fmt.Errorf("raid return target absent from source maze")
}
