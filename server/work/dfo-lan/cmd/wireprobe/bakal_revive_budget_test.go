package main

import (
	"context"
	"encoding/binary"
	"testing"
)

func TestBakalReviveBudgetBlocksBeforeSpendingCurrency(t *testing.T) {
	c, _ := bakalNativeClient(t)
	w, store, p := ceraReviveFixture(t, 1)
	w.bakal = c.worldState.bakal
	w.activeDungeon.RaidManaged = true
	coins, _ := w.bakal.Budget()
	for i := 0; i < coins; i++ {
		w.bakal.SpendCoinBudget()
	}
	if _, err := w.useCoinRevive(context.Background(), store, p, []byte{41, 1}, false); err == nil {
		t.Fatal("exhausted allowance revived")
	}
	if store.calls != 0 || len(store.grants) != 0 || !w.pilotDeath.Dead {
		t.Fatal("refusal spent token/CERA or changed death")
	}
}

func TestBakalReviveBudgetSuccessFailureAndReplay(t *testing.T) {
	c, _ := bakalNativeClient(t)
	w, store, p := ceraReviveFixture(t, 1)
	w.bakal = c.worldState.bakal
	w.activeDungeon.RaidManaged = true
	before, _ := w.bakal.Budget()
	plan, err := w.useCoinRevive(context.Background(), store, p, []byte{41, 3}, false)
	if err != nil || len(plan) != 4 || plan[3].ID != 2285 || binary.LittleEndian.Uint32(plan[3].Payload[9:]) != uint32(before-1) {
		t.Fatalf("revive budget plan: %v %+v", err, plan)
	}
	if after, _ := w.bakal.Budget(); after != before-1 {
		t.Fatal("successful revival not counted")
	}
	plan, err = w.useCoinRevive(context.Background(), store, p, []byte{41, 3}, false)
	if err != nil || len(plan) != 0 {
		t.Fatal("replayed revival was not ignored")
	}
	if after, _ := w.bakal.Budget(); after != before-1 {
		t.Fatal("replay spent second allowance")
	}
	if _, err = w.useCoinRevive(context.Background(), store, p, []byte{41, 4}, false); err == nil {
		t.Fatal("living player revived")
	}
	if after, _ := w.bakal.Budget(); after != before-1 {
		t.Fatal("failed revival spent allowance")
	}
}
