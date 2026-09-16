package dungeon

import (
	"dfolan/internal/catalog"
	"fmt"
)

// SelectTutorial opens a job's starting dungeon. The ordinary Select refuses
// a [tutorial dungeon] outright, so before 36 a character could never enter
// one: the client's own tutorial CMD15 sender (146cce650) supplies a source
// dungeon ID rather than the town gate's zero, and every such request was
// rejected.
//
// A tutorial route names its dungeon directly, so there is no quest
// connection to resolve and no difficulty, party or event option to honour.
// The minimum level is not applied: these dungeons exist for a level 1
// character and their own source rows may still carry a higher basis.
func SelectTutorial(c catalog.DungeonCatalog, id uint32) (*Session, error) {
	d, ok := c.Dungeons[id]
	if !ok {
		return nil, fmt.Errorf("tutorial dungeon absent from imported source")
	}
	if !d.Tutorial {
		return nil, fmt.Errorf("dungeon %d is not a source tutorial dungeon", id)
	}
	var chosen *catalog.DungeonMaze
	for _, m := range d.Mazes {
		if m.Quest != 0 {
			continue
		}
		if chosen != nil {
			return nil, fmt.Errorf("ambiguous tutorial maze")
		}
		v := m
		chosen = &v
	}
	if chosen == nil || len(chosen.Pending) > 0 {
		return nil, fmt.Errorf("no resolved tutorial maze")
	}
	return newSession(c, d, *chosen)
}
