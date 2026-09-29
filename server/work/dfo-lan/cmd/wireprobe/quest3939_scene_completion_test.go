package main

import (
	"bytes"
	"dfolan/internal/catalog"
	"dfolan/internal/dungeon"
	"dfolan/internal/game/protocol"
	"testing"
)

// Live 2026-09-29: map 312116 has one rank-0 source actor. Its death
// report is followed by CMD33 for quest 3939, never a boss check. The scene
// completion must reach clear-enable without trying to confirm entity zero.
func TestQuest3939SceneCompletionWithoutBossIdentity(t *testing.T) {
	c, err := catalog.LoadDungeons("../../configs/dungeons.full.json")
	if err != nil {
		t.Fatal(err)
	}
	s, err := dungeon.Select(c, protocol.DungeonSelection{ID: 5109, Difficulty: 1, Party: 65535, Quest: 3939}, 102, map[uint16]bool{3939: true})
	if err != nil {
		t.Fatal(err)
	}
	for _, xy := range [][2]byte{{5, 0}, {4, 0}, {3, 0}, {2, 0}, {1, 0}, {0, 0}} {
		s.Loaded = true
		for _, m := range s.Monsters {
			if _, err = s.ConfirmDeath(uint32(m.Entity), 11, 11); err != nil {
				t.Fatal(err)
			}
		}
		s, err = s.Move(c, xy)
		if err != nil {
			t.Fatal(err)
		}
	}
	s.Loaded = true
	if s.Room.Map != 312116 || len(s.Monsters) != 1 || s.Monsters[0].Rank != 0 {
		t.Fatalf("source scene shape changed: room=%+v monsters=%+v", s.Room, s.Monsters)
	}
	if _, err = s.ConfirmDeath(uint32(s.Monsters[0].Entity), 11, 11); err != nil {
		t.Fatal(err)
	}
	if s.Completed() {
		t.Fatal("scene must wait for the quest trigger after actor death")
	}
	s.MarkSceneCompleted()
	if s.CompletionTarget() != 0 || s.CompletionNeedsBossCheck() {
		t.Fatal("scene without a source boss requested boss confirmation")
	}
	w := &worldSession{activeDungeon: s}
	plan, err := w.completeDungeon()
	if err != nil {
		t.Fatalf("scene completion dropped: %v", err)
	}
	if len(plan) != 1 || plan[0].ID != 31 || plan[0].Kind != 0 || !bytes.Equal(plan[0].Payload, protocol.DungeonClearEnabled()) {
		t.Fatalf("missing native clear-enable: %+v", plan)
	}
	w.completionSent = true
	if retry, err := w.completeDungeon(); err != nil || len(retry) != 0 {
		t.Fatalf("completion replay: %+v %v", retry, err)
	}
}

func TestSceneCompletionKeepsRealBossConfirmation(t *testing.T) {
	s := &dungeon.Session{Monsters: []protocol.DungeonMonster{{Entity: 0x102f, Rank: 3, Team: 100}}}
	s.MarkSceneCompleted()
	w := &worldSession{activeDungeon: s}
	plan, err := w.completeDungeon()
	if err != nil {
		t.Fatal(err)
	}
	if len(plan) != 2 || plan[0].ID != 115 || !bytes.Equal(plan[0].Payload, []byte{1, 1, 0x2f, 0x10}) || plan[1].ID != 31 {
		t.Fatalf("real boss confirmation changed: %+v", plan)
	}
}
