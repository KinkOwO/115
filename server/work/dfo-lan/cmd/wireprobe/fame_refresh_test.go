package main

import (
	"context"
	"dfolan/internal/character"
	"dfolan/internal/database"
	"dfolan/internal/game/protocol"
	"dfolan/internal/inventory"
	"dfolan/internal/savecontract"
	"encoding/binary"
	"encoding/json"
	"testing"
)

func TestFameForcedRefreshKeepsStoredHighest(t *testing.T) {
	ctx := context.Background()
	fixture, err := database.OpenTestFixture(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer fixture.Close()
	store := fixture.Storage()
	if err := store.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	account, err := store.DevelopmentAccount(ctx, "fame-refresh-fixture")
	if err != nil {
		t.Fatal(err)
	}
	w := fameRefreshSession()
	w.role, err = store.CreateCharacter(ctx, database.Character{AccountID: account, Name: "FameFixture", ConfigVersion: savecontract.Identity(), State: w.role.State, Request: []byte{0}}, 24)
	if err != nil {
		t.Fatal(err)
	}
	w.role.WireID = 2
	w.store, w.characters.Store = store, store
	if _, err := store.RecordCharacterFame(ctx, account, w.role.ID, 900); err != nil {
		t.Fatal(err)
	}
	plan := w.appendFameAfterMove(nil, nil, protocol.ItemMoveRequest{SourceList: 1, DestinationList: 3}, json.RawMessage(`{}`))
	if len(plan) != 1 || binary.LittleEndian.Uint32(plan[0].Payload[2:]) != 0 || binary.LittleEndian.Uint32(plan[0].Payload[6:]) != 900 {
		t.Fatal("forced correction lowered stored highest", plan)
	}
	reopened, err := fixture.Reopen(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	highest, err := reopened.RecordCharacterFame(ctx, account, w.role.ID, 0)
	if err != nil || highest != 900 {
		t.Fatal("highest fame did not persist", highest, err)
	}
}

func fameRefreshSession() *worldSession {
	w := &worldSession{characters: &character.Service{Equipment: &inventory.EquipmentCatalog{}}, fameInitialized: true}
	w.role.ID, w.role.WireID = 1, 2
	w.role.State = json.RawMessage(`{"level":115,"worn":[],"avatar_revision":2}`)
	return w
}

func TestFameSameTotalWearChangeCorrectsAfterRestoration(t *testing.T) {
	w := fameRefreshSession()
	before := json.RawMessage(`{"level":115,"worn":[],"avatar_revision":1}`)
	plan := []outboundPacket{{"clone_actor_restored", 0, 2, []byte{1}}, {"oath_system_info_after_wear", 0, 2839, []byte{2}}}
	plan = w.appendFameAfterMove(plan, nil, protocol.ItemMoveRequest{SourceList: 1, DestinationList: 3}, before)
	if len(plan) != 3 || plan[2].ID != 2257 || plan[2].Name != "character_fame_updated" {
		t.Fatal("missing final authority correction", plan)
	}
	p := plan[2].Payload
	if len(p) != 10 || binary.LittleEndian.Uint16(p) != 2 || binary.LittleEndian.Uint32(p[2:]) != 0 || binary.LittleEndian.Uint32(p[6:]) != 0 {
		t.Fatal("wrong fame body", p)
	}
	if plan = w.appendFameUpdate(nil, nil); len(plan) != 0 {
		t.Fatal("unrelated unchanged total must stay suppressed")
	}
}

func TestFameMoveRefreshIgnoresBagMovesAndUnchangedState(t *testing.T) {
	w := fameRefreshSession()
	before := json.RawMessage(`{"level":115,"worn":[],"avatar_revision":1}`)
	for _, row := range []struct {
		name     string
		request  protocol.ItemMoveRequest
		previous json.RawMessage
	}{
		{"bag only", protocol.ItemMoveRequest{SourceList: 1, DestinationList: 1}, before},
		{"no committed change", protocol.ItemMoveRequest{SourceList: 1, DestinationList: 3}, w.role.State},
	} {
		t.Run(row.name, func(t *testing.T) {
			p := w.appendFameAfterMove(nil, nil, row.request, row.previous)
			if len(p) != 0 {
				t.Fatal("unnecessary fame packet", p)
			}
		})
	}
}

func TestFameCreatureActorRebuildCorrectsUnchangedTotal(t *testing.T) {
	w := fameRefreshSession()
	plan := w.appendFameAfterMove(nil, nil, protocol.ItemMoveRequest{SourceList: 7, SourceSlot: 26, DestinationList: 7}, json.RawMessage(`{}`))
	if len(plan) != 1 || plan[0].ID != 2257 {
		t.Fatal("creature actor rebuild omitted authority correction", plan)
	}
}

func TestFameFailedCalculationPreservesSentCache(t *testing.T) {
	w := fameRefreshSession()
	w.lastFame = 900
	w.role.State = json.RawMessage(`invalid`)
	var events []map[string]any
	original := []outboundPacket{{"committed_inventory", 0, 14, nil}}
	got := w.appendFameAfterMove(original, func(v map[string]any) { events = append(events, v) }, protocol.ItemMoveRequest{SourceList: 1, DestinationList: 3}, json.RawMessage(`{}`))
	if len(got) != 1 || w.lastFame != 900 || !w.fameInitialized || len(events) != 1 || events[0]["kind"] != "fame_refresh_failed" {
		t.Fatal("failed refresh altered authority", got, events)
	}
}
