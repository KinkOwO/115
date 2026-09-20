package main

import (
	"bytes"
	"dfolan/internal/catalog"
	"dfolan/internal/dungeon"
	"dfolan/internal/game/protocol"
	"encoding/binary"
	"encoding/hex"
	"testing"
)

const underfootWarpRequest = "0102170200006301000000b2cc000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000ec015833000017020000630100000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000040100000005050a02ec00ffff000000000000004cf4f5050000000000"

func underfootWarpFixture(t *testing.T) *worldSession {
	t.Helper()
	c, e := catalog.LoadDungeons("../../configs/dungeons.odyssey-scenes-candidate.json")
	if e != nil {
		t.Fatal(e)
	}
	s, e := dungeon.Select(c, protocol.DungeonSelection{ID: 100004940, Difficulty: 2, Party: 65535}, 40, nil)
	if e != nil {
		t.Fatal(e)
	}
	for _, xy := range [][2]byte{{2, 1}, {2, 2}, {1, 2}, {0, 2}, {0, 3}, {1, 3}, {2, 3}, {3, 3}} {
		s.Loaded = true
		for _, m := range s.Monsters {
			s.Dead[m.Entity] = true
		}
		s, e = s.Move(c, xy)
		if e != nil {
			t.Fatal(xy, e)
		}
	}
	s.Loaded = true
	return &worldSession{dungeons: &c, activeDungeon: s}
}

func TestUnderfootCapturedForcedWarp(t *testing.T) {
	w := underfootWarpFixture(t)
	p, _ := hex.DecodeString(underfootWarpRequest)
	next, plan, e := w.moveDungeonRoom(p)
	if e != nil {
		t.Fatal("captured original forced return rejected:", e)
	}
	if next.Room.Map != 100016035 || len(plan) != 2 || !bytes.Equal(plan[1].Payload[13:31], p[132:150]) {
		t.Fatal("destination or original transition lost")
	}
	if w.activeDungeon.Room.Map != 100016040 {
		t.Fatal("preflight mutated live room")
	}
	t.Log("MODIFIED: source forced warp 100016040 -> 100016035; 18-byte landing preserved; no fabricated monster deaths")
}

func TestUnderfootCinematicCipherPadding(t *testing.T) {
	p, _ := hex.DecodeString("20070000000000000000000000000000")
	id, e := protocol.DecodeCinematicSkip(p)
	if e != nil || id != 1824 {
		t.Fatal(id, e)
	}
	p[15] = 1
	if _, e = protocol.DecodeCinematicSkip(p); e == nil {
		t.Fatal("nonzero tail accepted")
	}
}

func TestMoveScriptFallbackToAdjacentMove(t *testing.T) {
	c, e := catalog.LoadDungeons("../../configs/dungeons.odyssey-release.json")
	if e != nil {
		t.Fatal(e)
	}
	s, e := dungeon.Select(c, protocol.DungeonSelection{ID: 100004962, Difficulty: 2, Party: 65535}, 87, nil)
	if e != nil {
		t.Fatal(e)
	}
	s.Loaded = true
	for _, m := range s.Monsters {
		s.Dead[m.Entity] = true
	}
	w := &worldSession{dungeons: &c, activeDungeon: s}

	// Craft CMD45 room transition from (0,1) to adjacent room (1,1) with Record[0] == 1 (script warp flag)
	req := make([]byte, 160)
	req[0] = 1   // target X = 1
	req[1] = 1   // target Y = 1
	req[132] = 1 // Record[0] = 1
	binary.LittleEndian.PutUint32(req[151:155], 100004962)

	next, plan, err := w.moveDungeonRoom(req)
	if err != nil {
		t.Fatalf("expected MoveScript fallback to adjacent Move to succeed, got error: %v", err)
	}
	if next == nil || next.Room.X != 1 || next.Room.Y != 1 {
		t.Fatalf("expected next room to be (1,1), got: %+v", next)
	}
	if len(plan) != 2 || plan[0].Name != "dungeon_move_ack" {
		t.Fatalf("unexpected plan: %+v", plan)
	}
}

func TestInteractDoorOrdinaryAndSirocco(t *testing.T) {
	c, e := catalog.LoadDungeons("../../configs/dungeons.odyssey-scenes-release.json")
	if e != nil {
		t.Fatal(e)
	}
	s, e := dungeon.Select(c, protocol.DungeonSelection{ID: 100004961, Difficulty: 2, Party: 65535}, 80, nil)
	if e != nil {
		t.Fatal(e)
	}
	s.Loaded = true
	w := &worldSession{dungeons: &c, activeDungeon: s}

	// 1. Ordinary room: interactDoor returns door_ack
	next, plan, err := w.interactDoor(nil)
	if err != nil {
		t.Fatalf("interactDoor failed on ordinary room: %v", err)
	}
	if next != nil {
		t.Fatal("expected next to be nil for ordinary room")
	}
	if len(plan) != 1 || plan[0].Name != "door_ack" || plan[0].ID != 38 {
		t.Fatalf("unexpected plan for ordinary door: %+v", plan)
	}

	// 2. Sirocco cutscene room 100016294 at (3,1): interactDoor synthesizes transition to boss room (4,1)
	s.Room = catalog.DungeonRoom{X: 3, Y: 1, Map: 100016294}
	next, plan, err = w.interactDoor(nil)
	if err != nil {
		t.Fatalf("interactDoor failed on Sirocco room 100016294: %v", err)
	}
	if next == nil || next.Room.X != 4 || next.Room.Y != 1 || next.Room.Map != 100016295 {
		t.Fatalf("expected next room to be boss room (4,1,100016295), got: %+v", next)
	}
	if len(plan) < 2 || plan[0].Name != "door_ack" {
		t.Fatalf("unexpected plan for Sirocco door: %+v", plan)
	}
}
