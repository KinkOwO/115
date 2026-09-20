package main

import (
	"dfolan/internal/game/protocol"
	"dfolan/internal/game/wire"
	"testing"
)

func TestDungeonRequestsReachVerifiedHandlers(t *testing.T) {
	keys := make([]byte, wire.SessionKeyBytes)
	for i := range keys {
		keys[i] = byte(i%127 + 1)
	}
	for _, id := range []uint16{6, 15, 16, 28, 29, 37, 38, 39, 40, 42, 43, 45, 46, 69, 70, 71, 72, 117, 132, 191, 451, 637, 2377} {
		if !observedGameRequest(id) {
			t.Fatalf("implemented command%d never decrypted", id)
		}
	}
	if !dungeonRequest(40) {
		t.Fatal("ordinary player death never reaches dungeon handler")
	}
	if observedGameRequest(2127) {
		t.Fatal("retaining process scan")
	}
	p := make([]byte, 21)
	p[0] = 9
	p[1] = 16
	c, e := wire.EncryptPayload(keys, 43, p)
	if e != nil {
		t.Fatal(e)
	}
	decoded, e := wire.DecryptPayload(keys, 43, c)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = protocol.DecodePickup(decoded); e != nil {
		t.Fatal("pickup decoder refuses actual cipher padding", len(decoded), e)
	}
}
