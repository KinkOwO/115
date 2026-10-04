package main

import (
	"dfolan/internal/catalog"
	"dfolan/internal/database"
	"encoding/binary"
	"path/filepath"
	"testing"
)

func TestTrainingRoomEntryFrames(t *testing.T) {
	c, err := catalog.LoadDungeons(filepath.Join("..", "..", "configs", "dungeons.training-room.json"))
	if err != nil {
		t.Fatal(err)
	}
	w := &worldSession{role: database.Character{ID: 1}, level: 1, dungeons: &c}
	gate := make([]byte, 8)
	binary.LittleEndian.PutUint32(gate, 5000)
	plan, err := w.dungeonGate(gate)
	if err != nil || len(plan) != 2 || plan[0].ID != 15 || plan[1].ID != 27 {
		t.Fatalf("gate plan=%+v err=%v", plan, err)
	}
	selection := make([]byte, 32)
	binary.LittleEndian.PutUint32(selection, 5000)
	selection[8] = 1 // observed town-door Flag
	binary.LittleEndian.PutUint16(selection[9:], 65535)
	s, plan, err := w.selectDungeon(selection)
	if err != nil {
		t.Fatal(err)
	}
	if s.Room.Map != 36250 || !s.Definition.NoFatigue || len(plan) < 3 || plan[0].ID != 16 || plan[len(plan)-2].ID != 28 || plan[len(plan)-1].ID != 29 {
		t.Fatalf("entry map=%d no_fatigue=%v plan=%+v", s.Room.Map, s.Definition.NoFatigue, plan)
	}
}
