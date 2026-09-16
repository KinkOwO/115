package main

import (
	"bytes"
	"dfolan/internal/dungeon"
	"dfolan/internal/game/wire"
	"dfolan/internal/storage"
	"testing"
)

func TestDungeonActorLifecyclePreflight(t *testing.T) {
	w := &worldSession{role: storage.Character{ID: 5, WireID: 503}, activeDungeon: &dungeon.Session{}, state: storage.WorldState{Position: storage.WorldPosition{Town: 38, Area: 2, X: 150, Y: 249}}}
	keys := make([]byte, wire.SessionKeyBytes)
	for i := range keys {
		keys[i] = byte(i%127 + 1)
	}
	enter, err := w.finishDungeonLoading(make([]byte, 16))
	if err != nil {
		t.Fatal(err)
	}
	leave, err := w.leaveDungeon()
	if err != nil {
		t.Fatal(err)
	}
	for i, plan := range [][]outboundPacket{enter, leave} {
		state := byte(1 - i)
		if len(plan) < 3 || plan[1].ID != 3 || plan[1].Kind != 0 || !bytes.Equal(plan[1].Payload, []byte{1, 247, 1, state}) {
			t.Fatalf("missing actor transition before scene: %+v", plan)
		}
		prepared, err := preparePackets(keys, plan)
		if err != nil {
			t.Fatal(err)
		}
		for _, p := range prepared {
			decoded, err := wire.DecryptPayload(keys, p.ID, p.Raw[16:])
			if err != nil || len(decoded) < len(p.Payload) || !bytes.Equal(decoded[:len(p.Payload)], p.Payload) {
				t.Fatalf("lifecycle preflight %s: %v", p.Name, err)
			}
		}
	}
	for _, p := range [][]byte{nil, make([]byte, 8), append([]byte{1}, make([]byte, 15)...)} {
		if _, err = w.finishDungeonLoading(p); err == nil {
			t.Fatal("accepted malformed loading request")
		}
	}
}
