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

// AddRaidBoss registers a source-resolved boss after native loading completes.
// A prior empty-room loading report cannot permanently complete this arena.
func (s *Session) AddBakalRaidBoss(m catalog.BakalRaidMonster) (protocol.UnassignedMonster115, bool, error) {
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
