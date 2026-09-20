package main

import (
	"context"
	"dfolan/internal/catalog"
	"dfolan/internal/catalog/pvf"
	"dfolan/internal/dungeon"
	"dfolan/internal/inventory"
	"dfolan/internal/loot"
	"dfolan/internal/storage"
	"encoding/binary"
	"encoding/json"
	"testing"
)

type lifeTokenStore struct {
	role  storage.Character
	keys  map[string]bool
	calls int
}

func (s *lifeTokenStore) CommitCharacterEvent(_ context.Context, _ int64, _ int64, _, key, _ string, apply func(storage.Character) (json.RawMessage, json.RawMessage, error)) (storage.Character, bool, error) {
	if s.keys == nil {
		s.keys = map[string]bool{}
	}
	if s.keys[key] {
		return s.role, false, nil
	}
	s.calls++
	state, _, err := apply(s.role)
	if err != nil {
		return s.role, false, err
	}
	s.role.State = state
	s.keys[key] = true
	return s.role, true, nil
}

func TestLifeTokenReviveConsumesOneTokenAndRefreshesBag(t *testing.T) {
	const source = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	state, err := inventory.SaveBag(json.RawMessage(`{"level":1}`), inventory.Bag{Version: "ordinary-bag-v1", Coin: 2})
	if err != nil {
		t.Fatal(err)
	}
	role := storage.Character{ID: 7, AccountID: 11, WireID: 9, ConfigVersion: source, State: state}
	store := &lifeTokenStore{role: role}
	w := &worldSession{
		role:          role,
		loot:          &loot.Service{Catalog: catalog.LootCatalog{Source: pvf.ArchiveSnapshot{Checksum: source}}},
		dungeons:      &catalog.DungeonCatalog{Maps: map[uint32]catalog.ScriptRecord{1: {}}},
		activeDungeon: &dungeon.Session{RunID: "run-1", Loaded: true, Room: catalog.DungeonRoom{Map: 1}},
		pilotDeath:    &odysseyDeath{Run: "run-1", Sequence: 3, Dead: true},
	}
	request := make([]byte, 8)
	binary.LittleEndian.PutUint16(request, role.WireID)

	plan, err := w.lifeTokenRevive(context.Background(), store, request, []byte{41, 3})
	if err != nil {
		t.Fatal(err)
	}
	if len(plan) != 3 || plan[0].ID != 41 || plan[1].ID != 32 || plan[2].ID != 14 {
		t.Fatalf("unexpected revive packets: %+v", plan)
	}
	if got := plan[0].Payload; len(got) != 3 || got[0] != 1 || binary.LittleEndian.Uint16(got[1:]) != role.WireID {
		t.Fatalf("unexpected revive ack: %x", plan[0].Payload)
	}
	if plan[1].Payload[2] != 1 {
		t.Fatalf("expected full revive state, got %x", plan[1].Payload)
	}
	bag, err := inventory.ReadBag(w.role.State)
	if err != nil || bag.Coin != 1 {
		t.Fatalf("coin was not consumed: coin=%d err=%v", bag.Coin, err)
	}
	if w.pilotDeath.Dead {
		t.Fatal("actor remains dead after successful revive")
	}
	if store.calls != 1 {
		t.Fatalf("expected one transaction, got %d", store.calls)
	}

	// A retransmitted CMD41 is acknowledged by the in-memory idempotency guard
	// without consuming another token or changing the revived state.
	plan, err = w.lifeTokenRevive(context.Background(), store, request, []byte{41, 3})
	if err != nil || len(plan) != 0 || store.calls != 1 {
		t.Fatalf("replay was not idempotent: packets=%d calls=%d err=%v", len(plan), store.calls, err)
	}

	if _, err = w.lifeTokenRevive(context.Background(), store, request, []byte{41, 4}); err == nil {
		t.Fatal("accepted a second revive after the actor was restored")
	}
}

func TestLifeTokenReviveRejectsCoinForbiddenMap(t *testing.T) {
	const source = "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
	state, err := inventory.SaveBag(json.RawMessage(`{"level":1}`), inventory.Bag{Version: "ordinary-bag-v1", Coin: 1})
	if err != nil {
		t.Fatal(err)
	}
	role := storage.Character{ID: 7, AccountID: 11, WireID: 9, ConfigVersion: source, State: state}
	w := &worldSession{
		role:          role,
		loot:          &loot.Service{Catalog: catalog.LootCatalog{Source: pvf.ArchiveSnapshot{Checksum: source}}},
		dungeons:      &catalog.DungeonCatalog{Maps: map[uint32]catalog.ScriptRecord{1: {Cells: []pvf.Token{{Type: 3, Text: "[cannot use coin map]"}}}}},
		activeDungeon: &dungeon.Session{RunID: "run-2", Loaded: true, Room: catalog.DungeonRoom{Map: 1}},
		pilotDeath:    &odysseyDeath{Run: "run-2", Sequence: 1, Dead: true},
	}
	request := make([]byte, 8)
	binary.LittleEndian.PutUint16(request, role.WireID)
	store := &lifeTokenStore{role: role}
	if _, err = w.lifeTokenRevive(context.Background(), store, request, []byte{41, 1}); err == nil {
		t.Fatal("accepted revive on a coin-forbidden map")
	}
	if store.calls != 0 {
		t.Fatalf("forbidden map opened a transaction: %d", store.calls)
	}
}
