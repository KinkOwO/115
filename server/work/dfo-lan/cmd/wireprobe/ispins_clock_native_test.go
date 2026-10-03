package main

import (
	"dfolan/internal/catalog"
	"dfolan/internal/legion"
	"encoding/binary"
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
