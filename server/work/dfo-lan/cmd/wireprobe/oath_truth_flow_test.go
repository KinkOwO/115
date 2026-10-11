package main

import (
	"context"
	"dfolan/internal/catalog"
	"dfolan/internal/database"
	"dfolan/internal/dungeon"
	"dfolan/internal/inventory"
	"encoding/json"
	"os"
	"testing"
)

func TestTruthWireFlowRefreshAndSceneRecovery(t *testing.T) {
	w, f := oathSelectionFixture(t)
	ctx := context.Background()
	raw, err := os.ReadFile("../../internal/catalog/testdata/oath_truth.cos")
	if err != nil {
		t.Fatal(err)
	}
	r, err := catalog.ParseOathTruth(string(raw))
	if err != nil {
		t.Fatal(err)
	}
	r.Source = w.role.ConfigVersion
	w.items.Truth = r
	w.items.Catalog.Source.Checksum = r.Source
	if _, err = w.store.LoadVault(ctx, w.account, w.role.ID, 8, w.role.ConfigVersion); err != nil {
		t.Fatal(err)
	}
	_, _, err = w.store.CommitVaultMove(ctx, w.account, w.role.ID, func(role database.Character, v database.VaultState) (json.RawMessage, json.RawMessage, error) {
		bag, e := inventory.ReadBag(role.State)
		if e != nil {
			return nil, nil, e
		}
		bag.Items = []inventory.BagItem{{Slot: 100, Template: 10401346, Amount: 100}, {Slot: 101, Template: 10421371, Amount: 40}}
		bag.Gold = 14274
		state, e := inventory.SaveBag(role.State, bag)
		return state, v.Items, e
	})
	if err != nil {
		t.Fatal(err)
	}
	w.role.WireID = 2
	body := make([]byte, 16)
	body[0] = 1
	// Captured 2026-10-11 01:51:00, character Deemo10110, UI 100+40.
	body[1] = 1
	plan, err := w.oathTruthPackets(ctx, 2420, body)
	if err != nil {
		t.Fatal(err)
	}
	if w.role.WireID != 2 || plan[len(plan)-1].ID != 2420 || len(plan[len(plan)-1].Payload) != 10 || plan[len(plan)-1].Payload[6] != 1 {
		t.Fatal("actor identity or ack")
	}
	bag, err := inventory.ReadBag(w.role.State)
	if err != nil || bag.Gold != 14274 || len(bag.Items) != 0 {
		t.Fatal("live material request charged gold or kept source materials", err)
	}
	infoAt, ackAt := -1, -1
	for i, p := range plan {
		if p.ID == 2839 {
			infoAt = i
			if p.Payload[4] != 1 || p.Payload[5] != 0 || p.Payload[6] != 0 {
				t.Fatal("first truth state")
			}
		}
		if p.ID == 2420 {
			ackAt = i
		}
	}
	if infoAt < 0 || infoAt >= ackAt {
		t.Fatal("state must precede animation callback")
	}
	selectBody := make([]byte, 8)
	selectBody[0] = 1
	selectBody[1] = 1
	if _, err = w.oathTruthPackets(ctx, 2421, selectBody); err != nil {
		t.Fatal(err)
	}
	snapshot, err := w.dungeonOathSelectionPacket(ctx)
	if err != nil || snapshot.Payload[4] != 1 || snapshot.Payload[6] != 1 {
		t.Fatal("scene rebuild erased option", err)
	}
	refresh := w.oathDisplayRefresh(ctx, "refresh")
	if len(refresh) != 1 || refresh[0].Payload[6] != 1 {
		t.Fatal("wear/return restore erased option")
	}
	w.activeDungeon = &dungeon.Session{}
	failed, err := w.oathTruthPackets(ctx, 2420, body)
	if err == nil || len(failed) != 1 || failed[0].Payload[0] != 0 {
		t.Fatal("dungeon mutation or missing failure ack")
	}
	w.activeDungeon = nil
	reopened, err := f.Reopen(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	truth, err := reopened.OathTruth(ctx, w.account, w.role.ID)
	if err != nil || truth.Options[1] != 1 {
		t.Fatal("persistent selection", err)
	}

}

func TestTruthRequestsRemainObservableAfterSamplingLimit(t *testing.T) {
	for _, id := range []uint16{2420, 2421} {
		seen := map[uint16]int{id: BodySampleLimit}
		for i := 0; i < 16; i++ {
			if !observedGameRequest(id) || !retainRequestBody(id, seen) {
				t.Fatal("implemented truth command lost its diagnostic body", id, i)
			}
		}
	}
}
