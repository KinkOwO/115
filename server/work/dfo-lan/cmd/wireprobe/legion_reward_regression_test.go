package main

import (
	"context"
	"dfolan/internal/inventory"
	"dfolan/internal/workflow"
	"encoding/json"
	"reflect"
	"testing"
)

func TestIspinsSQLiteFullBagRecoveryKeepsFrozenRewards(t *testing.T) {
	w, fixture := ispinsSQLiteRewardFixture(t)
	// Limit capacity to one stack slot: the second award must roll back the first.
	w.loot.BagRules.Slots["[etc]"] = [2]uint16{68, 68}
	before := w.ispins.cleared
	if plan, err := w.completeIspinsStage(); err == nil || len(plan) != 0 {
		t.Fatal("partial reward accepted", err)
	}
	if w.ispins.cleared != before {
		t.Fatal("failed grant advanced phase")
	}
	bag, err := inventory.ReadBag(w.role.State)
	if err != nil || len(bag.Items) != 0 {
		t.Fatal("partial grant persisted", bag, err)
	}
	// The reward was frozen before the failure. A changed source must not reroll it.
	op := w.dungeons.IspinsOperations[11]
	op.Rewards[0].Amount = 99
	w.dungeons.IspinsOperations[11] = op
	w.loot.BagRules.Slots["[etc]"] = [2]uint16{68, 69}
	// Drop the connection and in-memory role before recovery, as on restart.
	fixture.Storage().Close()
	reopened, err := fixture.Reopen(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(reopened.Close)
	roles, err := reopened.Characters(context.Background(), w.role.AccountID)
	if err != nil || len(roles) != 1 {
		t.Fatal("reload frozen owner", roles, err)
	}
	w.store, w.role = reopened, roles[0]
	svc := &workflow.LegionRewardService{Store: w.store, Awarder: &inventory.Awarder{Catalog: w.loot.Catalog, Rules: w.loot.BagRules}}
	saved, err := svc.Recover(context.Background(), w.role)
	if err != nil {
		t.Fatal(err)
	}
	w.role = saved
	bag, err = inventory.ReadBag(saved.State)
	if err != nil || len(bag.Items) != 2 || bag.Items[0].Amount != 8 || bag.Items[1].Amount != 3 {
		t.Fatal("recovery changed frozen awards", bag, err)
	}
	snapshot := append(json.RawMessage(nil), saved.State...)
	for i := 0; i < 2; i++ {
		if _, err = w.completeIspinsStage(); err != nil {
			t.Fatal(err)
		}
	}
	var a, b any
	json.Unmarshal(snapshot, &a)
	json.Unmarshal(w.role.State, &b)
	if !reflect.DeepEqual(a, b) {
		t.Fatal("replayed clear paid twice")
	}
}

func TestIspinsSQLiteRejectsMissingRulesWithoutCompletion(t *testing.T) {
	w := ispinsSQLiteRewardWorld(t)
	w.dungeons.LegionContents = nil
	old := append(json.RawMessage(nil), w.role.State...)
	if plan, err := w.completeIspinsStage(); err == nil || len(plan) != 0 {
		t.Fatal("missing source accepted")
	}
	if w.ispins.cleared[0] || string(w.role.State) != string(old) {
		t.Fatal("failure changed owner or stage")
	}
}

func TestIspinsSettlementNeverReplaysCapturedInventory(t *testing.T) {
	for stage := 0; stage < 4; stage++ {
		for _, rows := range [][]ispinsAuxPacket{ispinsAuxPre31[stage], ispinsAuxPre2252[stage], ispinsAuxPre2253[stage], ispinsAuxPre115[stage]} {
			for _, p := range appendIspinsAux(nil, rows) {
				if p.ID == 14 {
					t.Fatal("another player's inventory replayed")
				}
			}
		}
	}
}
