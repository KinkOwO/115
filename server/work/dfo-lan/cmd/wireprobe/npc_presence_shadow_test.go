package main

import (
	"dfolan/internal/storage"
	"testing"
)

func TestNPCShadowIsDisabledAndDoesNotRequireStorage(t *testing.T) {
	t.Setenv("DFO_NPC_PRESENCE_DIAGNOSTICS", "")
	w := &worldSession{role: storage.Character{ID: 1}}
	if got := w.npcPresenceShadow(make([]byte, 16)); got != nil {
		t.Fatalf("disabled diagnostic ran: %v", got)
	}
}

func TestNPCShadowMalformedAndUnselectedRequestsDoNotReadStorage(t *testing.T) {
	t.Setenv("DFO_NPC_PRESENCE_DIAGNOSTICS", "1")
	for _, p := range [][]byte{nil, {33, 0, 71, 0}, make([]byte, 16)} {
		if got := (&worldSession{}).npcPresenceShadow(p); got != nil {
			t.Fatalf("invalid/unselected request: %v", got)
		}
	}
}
