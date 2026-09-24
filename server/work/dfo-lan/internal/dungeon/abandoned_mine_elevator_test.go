package dungeon

import (
	"dfolan/internal/catalog"
	"path/filepath"
	"testing"
)

func TestAbandonedMineElevatorRoomDoesNotSpawnOffMapSentinel(t *testing.T) {
	c, err := catalog.LoadDungeons(filepath.Join("..", "..", "configs", "dungeons.full.json"))
	if err != nil {
		t.Fatal(err)
	}
	d := c.Dungeons[53]
	room := c.Maps[76384]
	if d.Mazes[2].Quest != 3352 || room.Path != "map/cataclysm/northmyre/05_town_of_doubt/3352_76384.map" {
		t.Fatal("quest 3352 elevator source changed")
	}
	monsters, err := fixedMonsters(room, d.BasisLevel)
	if err != nil {
		t.Fatal(err)
	}
	if len(monsters) != 0 {
		t.Fatalf("elevator room still has %d permanent monsters", len(monsters))
	}
}
