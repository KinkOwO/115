package main

import (
	"dfolan/internal/database"
	"dfolan/internal/dungeon"
	"testing"
)

// N24 is a town arrival notification, even with zero placement rows. The r10
// live failure returned the native game state to town with no visible actors.
func TestDungeonLoadingDoesNotSendTownArrival(t *testing.T) {
	w := &worldSession{role: database.Character{WireID: 2}, activeDungeon: &dungeon.Session{}, state: database.WorldState{Position: database.WorldPosition{Town: 38, Area: 2, X: 150, Y: 249}}}
	for i := 0; i < 2; i++ {
		plan, err := w.finishDungeonLoading(make([]byte, 16))
		if err != nil {
			t.Fatal(err)
		}
		loaded := 0
		for _, p := range plan {
			if p.Kind == 0 && p.ID == 24 {
				t.Fatal("dungeon loading sent town arrival notification")
			}
			if p.Kind == 0 && p.ID == 30 {
				loaded++
			}
		}
		if loaded != 1 {
			t.Fatalf("want exactly one dungeon completion, got %d", loaded)
		}
	}
}

func TestSoloRosterAllowsTownReinvite(t *testing.T) {
	w := &worldSession{role: database.Character{WireID: 2}}
	p, err := w.soloPartyInfo(nil)
	if err != nil {
		t.Fatal(err)
	}
	if p[20] != 4 || p[69] != 1 {
		t.Fatalf("capacity/member count must remain separate: capacity=%d members=%d", p[20], p[69])
	}
}
