package main

import (
	"dfolan/internal/catalog"
	"dfolan/internal/dungeon"
	"dfolan/internal/game/protocol"
	"dfolan/internal/legion"
	"encoding/hex"
	"testing"
)

// Native CMD45 from the reported Numak phase2 door at21:10:28 local time.
const ispinsPhase2DoorMove = "00015f0000002701000000a3f80000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000007300704002005f00000027010000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000005ffffffffffff00000000000000abecf5050000000000"

func TestIspinsPhase2NativeDoorMove(t *testing.T) {
	c := catalog.LoadNativeDungeons(t, legion.IspinsStageDungeons[0])
	s, err := dungeon.Select(c, protocol.DungeonSelection{ID: legion.IspinsStageDungeons[0], Difficulty: 4, Party: 65535}, 140, nil)
	if err != nil {
		t.Fatal(err)
	}
	s.Loaded = true
	if s.Room.Map != 100006476 || s.RoomCleared() || len(s.LivingMonsters()) == 0 {
		t.Fatal("native source must retain live actors in the starting room")
	}
	w := &worldSession{dungeons: &c, activeDungeon: s, ispins: &ispinsRun{confirmed: true}}
	body, err := hex.DecodeString(ispinsPhase2DoorMove)
	if err != nil {
		t.Fatal(err)
	}
	next, packets, err := w.moveDungeonRoom(body)
	if err != nil {
		t.Fatalf("reported native door refused: %v", err)
	}
	if next.Room.Map != 100006475 || next.RunID != s.RunID || next.Loaded || next.Completed() || len(next.Dead) != 0 || w.ispins.cleared != [4]bool{} {
		t.Fatal("movement corrupted source room/encounter/legion completion")
	}
	if len(packets) != 2 || packets[0].ID != 45 || packets[0].Kind != 1 || string(packets[0].Payload) != "\x01" || packets[1].ID != 29 {
		t.Fatalf("native door must use existing ACK45 and N29 grammar: %+v", packets)
	}
	for _, target := range [][2]byte{{1, 2}, {2, 1}, {1, 0}} {
		request := append([]byte(nil), body...)
		request[0], request[1] = target[0], target[1]
		if _, _, err := w.moveDungeonRoom(request); err != nil {
			t.Fatalf("native source door %v refused: %v", target, err)
		}
	}
}
