package database

import (
	"encoding/json"
	"errors"
	"math"
	"os"
	"strings"
	"testing"
	"time"
)

func TestSQLCGrantUsesOneConnectionAndRollsBackPayout(t *testing.T) {
	fixture, ctx := sqlcTestStore(t)
	for _, migrate := range []func() error{func() error { return fixture.Migrate(ctx) }, func() error { return fixture.MigrateGrants(ctx) }} {
		if err := migrate(); err != nil {
			t.Fatal(err)
		}
	}
	s, err := Open(ctx, Config{PostgresDSN: os.Getenv("DFO_TEST_POSTGRES_DSN"), PostgresSchema: testPool(t, fixture).Config().ConnConfig.RuntimeParams["search_path"], MaxConnections: 1})
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	account, err := s.DevelopmentAccount(ctx, "grant")
	if err != nil {
		t.Fatal(err)
	}
	role, err := s.CreateCharacter(ctx, Character{AccountID: account, Name: "Grant", Request: []byte{1}, ConfigVersion: strings.Repeat("e", 64), State: json.RawMessage(`{"unknown":true,"gold":0}`)}, 4)
	if err != nil {
		t.Fatal(err)
	}
	grant := Grant{ID: "grant-once", AccountID: account, Cera: 100, Operator: "test", Reason: "sqlc verification"}
	first, err := s.ApplyGrant(ctx, grant, nil)
	if err != nil || !first.Applied || first.Cera != 100 {
		t.Fatalf("grant: %+v %v", first, err)
	}
	replay, err := s.ApplyGrant(ctx, grant, nil)
	if err != nil || replay.Applied || replay.Cera != 100 || !sameJSON(t, replay.Receipt, first.Receipt) {
		t.Fatalf("grant replay: %+v %v", replay, err)
	}
	grant.ID = "grant-character"
	grant.Character = role.ID
	grant.Cera = 0
	applied, err := s.ApplyGrant(ctx, grant, func(c Character) (json.RawMessage, json.RawMessage, error) {
		return json.RawMessage(`{"unknown":true,"gold":10}`), json.RawMessage(`{"gold":10}`), nil
	})
	if err != nil || !applied.Applied || applied.Cera != 100 {
		t.Fatalf("character-only grant: %+v %v", applied, err)
	}
	grant.ID = "grant-rollback"
	grant.Cera = 30
	boom := errors.New("bag full")
	if _, err := s.ApplyGrant(ctx, grant, func(Character) (json.RawMessage, json.RawMessage, error) { return nil, nil, boom }); !errors.Is(err, boom) {
		t.Fatalf("mutation failure: %v", err)
	}
	if balance, err := s.AccountCera(ctx, account); err != nil || balance != 100 {
		t.Fatalf("rolled back cera: %d %v", balance, err)
	}
	history, err := s.GrantHistory(ctx, account, 100)
	if err != nil || len(history) != 2 {
		t.Fatalf("failed grant claimed: %+v %v", history, err)
	}
	grant.MaxCera = 110
	if _, err := s.ApplyGrant(ctx, grant, func(c Character) (json.RawMessage, json.RawMessage, error) {
		return c.State, json.RawMessage(`{}`), nil
	}); err == nil {
		t.Fatal("client ceiling bypass")
	}
	grant.MaxCera = 0
	grant.Cera = -101
	if _, err := s.ApplyGrant(ctx, grant, func(c Character) (json.RawMessage, json.RawMessage, error) {
		return c.State, json.RawMessage(`{}`), nil
	}); err == nil {
		t.Fatal("negative balance allowed")
	}
	grant.Cera = 30
	if result, err := s.ApplyGrant(ctx, grant, func(c Character) (json.RawMessage, json.RawMessage, error) {
		return c.State, json.RawMessage(`{}`), nil
	}); err != nil || !result.Applied || result.Cera != 130 {
		t.Fatalf("rolled back key not reusable: %+v %v", result, err)
	}
}

func TestSQLCPremiumRenewalAndEventReplay(t *testing.T) {
	s, ctx := sqlcTestStore(t)
	for _, migrate := range []func() error{func() error { return s.Migrate(ctx) }, func() error { return s.MigrateGrants(ctx) }, func() error { return s.MigratePremiums(ctx) }, func() error { return s.MigrateCharacterEvents(ctx) }} {
		if err := migrate(); err != nil {
			t.Fatal(err)
		}
	}
	account, err := s.DevelopmentAccount(ctx, "premium")
	if err != nil {
		t.Fatal(err)
	}
	version := strings.Repeat("f", 64)
	role, err := s.CreateCharacter(ctx, Character{AccountID: account, Name: "Premium", Request: []byte{1}, ConfigVersion: version, State: json.RawMessage(`{"unknown":true}`)}, 4)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	if rows, err := s.ActivePremiums(ctx, account, now); err != nil || rows == nil || len(rows) != 0 {
		t.Fatalf("empty premiums: %+v %v", rows, err)
	}
	first, err := s.ActivatePremium(ctx, account, PremiumGrowth, 3600)
	if err != nil {
		t.Fatal(err)
	}
	next, err := s.ActivatePremium(ctx, account, PremiumGrowth, 600)
	if err != nil || next != first+600 {
		t.Fatalf("renewal: %d %d %v", first, next, err)
	}
	if active, err := s.HasGrowthPremium(ctx, account, now); err != nil || !active {
		t.Fatalf("growth: %v %v", active, err)
	}
	if set, err := s.ActivePremiumSet(ctx, account, now); err != nil || !set[PremiumGrowth] {
		t.Fatalf("premium set: %+v %v", set, err)
	}
	if active, err := s.HasGrowthPremium(ctx, account, time.Unix(next, 0)); err != nil || active {
		t.Fatalf("expiry boundary: %v %v", active, err)
	}
	if _, err := s.ActivatePremium(ctx, account, PremiumGrowth, math.MaxInt64); err == nil {
		t.Fatal("expiry overflow allowed")
	}
	callbackCount := 0
	apply := func(c Character) (json.RawMessage, json.RawMessage, []CashPremiumActivation, error) {
		callbackCount++
		return c.State, json.RawMessage(`{"source":"test"}`), []CashPremiumActivation{{Type: PremiumGrowth, DurationSecond: 100}}, nil
	}
	for i := 0; i < 2; i++ {
		_, ok, err := s.CommitCharacterPremiumEvent(ctx, account, role.ID, version, "premium-test", "test", apply)
		if err != nil || ok != (i == 0) {
			t.Fatalf("premium event %d: %v %v", i, ok, err)
		}
	}
	if callbackCount != 1 {
		t.Fatalf("replay invoked callback %d times", callbackCount)
	}
	active, err := s.ActivePremiums(ctx, account, now)
	if err != nil || len(active) != 1 || active[0].EndTime != next+100 {
		t.Fatalf("event renewed twice: %+v %v", active, err)
	}
	receipt, err := s.CharacterEventReceipt(ctx, account, role.ID, "premium-test")
	var fields map[string]json.RawMessage
	if err != nil || json.Unmarshal(receipt, &fields) != nil || fields["premiums"] == nil {
		t.Fatalf("premium receipt: %s %v", receipt, err)
	}
	_, _, err = s.CommitCharacterPremiumEvent(ctx, account, role.ID, version, "premium-rollback", "test", func(c Character) (json.RawMessage, json.RawMessage, []CashPremiumActivation, error) {
		return c.State, json.RawMessage(`[]`), []CashPremiumActivation{{Type: PremiumGrowth, DurationSecond: 200}}, nil
	})
	if err == nil {
		t.Fatal("array receipt accepted")
	}
	active, err = s.ActivePremiums(ctx, account, now)
	if err != nil || len(active) != 1 || active[0].EndTime != next+100 {
		t.Fatalf("failed event renewed premium: %+v %v", active, err)
	}
}
