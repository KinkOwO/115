package dungeon

import (
	"testing"

	"dfolan/internal/catalog"
)

func TestArcherTutorialSecondMapOffMapActorDoesNotBlockExit(t *testing.T) {
	c, err := catalog.LoadDungeons("../../configs/tutorial-dungeons.current36.json")
	if err != nil {
		t.Fatal(err)
	}
	d := c.Dungeons[100003327]
	maze := d.Mazes[0]
	room := maze.Rooms[1]
	monsters, err := fixedMonsters(c.Maps[room.Map], d.BasisLevel)
	if err != nil {
		t.Fatal(err)
	}
	if len(monsters) != 3 {
		t.Fatalf("source actor rows changed: got %d, want 3", len(monsters))
	}
	actor := monsters[2]
	if actor.Template != 70216 || actor.SourceIndex != 2 || actor.Team != 0 || !actor.NonCombat {
		t.Fatalf("off-map source actor must keep its index but not block the exit: %+v", actor)
	}

	run := &Session{
		Definition: d,
		Maze:       maze,
		Room:       room,
		Monsters:   monsters,
		Dead:       map[uint16]bool{},
		Loaded:     true,
	}
	for _, monster := range monsters {
		if monster.NonCombat {
			continue
		}
		if _, err := run.ConfirmDeath(uint32(monster.Entity), 8, 8); err != nil {
			t.Fatal(err)
		}
	}
	if !run.RoomCleared() {
		t.Fatal("unreachable off-map actor kept the tutorial room occupied")
	}
	next, err := run.Move(c, [2]byte{2, 0})
	if err != nil {
		t.Fatalf("could not advance from tutorial map 100008880: %v", err)
	}
	if next.Room.Map != 100008881 {
		t.Fatalf("advanced to map %d, want 100008881", next.Room.Map)
	}
}

func TestGunbladerTutorialSecondMapOffMapActorDoesNotBlockExit(t *testing.T) {
	c, err := catalog.LoadDungeons("../../configs/tutorial-dungeons.current36.json")
	if err != nil {
		t.Fatal(err)
	}
	d := c.Dungeons[7145]
	maze := d.Mazes[0]
	room := maze.Rooms[1]
	monsters, err := fixedMonsters(c.Maps[room.Map], d.BasisLevel)
	if err != nil {
		t.Fatal(err)
	}
	if room.Map != 70577 || len(monsters) != 4 {
		t.Fatalf("unexpected gunblader second room: map=%d monsters=%d", room.Map, len(monsters))
	}
	actor := monsters[3]
	if actor.Template != 70216 || actor.SourceIndex != 3 || actor.Team != 0 || !actor.NonCombat {
		t.Fatalf("off-map source actor must keep its index but not block the exit: %+v", actor)
	}

	run := &Session{
		Definition: d,
		Maze:       maze,
		Room:       room,
		Monsters:   monsters,
		Dead:       map[uint16]bool{},
		Loaded:     true,
	}
	for _, monster := range monsters {
		if monster.NonCombat {
			continue
		}
		if _, err := run.ConfirmDeath(uint32(monster.Entity), 8, 8); err != nil {
			t.Fatal(err)
		}
	}
	if !run.RoomCleared() {
		t.Fatal("unreachable off-map actor kept the tutorial room occupied")
	}
	next, err := run.Move(c, [2]byte{2, 0})
	if err != nil {
		t.Fatalf("could not advance from tutorial map 70577: %v", err)
	}
	if next.Room.Map != 70578 {
		t.Fatalf("advanced to map %d, want 70578", next.Room.Map)
	}
}
