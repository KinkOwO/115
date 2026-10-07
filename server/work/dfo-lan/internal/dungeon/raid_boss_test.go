package dungeon

import (
	"dfolan/internal/catalog"
	"testing"
)

func TestRaidDynamicBossReopensEmptyArenaAndDoesNotDuplicate(t *testing.T) {
	s := &Session{Definition: catalog.DungeonDefinition{BasisLevel: 110}, Room: catalog.DungeonRoom{X: 1, Y: 2, Map: 99}, Maze: catalog.DungeonMaze{Boss: [2]byte{1, 2}}, NextEntity: 4096, completed: true}
	m := catalog.BakalRaidMonster{ID: 109014482, X: 945, Y: 311}
	row, spawn, err := s.AddRaidBoss(m)
	if err != nil || !spawn || row.Entity != 4096 || row.X != 945 || row.Y != 311 || row.Rank != 3 || s.completed || len(s.Monsters) != 1 || s.NextEntity != 4097 {
		t.Fatalf("dynamic arena ownership lost: %+v %+v %v %v", s, row, spawn, err)
	}
	_, spawn, err = s.AddRaidBoss(m)
	if err != nil || spawn || len(s.Monsters) != 1 || s.NextEntity != 4097 {
		t.Fatal("loading replay duplicated raid boss")
	}
	s.Room.X = 0
	if _, _, err = s.AddRaidBoss(m); err == nil {
		t.Fatal("spawn outside source boss room")
	}
}
