package dungeon

import (
	"dfolan/internal/catalog"
	"dfolan/internal/game/protocol"
	"testing"
)

func TestDelezieQuestWaitsForSourceClearMap(t *testing.T) {
	c, err := catalog.LoadDungeons("../../configs/dungeons.full.json")
	if err != nil {
		t.Fatal(err)
	}
	s, err := Select(c, protocol.DungeonSelection{ID: 7123, Quest: 23108, Party: 65535, Difficulty: 1}, 50, map[uint16]bool{23108: true})
	if err != nil {
		t.Fatal(err)
	}
	for x := byte(1); x <= 4; x++ {
		s.Loaded = true
		for _, monster := range s.Monsters {
			s.Dead[monster.Entity] = true
		}
		s, err = s.Move(c, [2]byte{x, 0})
		if err != nil {
			t.Fatalf("enter room %d: %v", x, err)
		}
	}
	s.Loaded = true
	// The first marked boss room contains only a display actor and settles on
	// load in the old implementation; it does not emit a native BossCheck.
	if !s.atSourceBossMap() || s.hasFightableBoss() || s.reportableDisplayBoss() == 0 {
		t.Fatal("source display boss room changed")
	}
	s.TryComplete()
	if s.Completed() {
		t.Fatal("boss room completed before quest clear map")
	}
	for x := byte(5); x <= 6; x++ {
		s, err = s.Move(c, [2]byte{x, 0})
		if err != nil {
			t.Fatalf("continue after boss to room %d: %v", x, err)
		}
		s.Loaded = true
		for _, monster := range s.Monsters {
			s.Dead[monster.Entity] = true
		}
		s.TryComplete()
		if s.Completed() {
			t.Fatalf("room %d completed before quest clear map", x)
		}
	}
	s, err = s.MoveScene(c, protocol.DungeonRoomTransition{Dungeon: 7123, Position: [2]byte{6, 0}, LayerChange: true})
	if err != nil {
		t.Fatal(err)
	}
	if s.Room.Map != 100008696 {
		t.Fatalf("final map = %d", s.Room.Map)
	}
	s.Loaded = true
	var boss uint16
	for _, monster := range s.Monsters {
		s.Dead[monster.Entity] = true
		if monster.Rank == 3 && monster.Team != 0 && !monster.NonCombat {
			boss = monster.Entity
		}
	}
	if boss == 0 {
		t.Fatal("final source boss missing")
	}
	if err := s.BossCheck(protocol.BossCheckRequest{Actor: 11, Target: boss}, 11); err != nil {
		t.Fatal(err)
	}
	s.TryComplete()
	if !s.Completed() {
		t.Fatal("source quest clear map did not settle run")
	}
	found := false
	for _, mapID := range s.ClearedMaps() {
		found = found || mapID == 100008696
	}
	if !found {
		t.Fatal("quest clear map absent from cleared map history")
	}
}
