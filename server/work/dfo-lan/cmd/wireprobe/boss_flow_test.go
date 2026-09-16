package main

import (
	"bytes"
	"dfolan/internal/catalog"
	"dfolan/internal/dungeon"
	"dfolan/internal/game/protocol"
	"dfolan/internal/game/wire"
	"dfolan/internal/storage"
	"encoding/binary"
	"testing"
)

func TestBossCompletionPreflightAndReplay(t *testing.T) {
	run := &dungeon.Session{Loaded: true, Room: catalog.DungeonRoom{Boss: true}, Monsters: []protocol.DungeonMonster{{Entity: 4096, Rank: 3}}, Dead: map[uint16]bool{}}
	w := &worldSession{role: storage.Character{WireID: 3}, activeDungeon: run}
	p := make([]byte, 16)
	binary.LittleEndian.PutUint16(p, 3)
	binary.LittleEndian.PutUint16(p[2:], 4096)
	plan, e := w.bossCheck(p)
	if e != nil || len(plan) != 0 {
		t.Fatal("early boss check completed", e)
	}
	if _, e = run.ConfirmDeath(4096, 3, 3); e != nil {
		t.Fatal(e)
	}
	plan, e = w.completeDungeon()
	if e != nil || len(plan) != 2 || plan[0].Kind != 0 || plan[0].ID != 115 || plan[1].ID != 31 {
		t.Fatal("wrong completion family/order", e)
	}
	keys := make([]byte, wire.SessionKeyBytes)
	for i := range keys {
		keys[i] = byte(i%127 + 1)
	}
	ready, e := preparePackets(keys, plan)
	if e != nil {
		t.Fatal(e)
	}
	for _, p := range ready {
		plain, e := wire.DecryptPayload(keys, p.ID, p.Raw[16:])
		if e != nil || !bytes.Equal(plain[:len(p.Payload)], p.Payload) {
			t.Fatal("boss frame mismatch", e)
		}
	}
	w.completionSent = true
	if plan, e = w.bossCheck(p); e != nil || len(plan) != 0 {
		t.Fatal("completion replay reopened results", e)
	}
}
