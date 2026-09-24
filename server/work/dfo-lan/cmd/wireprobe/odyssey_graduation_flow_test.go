package main

import (
	"dfolan/internal/dungeon"
	"testing"
)

func TestGraduationOnlyAfterTownReturn(t *testing.T) {
	for _, name := range []string{"dungeon_clear_enabled", "dungeon_clear_reward", "settlement_focus_ack", "settlement_exit_ack", "dungeon_session_started"} {
		if returnedToTown([]outboundPacket{{Name: name}}) {
			t.Fatalf("graduation during %s", name)
		}
	}
	if !returnedToTown([]outboundPacket{{Name: "dungeon_return_users"}}) {
		t.Fatal("town return missing")
	}
	w := &worldSession{activeDungeon: &dungeon.Session{}}
	if plan, err := w.graduateOdysseyAtTown(); err != nil || len(plan) != 0 {
		t.Fatalf("graduation inside run: %v", err)
	}
	w.activeDungeon = nil
	w.selectingDungeon = true
	if plan, err := w.graduateOdysseyAtTown(); err != nil || len(plan) != 0 {
		t.Fatalf("graduation at dungeon selection: %v", err)
	}
}
