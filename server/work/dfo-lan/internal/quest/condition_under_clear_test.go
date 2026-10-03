package quest

import (
	"dfolan/internal/catalog"
	"dfolan/internal/dungeon"
	"dfolan/internal/game/protocol"
	"dfolan/internal/testfixture"
	"reflect"
	"sort"
	"testing"
)

func TestCurrentStoryUnderClearSourceAndRoute(t *testing.T) {
	quests, err := catalog.LoadQuests(testfixture.CatalogPath(t, "quests"))
	if err != nil {
		t.Fatal(err)
	}
	dungeons := catalog.LoadNativeFullDungeons(t)
	x := BuildIndex(quests)
	var supported []uint32
	for id, q := range quests.Quests {
		if _, ok := ConditionUnderClearObjective(q); ok {
			supported = append(supported, id)
		}
	}
	sort.Slice(supported, func(i, j int) bool { return supported[i] < supported[j] })
	if !reflect.DeepEqual(supported, []uint32{3543, 13130}) {
		t.Fatalf("unexpected subtype-11 source forms: %v", supported)
	}
	for _, id := range supported {
		en := x.Entries[id]
		if en == nil || !en.Implemented || !en.RewardUsable || en.Model != AllRoomsUnderClear || en.Initial != 1 {
			t.Fatalf("quest %d source form unavailable: %+v", id, en)
		}
		if id == 3543 && !prerequisitesMet(en.PrerequisiteGroups, map[uint32]string{3540: "completed"}) {
			t.Fatal("level-59 successor has wrong prerequisite")
		}
		definition := dungeons.Dungeons[en.UnderClear.Dungeon]
		var maze catalog.DungeonMaze
		for _, candidate := range definition.Mazes {
			if uint32(candidate.Quest) == en.ID {
				maze = candidate
			}
		}
		if maze.Quest == 0 {
			t.Fatalf("quest %d has no source maze", id)
		}
		run := &dungeon.Session{Definition: definition, Maze: maze, Difficulty: 1}
		run.Visited = make(map[uint32][]protocol.DungeonMonster, len(maze.Rooms))
		for _, room := range maze.Rooms {
			run.Visited[room.Map] = nil
		}
		if !allRoomsUnderClearMatch(en, run) {
			t.Fatalf("quest %d all source rooms did not match", id)
		}
		for _, changed := range []func(*dungeon.Session){
			func(s *dungeon.Session) { delete(s.Visited, maze.Rooms[0].Map) },
			func(s *dungeon.Session) { s.Maze.Quest++ },
			func(s *dungeon.Session) { s.Definition.ID++ },
		} {
			copy := *run
			copy.Visited = make(map[uint32][]protocol.DungeonMonster, len(run.Visited))
			for mapID, monsters := range run.Visited {
				copy.Visited[mapID] = monsters
			}
			changed(&copy)
			if allRoomsUnderClearMatch(en, &copy) {
				t.Fatalf("quest %d foreign or incomplete run matched", id)
			}
		}
		badHint := *en
		badHint.UnderClear.RoomHint++
		if allRoomsUnderClearMatch(&badHint, run) {
			t.Fatalf("quest %d accepted an inconsistent room hint", id)
		}
		tooHard := *en
		tooHard.UnderClear.MinDifficulty = 2
		if allRoomsUnderClearMatch(&tooHard, run) {
			t.Fatalf("quest %d ignored its minimum difficulty", id)
		}
	}
}
