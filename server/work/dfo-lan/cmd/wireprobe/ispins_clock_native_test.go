package main

import (
	"dfolan/internal/catalog"
	"dfolan/internal/dungeon"
	"dfolan/internal/game/protocol"
	"dfolan/internal/legion"
	"encoding/binary"
	"encoding/hex"
	"testing"
	"time"
)

func TestIspinsNativeStagesUseOwnedClockAndSourceDoors(t *testing.T) {
	c := catalog.LoadNativeDungeons(t, legion.IspinsStageDungeons[:]...)
	for stage := 0; stage < 4; stage++ {
		w := &worldSession{dungeons: &c, level: 140, ispins: &ispinsRun{confirmed: true}}
		for previous := 0; previous < stage; previous++ {
			w.ispins.cleared[previous] = true
		}
		request := make([]byte, 24)
		binary.LittleEndian.PutUint32(request[13:], legion.IspinsContentID)
		binary.LittleEndian.PutUint32(request[17:], uint32(stage))
		before := time.Now().Unix()
		packets, _, err := w.enterIspinsStage(request)
		if err != nil {
			t.Fatal(stage, err)
		}
		count, timerAt, noticeAt, endAt := 0, -1, -1, -1
		for at, packet := range packets {
			switch packet.ID {
			case 390:
				noticeAt = at
			case 30:
				endAt = at
			case 1474:
				count++
				timerAt = at
				if len(packet.Payload) != 8 {
					t.Fatal("timeout must use current native two-u32 layout")
				}
				limit := binary.LittleEndian.Uint32(packet.Payload)
				start := int64(binary.LittleEndian.Uint32(packet.Payload[4:]))
				if limit != 3600 || start != w.activeDungeon.StartedAt.Unix() || start < before || start > time.Now().Unix() {
					t.Fatalf("stage%d replayed stale clock: limit=%d start=%d", stage, limit, start)
				}
				if start+int64(limit)-(start+5) != 3595 {
					t.Fatal("remaining time must decrease")
				}
			}
		}
		if count != 1 || !(noticeAt < timerAt && timerAt < endAt) {
			t.Fatal("entry clock count/order changed")
		}
		s := w.activeDungeon
		if !s.Definition.MoveMapEvenEnemy {
			t.Fatalf("stage%d lost source move-map-even-enemy rule", stage)
		}
		s.Loaded = true
		moves := 0
		for _, room := range s.Maze.Rooms {
			dx, dy := int(room.X)-int(s.Room.X), int(room.Y)-int(s.Room.Y)
			if dx*dx+dy*dy != 1 {
				continue
			}
			if _, err := s.Move(c, [2]byte{room.X, room.Y}); err != nil {
				t.Fatalf("stage%d source adjacent door refused: %v", stage, err)
			}
			moves++
		}
		if moves == 0 {
			t.Fatalf("stage%d had no tested source room edge", stage)
		}
	}
}

func TestIspinsSelectedFourMinuteSourceClock(t *testing.T) {
	c := catalog.LoadNativeDungeons(t, legion.IspinsStageDungeons[:]...)
	for stage := 0; stage < 4; stage++ {
		w := &worldSession{dungeons: &c, level: 140, ispins: &ispinsRun{confirmed: true}}
		for earlier := 0; earlier < stage; earlier++ {
			w.ispins.cleared[earlier] = true
		}
		confirm := make([]byte, 32)
		confirm[13] = 2
		binary.LittleEndian.PutUint16(confirm[17:], 11)
		if _, _, err := w.ispinsOperation(confirm); err != nil {
			t.Fatal(err)
		}
		request := make([]byte, 24)
		request[13] = 101
		binary.LittleEndian.PutUint32(request[17:], uint32(stage))
		packets, _, err := w.enterIspinsStage(request)
		if err != nil {
			t.Fatal(err)
		}
		found := false
		for _, p := range packets {
			if p.ID == 1474 {
				found = true
				start := binary.LittleEndian.Uint32(p.Payload[4:])
				if binary.LittleEndian.Uint32(p.Payload) != 240 || w.ispins.operationIndex != 11 || w.ispins.deadline.Unix() != int64(start)+240 {
					t.Fatal("selected operation did not produce04:00 clock", stage)
				}
			}
		}
		if !found {
			t.Fatal("no selected clock")
		}
	}
}

func TestIspinsReturnUsesNativeCachedMapMode(t *testing.T) {
	c := catalog.LoadNativeDungeons(t, legion.IspinsStageDungeons[0])
	s, err := dungeon.Select(c, protocol.DungeonSelection{ID: legion.IspinsStageDungeons[0], Party: 65535}, 140, nil)
	if err != nil {
		t.Fatal(err)
	}
	s.Loaded = true
	w := &worldSession{dungeons: &c, activeDungeon: s}
	leave, _ := hex.DecodeString(ispinsPhase2DoorMove)
	side, packets, err := w.moveDungeonRoom(leave)
	if err != nil || packets[1].Payload[31] != 1 {
		t.Fatal("fresh side-room mode", err)
	}
	side.Loaded = true
	w.activeDungeon = side
	// Live return at22:05:40, immediately before the actor reconstruction crash.
	backBody, _ := hex.DecodeString("0101510400003e0100000017d5000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000930000000000510400003e01000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000001000000050580022c01ffff00000000000000abecf5050000000000")
	back, packets, err := w.moveDungeonRoom(backBody)
	if err != nil {
		t.Fatal(err)
	}
	if back.Room.Map != 100006476 || len(back.LivingMonsters()) != len(s.LivingMonsters()) || packets[1].Payload[31] != 0 || len(packets[1].Payload) != 34 {
		t.Fatal("return recreated cached boss actors")
	}
	if back.Monsters[0].Entity != s.Monsters[0].Entity {
		t.Fatal("return replaced the owned boss")
	}
}
