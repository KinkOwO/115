package dungeon

import (
	"dfolan/internal/catalog"
	"dfolan/internal/game/protocol"
	"encoding/hex"
	"reflect"
	"testing"
)

func TestSkycastleCapturedEntry(t *testing.T) {
	old, err := catalog.LoadDungeons("../../configs/dungeons.odyssey-release.json")
	if err != nil {
		t.Fatal(err)
	}
	b, _ := hex.DecodeString("49f4f5050200000000ffff000000000000000000000000000000000000000000")
	r, err := protocol.DecodeDungeonSelection(b)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = Select(old, r, 35, nil); err == nil {
		t.Fatal("baseline unexpectedly admitted unresolved layers")
	}
	t.Log("BASELINE: no resolved source maze for requested quest")
	c, err := catalog.LoadDungeons("../../configs/dungeons.skycastle-candidate.json")
	if err != nil {
		t.Fatal(err)
	}
	for id, d := range old.Dungeons {
		if id != r.ID && !reflect.DeepEqual(d, c.Dungeons[id]) {
			t.Fatalf("unrelated dungeon changed: %d", id)
		}
	}
	s, err := Select(c, r, 35, nil)
	if err != nil {
		t.Fatal(err)
	}
	if s.Room.Map != 100015988 || len(s.Maze.Rooms) != 8 || len(s.Maze.Layers) != 1 || !reflect.DeepEqual(s.Maze.Layers[0].Maps, []uint32{100016000, 100016001, 100016002, 100016003}) {
		t.Fatal(s.Maze)
	}
	for _, room := range s.Maze.Rooms {
		if _, err = fixedMonsters(c.Maps[room.Map], s.Definition.BasisLevel); err != nil {
			t.Fatalf("map %d: %v", room.Map, err)
		}
	}
	for _, id := range s.Maze.Layers[0].Maps {
		if c.Maps[id].Path == "" {
			t.Fatal("layer omitted", id)
		}
	}
	if _, err = Select(c, r, 34, nil); err == nil {
		t.Fatal("level gate bypassed")
	}
	r.Difficulty = 0
	if _, err = Select(c, r, 35, nil); err == nil {
		t.Fatal("difficulty bypassed")
	}
	t.Log("MODIFIED: captured CMD16 accepted at35; 8 base rooms and4 layer maps preserved; level34 and wrong difficulty refused")
}
