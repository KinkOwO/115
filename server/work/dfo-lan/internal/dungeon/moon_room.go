package dungeon

import (
	"dfolan/internal/game/protocol"
	"fmt"
	"sort"
	"time"
)

// Dynamic actors cannot be encoded as fixed PVF indices in N29: index0 would
// recreate an unrelated map monster. Register/restore them after real C37 and
// the shared resource release, using N2194 for every member.
func (r *MoonSoloOwner) MoonEnterRoom(stamp MoonSoloStamp, member uint16, now time.Time) ([]protocol.UnassignedMonster115, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if e := r.validate(stamp, member); e != nil {
		return nil, e
	}
	s := r.session
	if r.phase != moonSoloRunning || !s.Loaded {
		return nil, fmt.Errorf("Moon dynamic entry before room release")
	}
	if r.reuseRoom {
		return nil, nil
	}
	grid := [2]byte{s.Room.X, s.Room.Y}
	var rows []protocol.UnassignedMonster115
	for _, m := range s.Monsters {
		if v, ok := s.MoonDynamic[m.Entity]; ok && !s.Dead[m.Entity] && v.Grid == grid {
			rows = append(rows, v)
		}
	}
	if gate, ok := s.MoonNamed[s.Room.Map]; s.Definition.ID == 100004136 && ok && gate.Slot == 0 && gate.Planned {
		if s.NextEntity == 0 || s.NextEntity >= 65534 || s.Definition.BasisLevel == 0 || s.Definition.BasisLevel > 255 {
			return nil, fmt.Errorf("Moon gate allocation invalid")
		}
		row := gate.Spawn
		row.Entity = s.NextEntity
		s.NextEntity++
		gate.Spawn, gate.Planned = row, false
		s.MoonNamed[s.Room.Map] = gate
		if s.MoonDynamic == nil {
			s.MoonDynamic = map[uint16]protocol.UnassignedMonster115{}
		}
		s.MoonDynamic[row.Entity] = row
		s.Monsters = append(s.Monsters, protocol.DungeonMonster{Entity: row.Entity, Template: row.Template, Level: byte(s.Definition.BasisLevel), Team: 100})
		s.Visited[s.Room.Map] = append([]protocol.DungeonMonster(nil), s.Monsters...)
		rows = append(rows, row)
	}
	if s.Definition.ID == 100004137 && s.MoonGridReady && !s.MoonZermioDefeated &&
		[2]int32{int32(grid[0]), int32(grid[1])} == s.MoonZermioGrid {
		present := false
		for _, row := range rows {
			present = present || row.Template == 109017557
		}
		if !present {
			if s.NextEntity == 0 || s.NextEntity >= 65534 || s.Definition.BasisLevel == 0 || s.Definition.BasisLevel > 255 {
				return nil, fmt.Errorf("Moon Zermio allocation invalid")
			}
			// G0260 N2194 uses-1/-1;300/300 in cos is not blindly a spawn point.
			row := protocol.UnassignedMonster115{Grid: grid, Entity: s.NextEntity, Template: 109017557, X: -1, Y: -1}
			s.NextEntity++
			if s.MoonDynamic == nil {
				s.MoonDynamic = map[uint16]protocol.UnassignedMonster115{}
			}
			s.MoonDynamic[row.Entity] = row
			s.MoonZermioSpawnedAt = now
			s.Monsters = append(s.Monsters, protocol.DungeonMonster{Entity: row.Entity, Template: row.Template, Level: byte(s.Definition.BasisLevel), Team: 100})
			s.Visited[s.Room.Map] = append([]protocol.DungeonMonster(nil), s.Monsters...)
			rows = append(rows, row)
		}
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].Entity < rows[j].Entity })
	return rows, nil
}
