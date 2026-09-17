package main

import (
	"bytes"
	"dfolan/internal/catalog"
	"dfolan/internal/dungeon"
	"dfolan/internal/game/protocol"
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
