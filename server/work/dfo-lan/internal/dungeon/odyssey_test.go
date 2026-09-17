package dungeon

import (
	"dfolan/internal/catalog"
	"dfolan/internal/game/protocol"
	"encoding/hex"
	"reflect"
	"testing"
)

func TestCapturedOdysseyEntry(t *testing.T) {
	c, e := catalog.LoadDungeons("../../configs/dungeons.odyssey-candidate.json")
	if e != nil {
		t.Fatal(e)
	}
	raw, _ := hex.DecodeString("46f4f5050200000000ffff000000000000000000000000000000000000000000")
	r, e := protocol.DecodeDungeonSelection(raw)
	if e != nil {
		t.Fatal(e)
	}
	s, e := Select(c, r, 1, nil)
	if e != nil {
		t.Fatal(e)
	}
	if !s.Definition.Odyssey || s.Definition.DesignatedDifficulty != 2 || s.Room.Map != 100016498 || len(s.Maze.Rooms) != 5 {
		t.Fatalf("wrong source route: %+v", s)
	}
	for _, room := range s.Maze.Rooms {
		if _, e := fixedMonsters(c.Maps[room.Map], s.Definition.BasisLevel); e != nil {
			t.Fatalf("map%d: %v", room.Map, e)
		}
	}
	for _, difficulty := range []byte{0, 1, 3, 4} {
		r.Difficulty = difficulty
		if _, e = Select(c, r, 1, nil); e == nil {
			t.Fatalf("accepted difficulty%d", difficulty)
		}
	}
	r.Difficulty = 2
	if _, e = Select(c, r, 0, nil); e == nil {
		t.Fatal("accepted below minimum level")
	}
	r.Event = 1
	if _, e = Select(c, r, 1, nil); e == nil {
		t.Fatal("accepted unrelated event option")
	}
}

func TestOrdinaryDungeonsUnchangedByOdysseyMerge(t *testing.T) {
	before, e := catalog.LoadDungeons("../../configs/dungeons.next28.json")
	if e != nil {
		t.Fatal(e)
	}
	after, e := catalog.LoadDungeons("../../configs/dungeons.odyssey-merged-candidate.json")
	if e != nil {
		t.Fatal(e)
	}
	for id, d := range before.Dungeons {
		if !reflect.DeepEqual(d, after.Dungeons[id]) {
			t.Fatalf("ordinary dungeon%d changed", id)
		}
	}
	for id, m := range before.Maps {
		if !reflect.DeepEqual(m, after.Maps[id]) {
			t.Fatalf("ordinary map%d changed", id)
		}
	}
	r := protocol.DungeonSelection{ID: 3, Party: 65535, Quest: 3145}
	if _, e = Select(after, r, 1, map[uint16]bool{3145: true}); e != nil {
		t.Fatal(e)
	}
	r.Difficulty = 6
	if _, e = Select(after, r, 1, map[uint16]bool{3145: true}); e == nil {
		t.Fatal("ordinary dungeon accepted unsupported difficulty")
	}
}
