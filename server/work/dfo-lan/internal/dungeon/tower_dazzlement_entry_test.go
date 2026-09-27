package dungeon

import (
	"dfolan/internal/catalog"
	"dfolan/internal/game/protocol"
	"path/filepath"
	"strings"
	"testing"
)

func TestDazzlementSourceEntry(t *testing.T) {
	configDir := filepath.Join("..", "..", "configs")
	c, err := catalog.LoadDungeons(filepath.Join(configDir, "dungeons.full.json"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Select(c, protocol.DungeonSelection{ID: 7601, Party: 65535}, 95, nil); err == nil {
		t.Fatal("unresolved Dazzlement maze entered without source maps")
	}
	if err := catalog.AttachDazzlementMaps(&c, filepath.Join(configDir, "dungeons.tower-of-dazzlement-maps.json")); err != nil {
		t.Fatal(err)
	}
	parsedMaps := 0
	for mapID, script := range c.Maps {
		if !strings.HasPrefix(script.Path, "map/towerofdazzlement/") {
			continue
		}
		if _, err := fixedMonsters(script, 95); err != nil {
			t.Fatalf("Dazzlement map %d spawn parser: %v", mapID, err)
		}
		parsedMaps++
	}
	if parsedMaps != 56 {
		t.Fatalf("parsed %d Dazzlement maps, want 56", parsedMaps)
	}
	for _, group := range []struct {
		first, last uint32
		rooms       int
	}{{7601, 7620, 2}, {7651, 7660, 1}, {7701, 7703, 2}} {
		for id := group.first; id <= group.last; id++ {
			s, err := Select(c, protocol.DungeonSelection{ID: id, Party: 65535}, 95, nil)
			if err != nil {
				t.Fatalf("Dazzlement dungeon %d: %v", id, err)
			}
			if len(s.Maze.Rooms) != group.rooms || s.Room.X != 0 || s.Room.Map == 0 || s.Definition.Tower != nil {
				t.Fatalf("Dazzlement dungeon %d wrong source entry: %+v", id, s.Room)
			}
			if group.rooms == 2 && (s.Maze.Rooms[1].Map == 0 || !s.Maze.Rooms[1].Boss) {
				t.Fatalf("Dazzlement dungeon %d missing boss room", id)
			}
			if _, err := protocol.StartMap(protocol.StartMapState{Position: s.Maze.Start, Map: s.Room.Map, Monsters: s.Monsters}); err != nil {
				t.Fatalf("Dazzlement dungeon %d entry packet: %v", id, err)
			}
		}
	}
}
