package dungeon

import (
	"dfolan/internal/game/protocol"
	"fmt"
)

type MoonNamedState struct {
	Planned     bool // gate reserved by progress, actor is allocated only after room load
	Spawn       protocol.UnassignedMonster115
	Slot        byte
	Dead        bool
	ResultIndex byte // capture: successive defeated named records carry0,1,2, not a bool
}

// Value snapshot at one combat event, not the newest room state at delivery.
// Multiple deaths can queue before a slow member receives the first one.
type MoonProgress struct {
	FirstGrid      [5]byte
	GridReady      bool
	Grid           [5][3]byte
	ZermioGrid     [2]int32
	ZermioDefeated bool
	ZermioHealth   uint64
	ZermioMeter    uint32
	Cleared        [5]bool
	Named          [4]MoonNamedState
	Present        [4]bool
	Troop, Fever   uint32
	Floor          uint32
}

func (s *Session) MoonProgress() MoonProgress {
	p := MoonProgress{Troop: s.MoonTroopScore, Fever: s.MoonFeverScore, Cleared: s.MoonCleared}
	p.FirstGrid = s.MoonFirstGrid
	p.GridReady, p.Grid, p.ZermioGrid, p.ZermioDefeated = s.MoonGridReady, s.MoonGrid, s.MoonZermioGrid, s.MoonZermioDefeated
	p.ZermioHealth, p.ZermioMeter = s.MoonZermioHealth, s.MoonZermioMeter
	if s.Definition.ID == 100004137 || s.Definition.ID == 100004136 && s.Completed() {
		p.Floor = 1
	}
	if s.Definition.ID == 100004137 && s.MoonGridReady {
		remaining := false
		for x := range s.MoonGrid {
			for y, count := range s.MoonGrid[x] {
				if (x != 4 || y != 1) && count != 0 {
					remaining = true
				}
			}
		}
		if !remaining {
			p.Floor = 2
		} // G0260: before entering final boss, not after killing it.
	}
	for _, n := range s.MoonNamed {
		if n.Slot < 4 {
			p.Named[n.Slot], p.Present[n.Slot] = n, true
		}
	}
	return p
}

func (r *MoonSoloOwner) MoonProgress(stamp MoonSoloStamp, member uint16) (MoonProgress, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if e := r.validate(stamp, member); e != nil {
		return MoonProgress{}, e
	}
	return r.session.MoonProgress(), nil
}

// Source base per-kill increments. Eight regular deaths in G0260 produce
// INFO118/122=520/1000: two scores, not score/max. Special bonuses/consumption
// are separate transitions, not fabricated here.
func (r *MoonSoloOwner) MoonScoreDeath(stamp MoonSoloStamp, member uint16, entity uint16) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if e := r.validate(stamp, member); e != nil {
		return e
	}
	s := r.session
	if r.phase != moonSoloRunning || (s.Definition.ID != 100004136 && s.Definition.ID != 100004137) || !s.Dead[entity] {
		return fmt.Errorf("Moon score requires current confirmed death")
	}
	if s.MoonScored[entity] {
		return nil
	}
	if s.MoonScored == nil {
		s.MoonScored = map[uint16]bool{}
	}
	s.MoonScored[entity] = true
	if actor, ok := s.MoonDynamic[entity]; ok && actor.Template == 109017557 {
		if !s.Unowned[entity] {
			s.MoonZermioDefeated = true
			s.MoonTroopScore = min(uint32(1000), s.MoonTroopScore+500)
		}
		return nil // Zermio is not an ordinary map population entry.
	}
	for _, n := range s.MoonNamed {
		if n.Spawn.Entity == entity {
			return nil
		}
	}
	for _, m := range s.Monsters {
		if m.Entity == entity && !m.NonCombat && !m.APC {
			if s.Definition.ID == 100004136 && s.MoonGridReady && s.Room.X == 0 && s.Room.Y < 5 && s.MoonFirstGrid[s.Room.Y] > 0 {
				s.MoonFirstGrid[s.Room.Y]--
			}
			if s.Definition.ID == 100004137 && s.MoonGridReady && s.Room.X < 5 && s.Room.Y < 3 && s.MoonGrid[s.Room.X][s.Room.Y] > 0 {
				s.MoonGrid[s.Room.X][s.Room.Y]--
			}
			if s.Unowned[entity] {
				return nil
			}
			s.MoonTroopScore = min(uint32(1000), s.MoonTroopScore+65)
			s.MoonFeverScore = min(uint32(1000), s.MoonFeverScore+125)
			return nil
		}
	}
	return nil
}

// Source moonlake.cos forest/lake/camp/gatekeeper, bound to the matching
// source maze maps. No client-supplied template, position or entity number.
func moonNamedBinding(mapID uint32) (slot byte, template uint32, ok bool) {
	switch mapID {
	case 100012770:
		return 3, 109017561, true
	case 100012775:
		return 2, 109017515, true
	case 100012780:
		return 1, 109017487, true
	case 100012766:
		return 0, 109017560, true
	}
	return 0, 0, false
}

// Called after a confirmed death while the same shared room is locked by the
// coordinator. An inserted actor becomes part of RoomCleared/death validation.
func (r *MoonSoloOwner) MoonAfterDeath(stamp MoonSoloStamp, member uint16) (*MoonNamedState, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if e := r.validate(stamp, member); e != nil {
		return nil, e
	}
	s := r.session
	if r.phase == moonSoloRunning && s.Loaded && s.Definition.ID == 100004137 && s.MoonGridReady && s.Room.Boss && [2]byte{s.Room.X, s.Room.Y} == s.Maze.Boss {
		// Moon's source final boss reports C39, not the ordinary C117 check.
		// Only a real, owned, registered final-boss death can start settlement.
		for _, m := range s.Monsters {
			if m.Template == 109017558 && m.Rank == 3 && s.Dead[m.Entity] && !s.Unowned[m.Entity] {
				s.completionTarget = m.Entity
				s.tryComplete()
				break
			}
		}
		return nil, nil
	}
	if r.phase != moonSoloRunning || s.Definition.ID != 100004136 || !s.RoomCleared() {
		return nil, nil
	}
	if s.Room.X == 0 && s.Room.Y < 5 {
		s.MoonCleared[s.Room.Y] = true
	}
	if old, ok := s.MoonNamed[s.Room.Map]; ok {
		if s.Dead[old.Spawn.Entity] {
			if !old.Dead {
				for _, previous := range s.MoonNamed {
					if previous.Dead {
						old.ResultIndex++
					}
				}
			}
			old.Dead = true
			s.MoonNamed[s.Room.Map] = old
			if old.Slot == 0 {
				s.completed = true
				s.completionTarget = old.Spawn.Entity
			} else {
				s.armMoonGate()
			}
		}
		return nil, nil
	}
	slot, template, ok := moonNamedBinding(s.Room.Map)
	if !ok {
		return nil, nil
	}
	if slot == 0 {
		return nil, nil
	} // a gate encounter is prepared before entering it
	// Native14132B280 disallows another thematic encounter once two named
	// coordinates have been allocated; G0260 visits camp/lake, skips forest,
	// then spawns the gatekeeper. Gatekeeper is not a third thematic encounter.
	if slot != 0 {
		allocated := 0
		for _, n := range s.MoonNamed {
			if n.Slot != 0 {
				allocated++
			}
		}
		if allocated >= 2 {
			s.armMoonGate()
			return nil, nil
		}
	}
	if s.NextEntity == 0 || s.NextEntity >= 65534 || s.Definition.BasisLevel == 0 || s.Definition.BasisLevel > 255 {
		return nil, fmt.Errorf("Moon named actor allocation unavailable")
	}
	entry := MoonNamedState{Spawn: protocol.UnassignedMonster115{Grid: [2]byte{s.Room.X, s.Room.Y}, Entity: s.NextEntity, Template: template, X: -100, Y: -100}, Slot: slot}
	s.NextEntity++
	// Neutral drop rank: do not invent a boss-drop multiplier from its name.
	s.Monsters = append(s.Monsters, protocol.DungeonMonster{Entity: entry.Spawn.Entity, Template: template, Level: byte(s.Definition.BasisLevel), Team: 100})
	s.Visited[s.Room.Map] = append([]protocol.DungeonMonster(nil), s.Monsters...)
	if s.MoonNamed == nil {
		s.MoonNamed = map[uint32]MoonNamedState{}
	}
	s.MoonNamed[s.Room.Map] = entry
	if s.MoonDynamic == nil {
		s.MoonDynamic = map[uint16]protocol.UnassignedMonster115{}
	}
	s.MoonDynamic[entry.Spawn.Entity] = entry.Spawn
	return &entry, nil
}

// moonlake.cos's gate row carries750, unlike the zero thematic thresholds.
// G0260 arms0/4 once the preceding encounter is cleared with troop1000:
// INFO already has its named coordinate and zero population before N29.
func (s *Session) armMoonGate() {
	if s.MoonTroopScore < 750 || s.Room.Y >= 4 {
		return
	}
	for _, room := range s.Maze.Rooms {
		if room.X != 0 || room.Y != 4 {
			continue
		}
		if _, exists := s.MoonNamed[room.Map]; exists {
			return
		}
		if s.MoonNamed == nil {
			s.MoonNamed = map[uint32]MoonNamedState{}
		}
		s.MoonNamed[room.Map] = MoonNamedState{Planned: true, Slot: 0, Spawn: protocol.UnassignedMonster115{Grid: [2]byte{0, 4}, Template: 109017560, X: -100, Y: -100}}
		s.MoonFirstGrid[4] = 0
		return
	}
}
