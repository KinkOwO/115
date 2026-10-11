package main

import (
	"dfolan/internal/character"
	"dfolan/internal/database"
	"dfolan/internal/dungeon"
	"dfolan/internal/game/protocol"
	"encoding/binary"
	"reflect"
	"testing"
)

func TestIspinsCompletedRunRejectsSelectionWithoutPanic(t *testing.T) {
	for _, action := range []byte{1, 2, 4} {
		w := &worldSession{ispins: &ispinsRun{stage: 3, cleared: [4]bool{true, true, true, true}, confirmed: true, operationIndex: 11}}
		before := *w.ispins
		p := make([]byte, 32)
		p[13] = action
		p[17] = 11
		if plan, _, err := w.ispinsOperation(p); err == nil || len(plan) != 0 {
			t.Fatalf("action %d: plan=%v err=%v", action, plan, err)
		}
		if !reflect.DeepEqual(before, *w.ispins) {
			t.Fatal("completed run changed")
		}
	}
}

func TestIspinsSettlementProjectionFailureRetainsProgress(t *testing.T) {
	for _, stage := range []int{0, 3} {
		run := &dungeon.Session{ArenaBoss: true, Loaded: true, Dead: map[uint16]bool{}, Monsters: []protocol.DungeonMonster{{Entity: 4096, Rank: 3, Team: 100}}}
		if _, err := run.ConfirmDeath(4096, 7, 7); err != nil || !run.Completed() {
			t.Fatal(err)
		}
		w := &worldSession{activeDungeon: run, ispins: &ispinsRun{stage: stage}, role: database.Character{WireID: 7, State: []byte("{")}, characters: &character.Service{}}
		before := *w.ispins
		if plan, err := w.completeIspinsStage(); err == nil || len(plan) != 0 {
			t.Fatal("broken projection accepted", plan, err)
		}
		if !reflect.DeepEqual(before, *w.ispins) || w.completionSent {
			t.Fatal("failed projection consumed clear")
		}
	}
}

func TestIspinsStaleRewardEndCannotReopenPreviousStage(t *testing.T) {
	w := &worldSession{ispins: &ispinsRun{stage: 1, cleared: [4]bool{true, true}}}
	before := *w.ispins
	p := make([]byte, 32)
	binary.LittleEndian.PutUint32(p[13:], 101)
	if plan, _, err := w.ispinsRewardEnd(p); err == nil || len(plan) != 0 {
		t.Fatal("stale stage result accepted", plan, err)
	}
	if !reflect.DeepEqual(before, *w.ispins) {
		t.Fatal("stale result mutated progress")
	}
	p[17], p[21] = 1, 3
	if plan, _, err := w.ispinsRewardEnd(p); err != nil || len(plan) != 2 {
		t.Fatal("current stage rejected", plan, err)
	}
}

func TestIspinsSettlementNeverSendsCaptureInventory(t *testing.T) {
	for stage := 0; stage < 4; stage++ {
		run := &dungeon.Session{ArenaBoss: true, Loaded: true, Dead: map[uint16]bool{}, Monsters: []protocol.DungeonMonster{{Entity: 4096, Rank: 3, Team: 100}}}
		if _, err := run.ConfirmDeath(4096, 7, 7); err != nil {
			t.Fatal(err)
		}
		w := &worldSession{activeDungeon: run, ispins: &ispinsRun{stage: stage}, role: database.Character{WireID: 7}}
		plan, err := w.completeIspinsStage()
		if err != nil {
			t.Fatal(err)
		}
		for _, p := range plan {
			if p.ID == 14 || p.Name == "ispins_reward_item_granted" {
				t.Fatalf("stage%d leaked capture inventory", stage)
			}
		}
	}
}
