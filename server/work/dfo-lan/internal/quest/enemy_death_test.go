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

func TestSingleHuntSourceForms(t *testing.T) {
	c, err := catalog.LoadQuests(testfixture.CatalogPath(t, "quests"))
	if err != nil {
		t.Fatal(err)
	}
	x := BuildIndex(c)
	var supported, deferred []uint32
	for id, d := range c.Quests {
		if d.Kind != "[hunt enemy]" {
			continue
		}
		en := x.Entries[id]
		dungeonID, enemy, ok := HuntEnemyObjective(d)
		if ok {
			supported = append(supported, id)
			if !en.Implemented || en.Model != SingleHuntEnemy || en.HuntDungeon != dungeonID || en.HuntEnemy != enemy || en.Initial != 1 {
				t.Errorf("quest %d source hunt unavailable: %+v", id, en)
			}
		} else {
			deferred = append(deferred, id)
			if en.Implemented {
				t.Errorf("quest %d unsupported hunt shape was offered", id)
			}
		}
	}
	sort.Slice(supported, func(i, j int) bool { return supported[i] < supported[j] })
	sort.Slice(deferred, func(i, j int) bool { return deferred[i] < deferred[j] })
	if !reflect.DeepEqual(supported, []uint32{3526, 3586, 3587, 3596, 3598}) ||
		!reflect.DeepEqual(deferred, []uint32{2942, 6705, 6710, 12476}) {
		t.Fatalf("hunt source coverage changed: supported=%v deferred=%v", supported, deferred)
	}
	if !prerequisitesMet(x.Entries[3526].PrerequisiteGroups, map[uint32]string{3525: "completed"}) ||
		!prerequisitesMet(x.Entries[3529].PrerequisiteGroups, map[uint32]string{3526: "completed"}) {
		t.Fatal("level-56 to 57 story chain is not connected")
	}
}

func TestSingleHuntRequiresConfirmedTargetDeathInQuestMaze(t *testing.T) {
	en := &Entry{ID: 3586, Implemented: true, Model: SingleHuntEnemy, HuntDungeon: 92, HuntEnemy: 66309}
	run := &dungeon.Session{
		Loaded: true, Definition: catalog.DungeonDefinition{ID: 92},
		Maze:     catalog.DungeonMaze{Quest: 3586},
		Monsters: []protocol.DungeonMonster{{Entity: 4096, Template: 66309}, {Entity: 4097, Template: 61495}},
		Dead:     map[uint16]bool{4096: true},
	}
	if !singleHuntMatch(en, run, 4096) {
		t.Fatal("confirmed source target death did not satisfy hunt")
	}
	run.Unowned = map[uint16]bool{4096: true}
	if !singleHuntMatch(en, run, 4096) {
		t.Fatal("confirmed scripted target death did not satisfy hunt")
	}
	for _, change := range []func(*dungeon.Session){
		func(s *dungeon.Session) { s.Dead[4096] = false },
		func(s *dungeon.Session) { s.Maze.Quest = 3587 },
		func(s *dungeon.Session) { s.Definition.ID = 93 },
		func(s *dungeon.Session) { s.Monsters[0].Template = 61495 },
		func(s *dungeon.Session) { s.Monsters[0].Entity = 4098 },
		func(s *dungeon.Session) { s.Loaded = false },
	} {
		copy := *run
		copy.Dead = map[uint16]bool{4096: true}
		copy.Monsters = append([]protocol.DungeonMonster(nil), run.Monsters...)
		change(&copy)
		if singleHuntMatch(en, &copy, 4096) {
			t.Fatal("unconfirmed, wrong-route, or wrong-target death advanced hunt")
		}
	}
}
