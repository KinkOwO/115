package dungeon

import (
	"dfolan/internal/catalog"
	"dfolan/internal/game/protocol"
	"fmt"
)

// AddRaidBoss registers a source-resolved boss after native loading completes.
// A prior empty-room loading report cannot permanently complete this arena.
func (s *Session) AddRaidBoss(m catalog.BakalRaidMonster) (protocol.UnassignedMonster115, bool, error) {
	if s == nil || m.ID == 0 || s.Definition.BasisLevel > 255 {
		return protocol.UnassignedMonster115{}, false, fmt.Errorf("raid boss does not belong to current arena")
	}
	grid := s.Maze.Boss
	if m.HasGrid {
		grid = m.Grid
	}
	if [2]byte{s.Room.X, s.Room.Y} != grid {
		return protocol.UnassignedMonster115{}, false, fmt.Errorf("raid actor outside source appearance grid")
	}
	for _, actor := range s.Monsters {
		if actor.Template == m.ID {
			return protocol.UnassignedMonster115{}, false, nil
		}
	}
	if s.NextEntity == 0 || s.NextEntity >= 65535 {
		return protocol.UnassignedMonster115{}, false, fmt.Errorf("raid monster actor allocation exhausted")
	}
	row := protocol.UnassignedMonster115{Grid: [2]byte{s.Room.X, s.Room.Y}, Entity: s.NextEntity, Template: m.ID, X: m.X, Y: m.Y, Rank: 3}
	if _, err := protocol.UnassignedMonsterAdd115([]protocol.UnassignedMonster115{row}); err != nil {
		return protocol.UnassignedMonster115{}, false, err
	}
	s.NextEntity++
	s.Monsters = append(s.Monsters, protocol.DungeonMonster{Entity: row.Entity, Template: m.ID, SourceIndex: 10000, Level: byte(s.Definition.BasisLevel), Rank: 3, Team: 100})
	if s.Visited == nil {
		s.Visited = map[uint32][]protocol.DungeonMonster{}
	}
	s.Visited[s.Room.Map] = append([]protocol.DungeonMonster(nil), s.Monsters...)
	s.completed = false
	s.ArenaBoss = true
	return row, true, nil
}
