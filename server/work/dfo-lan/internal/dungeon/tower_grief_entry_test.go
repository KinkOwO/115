package dungeon

import (
	"dfolan/internal/catalog"
	"dfolan/internal/game/protocol"
	"testing"
)

func TestTowerGriefSourceEntry(t *testing.T) {
	a := catalog.OpenNativeArchive(t)
	c := catalog.LoadNativeFullDungeons(t)
	if _, err := Select(c, protocol.DungeonSelection{ID: 5115, Party: 65535}, 95, nil); err == nil {
		t.Fatal("unresolved source tower maze unexpectedly entered without overlay")
	}
	overlay, err := catalog.ImportTowerGriefOverlay(a)
	if err != nil {
		t.Fatal(err)
	}
	if err := catalog.ApplyTowerGriefMaps(&c, overlay); err != nil {
		t.Fatal(err)
	}
	floors, err := c.TowerGriefFloors()
	if err != nil || floors[1] != 5115 || floors[2] != 5116 || floors[100] != 5214 {
		t.Fatalf("source floor mapping: floor1=%d floor2=%d floor100=%d error=%v", floors[1], floors[2], floors[100], err)
	}
	sharedFloors, err := c.TowerFloors("grief", 100)
	if err != nil || sharedFloors[1] != floors[1] || sharedFloors[2] != floors[2] || sharedFloors[100] != floors[100] {
		t.Fatalf("shared tower floor lookup: %v, %v", sharedFloors, err)
	}
	for _, check := range []struct {
		dungeon, mapID uint32
		floor          uint16
	}{{5115, 100247, 1}, {5116, 100248, 2}, {5214, 100346, 100}} {
		s, err := Select(c, protocol.DungeonSelection{ID: check.dungeon, Party: 65535}, 95, nil)
		if err != nil {
			t.Fatalf("dungeon %d: %v", check.dungeon, err)
		}
		if s.Room.Map != check.mapID || s.Maze.Size != [2]byte{1, 1} || !s.Room.Boss || s.Definition.TowerGriefFloor != check.floor || s.Definition.Tower == nil || s.Definition.Tower.Floor != check.floor || s.Definition.Tower.DailyEntries != 1 {
			t.Fatalf("dungeon %d resolved wrong source room: %+v", check.dungeon, s.Room)
		}
		wantRule := "tower_grief_reward_normal"
		if check.floor == 100 {
			wantRule = "tower_grief_reward_special"
		}
		if s.Definition.Tower.RewardRule != wantRule {
			t.Fatalf("floor %d reward rule = %q", check.floor, s.Definition.Tower.RewardRule)
		}
		boss := false
		for _, m := range s.Monsters {
			boss = boss || m.APC && m.Rank == 8 && m.Team == 100
		}
		if !boss {
			t.Fatalf("dungeon %d has no source APC boss", check.dungeon)
		}
		if _, err := protocol.StartMap(protocol.StartMapState{Position: s.Maze.Start, Map: s.Room.Map, Monsters: s.Monsters}); err != nil {
			t.Fatalf("dungeon %d start map encoding: %v", check.dungeon, err)
		}
	}
}
