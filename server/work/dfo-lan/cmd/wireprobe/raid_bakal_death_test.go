package main

import (
	"bytes"
	"dfolan/internal/catalog"
	"dfolan/internal/character"
	"dfolan/internal/database"
	"dfolan/internal/dungeon"
	"dfolan/internal/game/protocol"
	"dfolan/internal/raid"
	"encoding/binary"
	"os"
	"testing"
)

// Reproduce the actual battlefield, death response and adjoining room using
// the native PVF. Disabled rewards must not require a character transaction.
func TestBakalNativeDeathsRemoveMonstersAndAllowNextRoom(t *testing.T) {
	path := os.Getenv("DFO_PVF_CORE_TEST_ARCHIVE")
	if path == "" {
		t.Skip("native PVF archive required")
	}
	a, err := catalog.OpenTestArchiveCached(path, "")
	if err != nil {
		t.Fatal(err)
	}
	c, err := catalog.ImportDungeons(a, []uint32{100003160})
	if err != nil {
		t.Fatal(err)
	}
	run, err := dungeon.Select(c, protocol.DungeonSelection{ID: 100003160, Party: 65535}, 115, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !run.Definition.ExperienceDisabled() || !run.Definition.ItemDropsDisabled() || len(run.Monsters) == 0 {
		t.Fatal("missing native reward flags or monsters")
	}
	run.Loaded = true
	ps := &character.ProgressionService{} // nil Store proves no reward transaction.
	w := &worldSession{role: database.Character{ID: 2, WireID: 2, State: []byte(`{"level":115}`), ConfigVersion: ps.Catalog.Source.SaveIdentity()}, level: 115,
		activeDungeon: run, progression: ps, deathSent: map[uint16]bool{}, bakalOpening: &raid.BakalOpening{}}
	for _, m := range run.Monsters {
		if m.NonCombat {
			continue
		}
		body := make([]byte, 64)
		binary.LittleEndian.PutUint32(body, uint32(m.Entity))
		binary.LittleEndian.PutUint16(body[4:], 2)
		plan, err := w.monsterDeath(body, func(map[string]any) {})
		if err != nil {
			t.Fatalf("entity %d death withheld: %v", m.Entity, err)
		}
		removed := false
		for _, p := range plan {
			if p.ID == 38 && bytes.Equal(p.Payload, protocol.MonsterDeathConfirmed(m.Entity)) {
				removed = true
			}
		}
		if len(plan) == 0 || plan[0].ID != 39 || !removed {
			t.Fatalf("missing death acknowledgement/removal for %d", m.Entity)
		}
		w.deathSent[m.Entity] = true
	}
	if !run.RoomCleared() {
		t.Fatal("room still blocked after actual kills")
	}
	for _, r := range run.Maze.Rooms {
		dx, dy := int(r.X)-int(run.Room.X), int(r.Y)-int(run.Room.Y)
		if dx*dx+dy*dy == 1 {
			next, err := run.Move(c, [2]byte{r.X, r.Y})
			if err != nil || next.Room.Map == run.Room.Map {
				t.Fatal("next room blocked", err)
			}
			return
		}
	}
	t.Fatal("native battlefield has no adjoining room")
}
