package main

import (
	"bytes"
	"dfolan/internal/catalog"
	"dfolan/internal/dungeon"
	"dfolan/internal/game/protocol"
	"dfolan/internal/testfixture"
	"encoding/binary"
	"encoding/hex"
	"testing"
)

const underfootWarpRequest = "0102170200006301000000b2cc000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000ec015833000017020000630100000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000040100000005050a02ec00ffff000000000000004cf4f5050000000000"

func underfootWarpFixture(t *testing.T) *worldSession {
	t.Helper()
	c, e := catalog.LoadDungeons(testfixture.DungeonPath(t, "dungeons.odyssey-scenes-release.json"))
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
	c, e := catalog.LoadDungeons(testfixture.DungeonPath(t, "dungeons.odyssey-release.json"))
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

// C2S38 是 USE_SKILL（官服抓包 2026-10-05），服务端不响应也不换房；
// 原来的 TestInteractDoorOrdinaryAndSirocco 锁定的「点门只回 ack」是被误认
// 出来的旁路，随 interactDoor 一并移除。Boss 门开后换房走客户端原生 CMD45
// （见 sirocco_boss_door_test.go）。
