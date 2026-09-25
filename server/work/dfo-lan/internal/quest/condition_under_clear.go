package quest

import (
	"dfolan/internal/catalog"
	"dfolan/internal/dungeon"
)

// UnderClearObjective is the three-cell subtype-11 form found in the current
// story catalog: dungeon, minimum difficulty, and a positive room hint. The
// final cell is retained as a source consistency check; it is not treated as
// a number of dungeon clears.
type UnderClearObjective struct {
	Dungeon       uint32
	MinDifficulty int32
	RoomHint      uint32
}

func ConditionUnderClearObjective(d catalog.QuestDefinition) (UnderClearObjective, bool) {
	c := d.ObjectiveCells
	if d.Kind != "[condition under clear]" || len(d.Pending) != 0 || reachSubtype(d) != 11 || len(c) != 3 {
		return UnderClearObjective{}, false
	}
	for _, value := range c {
		if value.Type != 0 {
			return UnderClearObjective{}, false
		}
	}
	if c[0].Value <= 0 || c[1].Value < -1 || c[1].Value > 5 || c[2].Value <= 0 {
		return UnderClearObjective{}, false
	}
	info := cells(d.Script.Cells, "[dungeon info]")
	if len(info) != 2 || info[0].Type != 0 || info[0].Value != c[0].Value || info[1].Type != 0 || info[1].Value != c[1].Value {
		return UnderClearObjective{}, false
	}
	return UnderClearObjective{uint32(c[0].Value), c[1].Value, uint32(c[2].Value)}, true
}

// The final boss only proves the route ended. A subtype-11 objective also
// needs every configured room in its own quest maze to have been visited.
// Dungeon movement requires clearing the previous room; the caller checks
// run.Completed(), which proves the final room was cleared too.
func allRoomsUnderClearMatch(en *Entry, run *dungeon.Session) bool {
	if en == nil || !en.Implemented || en.Model != AllRoomsUnderClear || run == nil ||
		run.Maze.Quest == 0 || en.ID != uint32(run.Maze.Quest) ||
		run.Definition.ID != en.UnderClear.Dungeon ||
		(en.UnderClear.MinDifficulty >= 0 && int32(run.Difficulty) < en.UnderClear.MinDifficulty) ||
		len(run.Maze.Layers) != 0 || len(run.Maze.Rooms) < 2 ||
		uint32(len(run.Maze.Rooms)-1) != en.UnderClear.RoomHint {
		return false
	}
	seen := make(map[uint32]bool, len(run.Maze.Rooms))
	for _, room := range run.Maze.Rooms {
		if room.Map == 0 || seen[room.Map] {
			return false
		}
		seen[room.Map] = true
		if _, visited := run.Visited[room.Map]; !visited {
			return false
		}
	}
	return true
}
