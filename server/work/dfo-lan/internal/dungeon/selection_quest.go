package dungeon

import (
	"dfolan/internal/catalog"
	"fmt"
)

// selectionMazeQuest resolves a quest-only dungeon when CMD16 omits Quest.
// The live Roar Ravine request has Quest=0 even while quest 12430 is accepted;
// its source DGN declares only the maze connected to that quest. Preserve
// explicit requests and ordinary dungeon routes, and require one unambiguous
// accepted quest rather than granting access to an unowned story route.
// The allowed quest set is supplied by the caller; the gateway's existing
// acceptedQuestIDs also permits completed quests with the same config version.
func selectionMazeQuest(d catalog.DungeonDefinition, requested uint32, accepted map[uint16]bool) (uint32, error) {
	if requested != 0 {
		return requested, nil
	}
	for _, m := range d.Mazes {
		if m.Quest == 0 {
			return 0, nil
		}
	}
	var quest uint16
	for _, m := range d.Mazes {
		if len(m.Pending) != 0 || !accepted[m.Quest] {
			continue
		}
		if quest != 0 && quest != m.Quest {
			return 0, fmt.Errorf("quest-only dungeon has multiple accepted source quests")
		}
		quest = m.Quest
	}
	return uint32(quest), nil
}
