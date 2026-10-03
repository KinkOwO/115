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

func TestEpicHuntMonsterSourceRoutes(t *testing.T) {
	quests, err := catalog.LoadQuests(testfixture.CatalogPath(t, "quests"))
	if err != nil {
		t.Fatal(err)
	}
	dungeons := catalog.LoadNativeFullDungeons(t)
	x := BuildIndex(quests)
	var supported []uint32
	for id, q := range quests.Quests {
		dungeonID, target, ok := HuntMonsterObjective(q)
		if !ok {
			continue
		}
		supported = append(supported, id)
		en := x.Entries[id]
		if en == nil || !en.Implemented || !en.RewardUsable || en.Model != SingleHuntMonster ||
			en.Initial != 1 || en.HuntDungeon != dungeonID || en.HuntMonster != target {
			t.Fatalf("quest %d cannot be accepted under its source target: %+v", id, en)
		}
		d := dungeons.Dungeons[dungeonID]
		var maze *catalog.DungeonMaze
		for i := range d.Mazes {
			if uint32(d.Mazes[i].Quest) == id {
				maze = &d.Mazes[i]
			}
		}
		if maze == nil {
			t.Fatalf("quest %d has no dedicated source maze", id)
		}
		found := false
		for _, room := range maze.Rooms {
			for _, cell := range dungeons.Maps[room.Map].Cells {
				if cell.Type == 0 && cell.Value == int32(target) {
					found = true
				}
			}
		}
		if !found {
			t.Fatalf("quest %d target %d absent from its source maze", id, target)
		}
	}
	sort.Slice(supported, func(i, j int) bool { return supported[i] < supported[j] })
	if !reflect.DeepEqual(supported, []uint32{3571, 3579, 3740, 3752, 3754, 3849, 6210}) {
		t.Fatalf("epic single-target source set changed: %v", supported)
	}
	if !prerequisitesMet(x.Entries[3571].PrerequisiteGroups, map[uint32]string{3570: "completed"}) ||
		!prerequisitesMet(x.Entries[3573].PrerequisiteGroups, map[uint32]string{3571: "completed"}) {
		t.Fatal("level-62 mainline prerequisite chain is disconnected")
	}
}

func TestHuntMonsterDeathRequiresConfirmedSourceTarget(t *testing.T) {
	en := &Entry{ID: 3571, Implemented: true, Model: SingleHuntMonster, HuntDungeon: 86, HuntMonster: 65472}
	run := &dungeon.Session{
		Loaded: true, Definition: catalog.DungeonDefinition{ID: 86},
		Maze:     catalog.DungeonMaze{Quest: 3571},
		Monsters: []protocol.DungeonMonster{{Entity: 4096, Template: 65472}, {Entity: 4097, Template: 65473}},
		Dead:     map[uint16]bool{4096: true},
	}
	if !singleKillMatch(en, run, 4096, en.HuntMonster) {
		t.Fatal("confirmed source target death did not advance hunt")
	}
	run.Unowned = map[uint16]bool{4096: true}
	if !singleKillMatch(en, run, 4096, en.HuntMonster) {
		t.Fatal("confirmed scripted target death did not advance hunt monster")
	}
	for _, mutate := range []func(*dungeon.Session){
		func(s *dungeon.Session) { delete(s.Dead, 4096) },
		func(s *dungeon.Session) { s.Maze.Quest++ },
		func(s *dungeon.Session) { s.Definition.ID++ },
		func(s *dungeon.Session) { s.Monsters[0].Template++ },
		func(s *dungeon.Session) { s.Monsters[0].Entity++ },
		func(s *dungeon.Session) { s.Loaded = false },
	} {
		copy := *run
		copy.Dead = map[uint16]bool{4096: true}
		copy.Monsters = append([]protocol.DungeonMonster(nil), run.Monsters...)
		mutate(&copy)
		if singleKillMatch(en, &copy, 4096, en.HuntMonster) {
			t.Fatal("unconfirmed or wrong-target death advanced hunt")
		}
	}
}
