package main

import (
	"dfolan/internal/dungeon"
	"testing"
)

func TestDeathTimerCannotEndLaterAttempt(t *testing.T) {
	w := &worldSession{activeDungeon: &dungeon.Session{RunID: "second-run"}, pilotDeath: &odysseyDeath{Run: "second-run", Sequence: 2, Dead: true}}
	if w.deathAttemptActive("first-run", 2) {
		t.Fatal("timer from previous run still active")
	}
	if w.deathAttemptActive("second-run", 1) {
		t.Fatal("timer from death before revive still active")
	}
	if !w.deathAttemptActive("second-run", 2) {
		t.Fatal("owned death did not arm timeout")
	}
	w.pilotDeath.Dead = false
	if w.deathAttemptActive("second-run", 2) {
		t.Fatal("revive did not cancel timeout")
	}
	w.pilotDeath.Dead = true
	w.activeDungeon = nil
	if w.deathAttemptActive("second-run", 2) {
		t.Fatal("town return did not cancel timeout")
	}
}
