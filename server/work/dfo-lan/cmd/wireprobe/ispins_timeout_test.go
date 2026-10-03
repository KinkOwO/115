package main

import (
	"dfolan/internal/dungeon"
	"dfolan/internal/game/protocol"
	"dfolan/internal/storage"
	"testing"
	"time"
)

func TestIspinsTimeLimitFailsWithoutClearOrRewards(t *testing.T) {
	deadline := time.Now().Add(-time.Hour)
	w := &worldSession{
		role:          storage.Character{ID: 7, WireID: 7},
		state:         storage.WorldState{Position: storage.WorldPosition{Town: 146, Area: 0, X: 700, Y: 300}},
		ispins:        &ispinsRun{stage: 1, confirmed: true, cleared: [4]bool{true, false, false, false}, deadline: deadline},
		activeDungeon: &dungeon.Session{Loaded: true, ArenaBoss: true, RunID: "timed-run", Dead: map[uint16]bool{}, Monsters: []protocol.DungeonMonster{{Entity: 4096, Rank: 3, Team: 100}}},
	}
	if plan, err := w.ispinsTimeout(deadline.Add(-time.Nanosecond)); err != nil || len(plan) != 0 || w.activeDungeon == nil {
		t.Fatal("timeout fired before deadline", err)
	}
	plan, err := w.ispinsTimeout(deadline)
	if err != nil {
		t.Fatal(err)
	}
	if len(plan) < 4 || plan[0].ID != 33 || plan[0].Kind != 0 || string(plan[0].Payload) != string(protocol.DungeonFailClear(100)) {
		t.Fatalf("missing native failure and town return: %+v", plan)
	}
	for _, p := range plan {
		if p.Kind == 1 || p.ID == 2252 || p.ID == 2253 || p.ID == 31 || p.ID == 14 {
			t.Fatalf("timeout fabricated request/rewards: %+v", p)
		}
	}
	if w.activeDungeon != nil || w.ispins.confirmed || !w.ispins.deadline.IsZero() || w.ispins.cleared != [4]bool{true, false, false, false} {
		t.Fatal("timeout changed completed stages or left combat active")
	}
	if plan, err := w.ispinsTimeout(deadline.Add(time.Second)); err != nil || len(plan) != 0 {
		t.Fatal("timeout repeated", err)
	}
}

func TestIspinsLateBossCannotBypassTimeLimit(t *testing.T) {
	w := &worldSession{role: storage.Character{ID: 7, WireID: 7},
		state:         storage.WorldState{Position: storage.WorldPosition{Town: 146, Area: 0, X: 700, Y: 300}},
		ispins:        &ispinsRun{confirmed: true, deadline: time.Now().Add(-time.Second)},
		activeDungeon: &dungeon.Session{Loaded: true, ArenaBoss: true, Dead: map[uint16]bool{}, Monsters: []protocol.DungeonMonster{{Entity: 4096, Rank: 3, Team: 100}}},
	}
	if _, err := w.activeDungeon.ConfirmDeath(4096, 7, 7); err != nil {
		t.Fatal(err)
	}
	plan, err := w.completeIspinsStage()
	if err != nil || len(plan) == 0 || plan[0].ID != 33 || w.activeDungeon != nil || w.ispins.cleared != [4]bool{} {
		t.Fatal("late completion bypassed failure", err)
	}
	accepted := &worldSession{ispins: &ispinsRun{deadline: time.Now().Add(-time.Second)}, activeDungeon: &dungeon.Session{}, completionSent: true}
	if plan, err := accepted.ispinsTimeout(time.Now()); err != nil || len(plan) != 0 || accepted.activeDungeon == nil {
		t.Fatal("accepted completion was revoked", err)
	}
}
