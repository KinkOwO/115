package main

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"testing"
)

func TestUnderfootStageCachedReturn(t *testing.T) {
	w := underfootWarpFixture(t)
	p, _ := hex.DecodeString(underfootWarpRequest)
	next, plan, err := w.moveDungeonRoom(p)
	if err != nil {
		t.Fatal(err)
	}
	body := plan[1].Payload
	if len(body) != 34 || body[31] != 0 || !bytes.Equal(body[13:31], p[132:150]) {
		t.Fatalf("cached return must preserve native room, got mode=%d bytes=%d", body[31], len(body))
	}
	if next.Room.Map != 100016035 || !next.ScriptWarps[100016040] {
		t.Fatal("lost stage")
	}
	if w.activeDungeon.Room.Map != 100016040 {
		t.Fatal("preflight mutated session")
	}
	// Actual return has no next-layer command. Re-entering the old room must
	// restore its existing triggers instead of reconstructing the intro.
	w.activeDungeon = next
	for _, xy := range [][2]byte{{0, 2}, {1, 2}} {
		w.activeDungeon.Loaded = true
		p[0], p[1] = xy[0], xy[1]
		clear(p[132:150])
		next, plan, err = w.moveDungeonRoom(p)
		if err != nil {
			t.Fatal(err)
		}
		if len(plan[1].Payload) != 34 || plan[1].Payload[31] != 0 {
			t.Fatal("revisit recreated room")
		}
		w.activeDungeon = next
	}
	// The legitimate source next-layer request still creates a fresh layer.
	w.activeDungeon.Loaded = true
	for _, route := range w.dungeons.SceneRoutes {
		if route.From != 100016035 {
			continue
		}
		p[10] = 1
		copy(p[132:150], route.Record[:])
		next, plan, err = w.moveDungeonRoom(p)
		if err != nil {
			t.Fatal(err)
		}
		body = plan[1].Payload
		if next.Room.Map != 100016041 || body[2] != 1 || body[31] != 1 || binary.LittleEndian.Uint32(body[32:36]) != 100016041 {
			t.Fatal("new layer not initialized")
		}
		t.Log("STAGE PASS: forced return and ordinary revisits use cached room; first next layer stays fresh; preflight unchanged")
		return
	}
	t.Fatal("missing source layer")
}
