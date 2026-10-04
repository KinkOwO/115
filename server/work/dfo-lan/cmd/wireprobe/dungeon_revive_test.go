package main

import (
	"context"
	"dfolan/internal/catalog"
	"dfolan/internal/catalog/pvf"
	"dfolan/internal/database"
	"dfolan/internal/dungeon"
	"dfolan/internal/inventory"
	"dfolan/internal/loot"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"testing"
)

type lifeTokenStore struct {
	role  database.Character
	keys  map[string]bool
	calls int
}

type ceraReviveFake struct {
	lifeTokenStore
	balance uint64
	grants  []database.Grant
	failed  error
	paid    map[string]bool
}

func (s *ceraReviveFake) ApplyGrant(_ context.Context, g database.Grant, _ func(database.Character) (json.RawMessage, json.RawMessage, error)) (database.GrantResult, error) {
	s.grants = append(s.grants, g)
	if s.failed != nil {
		return database.GrantResult{}, s.failed
	}
	if s.paid == nil {
		s.paid = map[string]bool{}
	}
	if !s.paid[g.ID] {
		if g.Cera != -lifeTokenCeraCost || s.balance < uint64(-g.Cera) {
			return database.GrantResult{}, fmt.Errorf("insufficient CERA")
		}
		remaining := s.balance - uint64(-g.Cera)
		if g.MaxCera != 0 && remaining > g.MaxCera {
			return database.GrantResult{}, fmt.Errorf("CERA exceeds client range")
		}
		s.balance = remaining
		s.paid[g.ID] = true
	}
	return database.GrantResult{Cera: s.balance, Applied: true}, nil
}

func ceraReviveFixture(t *testing.T, coins uint32) (*worldSession, *ceraReviveFake, []byte) {
	t.Helper()
	const source = "cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc"
	state, e := inventory.SaveBag(json.RawMessage(`{"level":1}`), inventory.Bag{Version: "ordinary-bag-v1", Coin: coins})
	if e != nil {
		t.Fatal(e)
	}
	role := database.Character{ID: 7, AccountID: 11, WireID: 9, ConfigVersion: source, State: state}
	store := &ceraReviveFake{lifeTokenStore: lifeTokenStore{role: role}, balance: 30}
	w := &worldSession{
		role:          role,
		loot:          &loot.Service{Catalog: catalog.LootCatalog{Source: pvf.ArchiveSnapshot{Checksum: source}}},
		dungeons:      &catalog.DungeonCatalog{Maps: map[uint32]catalog.ScriptRecord{1: {}}},
		activeDungeon: &dungeon.Session{RunID: "run-cera", Loaded: true, Room: catalog.DungeonRoom{Map: 1}},
		pilotDeath:    &odysseyDeath{Run: "run-cera", Sequence: 3, Dead: true},
	}
	p := make([]byte, 8)
	binary.LittleEndian.PutUint16(p, role.WireID)
	return w, store, p
}

func TestCeraReviveChargesFifteenAndSendsThreeFrames(t *testing.T) {
	w, store, p := ceraReviveFixture(t, 0)
	plan, e := w.ceraRevive(context.Background(), store, p, []byte{41, 3})
	if e != nil {
		t.Fatal(e)
	}
	if len(plan) != 3 || plan[0].ID != 41 || plan[1].ID != 32 || plan[2].ID != 53 || plan[1].Payload[2] != 1 {
		t.Fatalf("wrong packet sequence: %+v", plan)
	}
	if got := plan[0].Payload; len(got) != 3 || got[0] != 1 || binary.LittleEndian.Uint16(got[1:]) != w.role.WireID {
		t.Fatalf("wrong ACK: %x", got)
	}
	if got := plan[2].Payload; len(got) != 9 || got[0] != 1 || binary.LittleEndian.Uint32(got[1:5]) != 15 {
		t.Fatalf("wrong CERA balance: %x", got)
	}
	if len(store.grants) != 1 || store.grants[0].ID != "cera-revive:run-cera:3" || store.grants[0].AccountID != 11 || store.grants[0].Character != 0 || store.grants[0].Cera != -15 || store.grants[0].MaxCera != math.MaxInt32 || store.balance != 15 || w.pilotDeath.Dead {
		t.Fatalf("wrong charge or state: grants=%+v balance=%d dead=%t", store.grants, store.balance, w.pilotDeath.Dead)
	}
}

func TestCeraReviveIsIdempotentPerDeath(t *testing.T) {
	w, store, p := ceraReviveFixture(t, 0)
	frame := []byte{41, 3}
	if _, e := w.ceraRevive(context.Background(), store, p, frame); e != nil {
		t.Fatal(e)
	}
	plan, e := w.useCoinRevive(context.Background(), store, p, frame, false)
	if e != nil || plan != nil || len(store.grants) != 1 || store.balance != 15 {
		t.Fatalf("replay charged again: plan=%+v grants=%d balance=%d err=%v", plan, len(store.grants), store.balance, e)
	}
}

func TestCeraReviveInsufficientBalanceRefuses22(t *testing.T) {
	w, store, p := ceraReviveFixture(t, 0)
	store.balance = 14
	plan, e := w.useCoinRevive(context.Background(), store, p, []byte{41, 3}, false)
	if e == nil || len(plan) != 0 || !w.pilotDeath.Dead || store.balance != 14 {
		t.Fatalf("insufficient balance revived: plan=%+v err=%v", plan, e)
	}
	if got := boosterActionRefusal(41); len(got) != 3 || binary.LittleEndian.Uint16(got[1:]) != 22 {
		t.Fatalf("wrong refusal payload: %x", got)
	}
	if got := boosterActionRefusal(160); binary.LittleEndian.Uint16(got[1:]) != 4 {
		t.Fatalf("other action refusal changed: %x", got)
	}
}

func TestReviveFallsThroughTokenToCera(t *testing.T) {
	w, store, p := ceraReviveFixture(t, 0)
	plan, e := w.useCoinRevive(context.Background(), store, p, []byte{41, 3}, false)
	if e != nil || len(plan) != 3 || len(store.grants) != 1 || store.balance != 15 {
		t.Fatalf("CERA fallback failed: plan=%+v grants=%d err=%v", plan, len(store.grants), e)
	}
}

func TestReviveWithTokenDoesNotChargeCera(t *testing.T) {
	w, store, p := ceraReviveFixture(t, 1)
	plan, e := w.useCoinRevive(context.Background(), store, p, []byte{41, 3}, false)
	if e != nil || len(plan) != 3 || plan[2].ID != 14 || len(store.grants) != 0 || store.balance != 30 {
		t.Fatalf("token revive charged CERA: plan=%+v grants=%d err=%v", plan, len(store.grants), e)
	}
}

func TestOdysseyCreditsExhaustThenTokenThenCera(t *testing.T) {
	role, _ := odysseyRewardFixture(t)
	role.AccountID = 11
	state, e := inventory.SaveBag(role.State, inventory.Bag{Version: "ordinary-bag-v1", Coin: 0})
	if e != nil {
		t.Fatal(e)
	}
	role.State = state
	store := &ceraReviveFake{lifeTokenStore: lifeTokenStore{role: role}, balance: 30}
	w := &worldSession{
		role:          role,
		loot:          &loot.Service{Catalog: catalog.LootCatalog{Source: pvf.ArchiveSnapshot{Checksum: odysseySource()}}},
		dungeons:      &catalog.DungeonCatalog{Maps: map[uint32]catalog.ScriptRecord{1: {}}},
		activeDungeon: &dungeon.Session{RunID: "odyssey-run", Loaded: true, Definition: catalog.DungeonDefinition{Odyssey: true}, Room: catalog.DungeonRoom{Map: 1}},
		pilotDeath:    &odysseyDeath{Run: "odyssey-run", Sequence: 1, Dead: true, Revives: map[[32]byte]bool{}},
	}
	p := make([]byte, 8)
	binary.LittleEndian.PutUint16(p, role.WireID)
	plan, e := w.useCoinRevive(context.Background(), store, p, []byte{41, 1}, true)
	if e != nil || len(plan) != 3 || plan[2].ID != 53 || len(store.grants) != 1 || store.calls != 2 {
		t.Fatalf("exhausted credits did not reach CERA: plan=%+v calls=%d grants=%d err=%v", plan, store.calls, len(store.grants), e)
	}
}

func TestOdysseyCreditsDoNotChargeTokenOrCera(t *testing.T) {
	role, _ := odysseyRewardFixture(t)
	role.AccountID = 11
	role.State = json.RawMessage(`{"level":1,"custom_marker":42,"odyssey_pilot_revive_credits":1}`)
	state, e := inventory.SaveBag(role.State, inventory.Bag{Version: "ordinary-bag-v1", Coin: 1})
	if e != nil {
		t.Fatal(e)
	}
	role.State = state
	store := &ceraReviveFake{lifeTokenStore: lifeTokenStore{role: role}, balance: 30}
	w := &worldSession{
		role:          role,
		loot:          &loot.Service{Catalog: catalog.LootCatalog{Source: pvf.ArchiveSnapshot{Checksum: odysseySource()}}},
		dungeons:      &catalog.DungeonCatalog{Maps: map[uint32]catalog.ScriptRecord{1: {}}},
		activeDungeon: &dungeon.Session{RunID: "odyssey-run", Loaded: true, Definition: catalog.DungeonDefinition{Odyssey: true}, Room: catalog.DungeonRoom{Map: 1}},
		pilotDeath:    &odysseyDeath{Run: "odyssey-run", Sequence: 1, Dead: true, Revives: map[[32]byte]bool{}},
	}
	p := make([]byte, 8)
	binary.LittleEndian.PutUint16(p, role.WireID)
	plan, e := w.useCoinRevive(context.Background(), store, p, []byte{41, 1}, true)
	bag, bagErr := inventory.ReadBag(store.role.State)
	if e != nil || bagErr != nil || len(plan) != 2 || len(store.grants) != 0 || store.calls != 1 || bag.Coin != 1 {
		t.Fatalf("pilot success fell through: plan=%+v calls=%d grants=%d bag=%+v err=%v bagErr=%v", plan, store.calls, len(store.grants), bag, e, bagErr)
	}
}

func TestReviveDoesNotFallThroughUnrelatedErrors(t *testing.T) {
	w, store, p := ceraReviveFixture(t, 0)
	p[0]++
	if plan, e := w.useCoinRevive(context.Background(), store, p, []byte{41, 3}, false); e == nil || plan != nil || len(store.grants) != 0 {
		t.Fatalf("invalid request fell through: plan=%+v err=%v", plan, e)
	}
	w, store, p = ceraReviveFixture(t, 0)
	if plan, e := w.useCoinRevive(context.Background(), store, p, []byte{41, 3}, true); e == nil || len(plan) != 0 || store.calls != 0 || len(store.grants) != 0 {
		t.Fatalf("pilot eligibility error fell through: plan=%+v err=%v", plan, e)
	}
	w, store, p = ceraReviveFixture(t, 0)
	store.failed = errors.New("storage unavailable")
	if plan, e := w.useCoinRevive(context.Background(), store, p, []byte{41, 3}, false); e == nil || len(plan) != 0 || !w.pilotDeath.Dead {
		t.Fatalf("storage failure revived: plan=%+v err=%v", plan, e)
	}
	w, store, p = ceraReviveFixture(t, 0)
	w.dungeons.Maps[1] = catalog.ScriptRecord{Cells: []pvf.Token{{Type: 3, Text: "[cannot use coin map]"}}}
	if plan, e := w.useCoinRevive(context.Background(), store, p, []byte{41, 3}, false); e == nil || len(plan) != 0 || len(store.grants) != 0 {
		t.Fatalf("forbidden map reached CERA: plan=%+v err=%v", plan, e)
	}
}

func (s *lifeTokenStore) CommitCharacterEvent(_ context.Context, _ int64, _ int64, _, key, _ string, apply func(database.Character) (json.RawMessage, json.RawMessage, error)) (database.Character, bool, error) {
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
	role := database.Character{ID: 7, AccountID: 11, WireID: 9, ConfigVersion: source, State: state}
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
	role := database.Character{ID: 7, AccountID: 11, WireID: 9, ConfigVersion: source, State: state}
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
