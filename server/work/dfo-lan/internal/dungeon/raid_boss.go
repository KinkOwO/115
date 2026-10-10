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
	return s.addSourceRaidBoss(m.ID, m.X, m.Y, grid)
}

// AddAsrahanBoss resolves the template and coordinates from the same native
// phase/COS and registers it only in the owned, actually loaded boss room.
// This is the room executor; the raid gateway must still authorize the source
// action and deliver the returned N2194 before accepting client death reports.
func (s *Session) AddAsrahanBoss(r *catalog.AsrahanRaidRules, hard bool, group uint32) (protocol.UnassignedMonster115, bool, error) {
	if s == nil || !s.Loaded || s.RunID == "" || r == nil {
		return protocol.UnassignedMonster115{}, false, fmt.Errorf("Asrahan spawn before owned room loading")
	}
	p := r.Normal
	if hard {
		p = r.Hard
	}
	g, err := r.CombatGroup(p, group)
	if err != nil {
		return protocol.UnassignedMonster115{}, false, err
	}
	if s.Definition.ID != g.Dungeon || s.Definition.BasisLevel == 0 || s.Definition.BasisLevel > 255 {
		return protocol.UnassignedMonster115{}, false, fmt.Errorf("Asrahan group does not own current source dungeon")
	}
	if [2]byte{s.Room.X, s.Room.Y} != s.Maze.Boss {
		return protocol.UnassignedMonster115{}, false, fmt.Errorf("Asrahan actor outside source boss room")
	}
	// Lopes is already declared by the native map, while its COS record is
	// an index-zero placeholder. Loading created the real map roster first.
	// Do not invent a COS spawn or issue a duplicate N2194 for that actor.
	var existing *protocol.DungeonMonster
	for i := range s.Monsters {
		actor := &s.Monsters[i]
		if actor.Template != g.Monster {
			continue
		}
		if existing != nil || actor.Entity == 0 || actor.Rank != 3 || actor.Team == 0 || actor.NonCombat || s.Dead[actor.Entity] {
			return protocol.UnassignedMonster115{}, false, fmt.Errorf("Asrahan source boss roster is ambiguous or inactive")
		}
		existing = actor
	}
	if existing != nil {
		s.completed = false
		s.ArenaBoss = true
		return protocol.UnassignedMonster115{}, false, nil
	}
	_, m, err := r.MonsterForGroup(p, group)
	if err != nil {
		return protocol.UnassignedMonster115{}, false, err
	}
	return s.addSourceRaidBoss(m.Template, m.X, m.Y, s.Maze.Boss)
}

func (s *Session) addSourceRaidBoss(template uint32, x, y int32, grid [2]byte) (protocol.UnassignedMonster115, bool, error) {
	if [2]byte{s.Room.X, s.Room.Y} != grid {
		return protocol.UnassignedMonster115{}, false, fmt.Errorf("raid actor outside source appearance grid")
	}
	for _, actor := range s.Monsters {
		if actor.Template == template {
			return protocol.UnassignedMonster115{}, false, nil
		}
	}
	if s.NextEntity == 0 || s.NextEntity >= 65535 {
		return protocol.UnassignedMonster115{}, false, fmt.Errorf("raid monster actor allocation exhausted")
	}
	row := protocol.UnassignedMonster115{Grid: [2]byte{s.Room.X, s.Room.Y}, Entity: s.NextEntity, Template: template, X: x, Y: y, Rank: 3}
	if _, err := protocol.UnassignedMonsterAdd115([]protocol.UnassignedMonster115{row}); err != nil {
		return protocol.UnassignedMonster115{}, false, err
	}
	s.NextEntity = advanceMonsterEntity(s.NextEntity, s.monsterEntityStride())
	s.Monsters = append(s.Monsters, protocol.DungeonMonster{Entity: row.Entity, Template: template, SourceIndex: 10000, Level: byte(s.Definition.BasisLevel), Rank: 3, Team: 100})
	if s.Visited == nil {
		s.Visited = map[uint32][]protocol.DungeonMonster{}
	}
	s.Visited[s.Room.Map] = append([]protocol.DungeonMonster(nil), s.Monsters...)
	s.completed = false
	s.ArenaBoss = true
	return row, true, nil
}
