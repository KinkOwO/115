package dungeon

import (
	"dfolan/internal/catalog"
	"dfolan/internal/game/protocol"
	"fmt"
)

var moonSecondFloorRoute = [][2]byte{{4, 0}, {3, 0}, {2, 0}, {1, 0}, {0, 0}, {0, 1}, {1, 1}, {2, 1}, {3, 1}, {4, 1}}

// The source route explicitly drives surviving normal monsters into the next
// room. G0260 carries1,10,14,19 actors with unchanged IDs and source positions.
// Ordinary dungeons retain their clear-before-move rule.
func (s *Session) moveMoonSecondFloor(c catalog.DungeonCatalog, target [2]byte) (*Session, error) {
	if !s.Loaded || s.Completed() || !s.MoonGridReady || (s.Definition.ID != 100004137 && s.Definition.ID != 100004136) {
		return nil, fmt.Errorf("Moon drive requires current loaded Moon floor")
	}
	current := [2]byte{s.Room.X, s.Room.Y}
	first := s.Definition.ID == 100004136
	forward := first && current[0] == 0 && target[0] == 0 && current[1] < 4 && target[1] == current[1]+1
	if !first {
		for i := 0; i+1 < len(moonSecondFloorRoute); i++ {
			if moonSecondFloorRoute[i] == current && moonSecondFloorRoute[i+1] == target {
				forward = true
				break
			}
		}
	}
	if !forward {
		return s.Move(c, target)
	} // no special bypass for backward/unknown edges
	var carried []protocol.DungeonMonster
	var kept []protocol.DungeonMonster
	for _, m := range s.Monsters {
		if s.Dead[m.Entity] || m.NonCombat || m.APC {
			kept = append(kept, m)
			continue
		}
		named := false
		for _, n := range s.MoonNamed {
			named = named || n.Spawn.Entity == m.Entity
		}
		if m.Rank == 3 || m.Template == 109017557 || named {
			return nil, fmt.Errorf("Moon named encounter must be completed before drive")
		}
		carried = append(carried, m)
	}
	if len(carried) > 30 {
		return nil, fmt.Errorf("Moon drive exceeds source maximum30")
	}
	if !first && len(carried) > 0 && (target == [2]byte{4, 1} || target == [2]byte{byte(s.MoonZermioGrid[0]), byte(s.MoonZermioGrid[1])} && !s.MoonZermioDefeated) {
		return nil, fmt.Errorf("Moon named destination requires clearing carried troops")
	}
	var room *catalog.DungeonRoom
	for _, candidate := range s.Maze.Rooms {
		if [2]byte{candidate.X, candidate.Y} == target {
			v := candidate
			room = &v
			break
		}
	}
	if room == nil {
		return nil, fmt.Errorf("Moon source route room missing")
	}
	if first && target == [2]byte{0, 4} {
		gate, armed := s.MoonNamed[room.Map]
		if !armed || gate.Slot != 0 || len(carried) > 0 {
			return nil, fmt.Errorf("Moon gate requires prepared encounter and no carried troops")
		}
	}
	if len(carried) > 0 {
		if _, visited := s.Visited[room.Map]; visited {
			return nil, fmt.Errorf("Moon drive cannot duplicate a visited population")
		}
	}
	next, err := s.enterRoom(c, *room)
	if err != nil {
		return nil, err
	}
	next.MoonDynamic = make(map[uint16]protocol.UnassignedMonster115, len(s.MoonDynamic)+len(carried))
	for id, row := range s.MoonDynamic {
		next.MoonDynamic[id] = row
	}
	for _, m := range carried {
		row, ok := s.MoonDynamic[m.Entity]
		if !ok {
			x, y, err := moonSourcePlacement(c.Maps[s.Room.Map], m.SourceIndex)
			if err != nil {
				return nil, err
			}
			row = protocol.UnassignedMonster115{Entity: m.Entity, Template: m.Template, X: x, Y: y}
		}
		row.Grid = target
		row.Carried = true
		next.MoonDynamic[m.Entity] = row
	}
	next.Monsters = append(append([]protocol.DungeonMonster(nil), next.Monsters...), carried...)
	next.Visited[s.Room.Map] = kept
	next.Visited[next.Room.Map] = append([]protocol.DungeonMonster(nil), next.Monsters...)
	count := 0
	for _, m := range next.Monsters {
		if !next.Dead[m.Entity] && !m.NonCombat && !m.APC && m.Template != 109017557 {
			count++
		}
	}
	if count > 255 {
		return nil, fmt.Errorf("Moon target population overflow")
	}
	if first {
		next.MoonFirstGrid[s.Room.Y] = 0
		next.MoonFirstGrid[target[1]] = byte(count)
		next.MoonCleared[s.Room.Y] = true
	} else {
		next.MoonGrid[s.Room.X][s.Room.Y] = 0
		next.MoonGrid[target[0]][target[1]] = byte(count)
	}
	return next, nil
}

// G0260 C45/N29 agree on this normal-door record even with record[0]=0.
// Build it from the accepted source edge, never from client landing coordinates.
// Floor entry has no edge and keeps the native all-FF gate sentinel instead.
func moonDoorTransition(from, to [2]byte) ([18]byte, error) {
	var record [18]byte
	dx, dy := int(to[0])-int(from[0]), int(to[1])-int(from[1])
	switch {
	case dx == -1 && dy == 0:
		record[4] = 0
	case dx == 1 && dy == 0:
		record[4] = 1
	case dx == 0 && dy == -1:
		record[4] = 2
	case dx == 0 && dy == 1:
		record[4] = 3
	default:
		return record, fmt.Errorf("Moon normal door is not an adjacent source edge")
	}
	record[5] = 5
	for i := 6; i < 12; i++ {
		record[i] = 255
	}
	return record, nil
}

// Read positions from the exact fixed row already used to create the actor.
// Do not reuse a source index in another map or assign a fresh entity ID.
func moonSourcePlacement(script catalog.ScriptRecord, source uint32) (int32, int32, error) {
	c := script.Cells
	active := false
	index := uint32(0)
	for i := 0; i < len(c); i++ {
		if c[i].Type == 3 {
			active = c[i].Text == "[monster]"
			continue
		}
		if !active {
			continue
		}
		if i+8 > len(c) {
			break
		}
		for j := 0; j < 8; j++ {
			if c[i+j].Type != 0 {
				return 0, 0, fmt.Errorf("Moon source placement malformed")
			}
		}
		if index == source {
			return c[i+3].Value, c[i+4].Value, nil
		}
		index++
		i += 8
		champion := false
		for i < len(c) && c[i].Type == 6 {
			champion = champion || c[i].Text == "[champion]"
			i++
		}
		if champion && i < len(c) && c[i].Type == 0 && c[i].Value == 0 {
			i++
		}
		i--
	}
	return 0, 0, fmt.Errorf("Moon source placement index absent")
}
