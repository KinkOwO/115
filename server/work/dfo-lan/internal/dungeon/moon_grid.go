package dungeon

import (
	"dfolan/internal/catalog"
	"dfolan/internal/game/protocol"
	"fmt"
)

// INFO82 is counts[x][y], not five opaque three-byte records.
// Native1415F0F50..1415F0F83 indexes 3*x+y. Source137 fixed combat
// populations exactly match G0260 initial counts (including the final boss).
func MoonInitialProgress(c catalog.DungeonCatalog) (MoonProgress, error) {
	p := MoonProgress{GridReady: true, ZermioGrid: [2]int32{3, 0}}
	d, ok := c.Dungeons[100004137]
	if !ok {
		return p, fmt.Errorf("Moon second-floor definition missing")
	}
	var maze *catalog.DungeonMaze
	for i := range d.Mazes {
		if d.Mazes[i].Quest == 0 {
			maze = &d.Mazes[i]
			break
		}
	}
	if maze == nil {
		return p, fmt.Errorf("Moon second-floor maze missing")
	}
	seen := map[[2]byte]bool{}
	for _, room := range maze.Rooms {
		key := [2]byte{room.X, room.Y}
		if room.X >= 5 || room.Y >= 3 || seen[key] {
			return p, fmt.Errorf("invalid Moon grid source")
		}
		seen[key] = true
		_, script, ok, err := resolveRoomMap(c, room)
		if err != nil {
			return p, err
		}
		if !ok {
			return p, fmt.Errorf("Moon grid map missing: %d", room.Map)
		}
		monsters, err := fixedMonsters(script, d.BasisLevel)
		if err != nil {
			return p, err
		}
		for _, m := range monsters {
			if !m.NonCombat && !m.APC {
				if p.Grid[room.X][room.Y] == 255 {
					return p, fmt.Errorf("Moon grid count overflow")
				}
				p.Grid[room.X][room.Y]++
			}
		}
	}
	if !seen[[2]byte{3, 0}] {
		return p, fmt.Errorf("Moon source Zermio start grid missing")
	}
	first, ok := c.Dungeons[100004136]
	if !ok || len(first.Mazes) == 0 {
		return p, fmt.Errorf("Moon first-floor source missing")
	}
	firstCounts, err := moonFirstCounts(c, first, first.Mazes[0])
	if err != nil {
		return p, err
	}
	p.FirstGrid = firstCounts
	return p, nil
}

func moonFirstCounts(c catalog.DungeonCatalog, d catalog.DungeonDefinition, maze catalog.DungeonMaze) ([5]byte, error) {
	var counts [5]byte
	for _, room := range maze.Rooms {
		// Wire first-floor population excludes the source's unused0/5 room.
		if room.X != 0 || room.Y >= 5 {
			continue
		}
		_, script, ok, err := resolveRoomMap(c, room)
		if err != nil {
			return counts, err
		}
		if !ok {
			return counts, fmt.Errorf("Moon first-floor map missing")
		}
		ms, e := fixedMonsters(script, d.BasisLevel)
		if e != nil {
			return counts, e
		}
		n := 0
		for _, m := range ms {
			if !m.NonCombat && !m.APC {
				n++
			}
		}
		if n > 255 {
			return counts, fmt.Errorf("Moon first-floor population overflow")
		}
		counts[room.Y] = byte(n)
	}
	return counts, nil
}

// The one shared run selects one PVF-listed theme order. No per-member RNG,
// maze invention, or reuse of a different route's source indices.
func SelectMoon(c catalog.DungeonCatalog, request protocol.DungeonSelection, level byte, accepted map[uint16]bool, seed uint32) (*Session, error) {
	base, e := Select(c, request, level, accepted)
	if e != nil {
		return nil, e
	}
	d := base.Definition
	if d.ID != 100004136 {
		return nil, fmt.Errorf("Moon preparation requires first floor")
	}
	var candidates []catalog.DungeonMaze
	for _, m := range d.Mazes {
		if m.Quest == 0 && len(m.Pending) == 0 {
			candidates = append(candidates, m)
		}
	}
	if len(candidates) == 0 {
		return nil, fmt.Errorf("Moon source theme order unavailable")
	}
	return newSession(c, d, candidates[seed%uint32(len(candidates))])
}

func (s *Session) InitializeMoon(c catalog.DungeonCatalog) error {
	if s == nil || s.Definition.ID != 100004136 || s.Loaded || s.MoonGridReady {
		return fmt.Errorf("Moon initialization requires fresh first-floor session")
	}
	p, err := MoonInitialProgress(c)
	if err != nil {
		return err
	}
	firstCounts, err := moonFirstCounts(c, s.Definition, s.Maze)
	if err != nil {
		return err
	}
	s.MoonGridReady, s.MoonGrid, s.MoonZermioGrid = p.GridReady, p.Grid, p.ZermioGrid
	s.MoonFirstGrid = firstCounts
	return nil
}
