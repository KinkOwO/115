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
	after, e := catalog.LoadDungeons("../../configs/dungeons.odyssey-release.json")
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

func TestFixedMonstersZeroLevelFallback(t *testing.T) {
	c, e := catalog.LoadDungeons("../../configs/dungeons.odyssey-release.json")
	if e != nil {
		t.Fatal(e)
	}
	m, ok := c.Maps[100016332]
	if !ok {
		t.Fatal("map 100016332 not found in odyssey-release")
	}
	monsters, err := fixedMonsters(m, 92)
	if err != nil {
		t.Fatalf("fixedMonsters failed: %v", err)
	}
	foundZeroSrc := false
	for _, mon := range monsters {
		if mon.Template == 109019135 {
			foundZeroSrc = true
			if mon.Level != 92 {
				t.Fatalf("expected template 109019135 level to be fallback to basis 92, got %d", mon.Level)
			}
		}
	}
	if !foundZeroSrc {
		t.Fatal("monster 109019135 not found in map 100016332")
	}
}

func TestOdysseyRaidRoomCleared(t *testing.T) {
	c, e := catalog.LoadDungeons("../../configs/dungeons.odyssey-scenes-release.json")
	if e != nil {
		t.Fatal(e)
	}
	s, e := Select(c, protocol.DungeonSelection{ID: 100004965, Difficulty: 2, Party: 65535}, 90, nil)
	if e != nil {
		t.Fatal(e)
	}
	s.Loaded = true
	// All Odyssey dungeons (including < 100004960 and >= 100004960) should be cleared immediately
	// without waiting for un-reported boss deaths
	if !s.RoomCleared() {
		t.Fatal("expected Odyssey raid 100004965 room to be cleared")
	}

	sEarly, e := Select(c, protocol.DungeonSelection{ID: 100004938, Difficulty: 2, Party: 65535}, 37, nil)
	if e != nil {
		t.Fatal(e)
	}
	sEarly.Loaded = true
	if !sEarly.RoomCleared() {
		t.Fatal("expected early Odyssey dungeon 100004938 room to be cleared")
	}

	// Sirocco cutscene room 100016294
	cutsceneSession := &Session{
		Loaded: true,
		Room:   catalog.DungeonRoom{Map: 100016294},
	}
	if !cutsceneSession.RoomCleared() {
		t.Fatal("expected Sirocco cutscene room 100016294 to be cleared")
	}
}
