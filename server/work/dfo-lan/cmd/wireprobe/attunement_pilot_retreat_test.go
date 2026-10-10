package main

import (
	"bytes"
	"dfolan/internal/database"
	"dfolan/internal/dungeon"
	"testing"
)

func TestAttunementPilotCanAbandonWithoutClearOrReward(t *testing.T) {
	s := &dungeon.Session{Definition: attunementPilotParsedDefinition(t), Loaded: true}
	if s.Completed() {
		t.Fatal("test dungeon already completed")
	}
	w := &worldSession{role: database.Character{ID: 5, WireID: 503, State: []byte("save-must-stay-unchanged")},
		activeDungeon: s, state: database.WorldState{Position: database.WorldPosition{Town: 38, Area: 2, X: 150, Y: 249}}}
	origin := w.state
	before := append([]byte(nil), w.role.State...)
	plan, err := w.leaveDungeon()
	if err != nil {
		t.Fatal(err)
	}
	if len(plan) != 4 || plan[0].ID != 42 || plan[0].Kind != 1 || !bytes.Equal(plan[0].Payload, []byte{1}) ||
		plan[1].ID != 3 || plan[2].ID != 23 || plan[3].ID != 24 {
		t.Fatalf("unfinished run did not take the ordinary abandon/town path: %+v", plan)
	}
	if !bytes.Equal(before, w.role.State) || w.state != origin || s.Completed() || w.completionSent || w.resultSent {
		t.Fatal("abandon changed the save, return origin, or clear/reward state")
	}
}
