package database

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestVaultTransferIntegration(t *testing.T) {
	if os.Getenv("DFO_TEST_POSTGRES_DSN") == "" {
		t.Skip("DFO_TEST_POSTGRES_DSN uses isolated PostgreSQL schema")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	cfg, e := loadPostgresTestConfig()
	if e != nil {
		t.Fatal(e)
	}
	live, e := Open(ctx, cfg)
	if e != nil {
		t.Fatal(e)
	}
	defer live.Close()
	schema := fmt.Sprintf("vault_test_%d", time.Now().UnixNano())
	if _, e = testPool(t, live).Exec(ctx, "CREATE SCHEMA "+schema); e != nil {
		t.Fatal(e)
	}
	defer func() {
		if _, e := testPool(t, live).Exec(context.Background(), "DROP SCHEMA "+schema+" CASCADE"); e != nil {
			t.Error(e)
		}
	}()
	cfg.PostgresSchema = schema
	cfg.MaxConnections = 8
	s, e := Open(ctx, cfg)
	if e != nil {
		t.Fatal(e)
	}
	defer s.Close()
	for _, f := range []func(context.Context) error{s.Migrate, s.MigrateCharacterEvents, s.MigrateVault} {
		if e = f(ctx); e != nil {
			t.Fatal(e)
		}
	}
	account, e := s.DevelopmentAccount(ctx, "vault-fixture")
	if e != nil {
		t.Fatal(e)
	}
	h := strings.Repeat("a", 64)
	role, e := s.CreateCharacter(ctx, Character{AccountID: account, Name: "VaultFixture", Request: []byte{0}, ConfigVersion: h, State: []byte(`{"count":10}`)}, 24)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = s.LoadVault(ctx, account, role.ID, 8, h); e != nil {
		t.Fatal(e)
	}
	var calls atomic.Int32
	apply := func(c Character, v VaultState) (json.RawMessage, json.RawMessage, error) {
		calls.Add(1)
		return []byte(`{"count":6}`), []byte(`[{"slot":0,"Template":15,"Amount":4}]`), nil
	}
	var wg sync.WaitGroup
	var applied atomic.Int32
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _, yes, e := s.CommitVaultTransfer(ctx, account, role.ID, h, h, "same-drag", []byte("deposit-four"), apply)
			if e != nil {
				t.Error(e)
			}
			if yes {
				applied.Add(1)
			}
		}()
	}
	wg.Wait()
	if calls.Load() != 1 || applied.Load() != 1 {
		t.Fatal("concurrent replay duplicated transfer", calls.Load(), applied.Load())
	}
	if _, _, _, e = s.CommitVaultTransfer(ctx, account, role.ID, h, h, "same-drag", []byte("different"), apply); e == nil {
		t.Fatal("replay conflict accepted")
	}
	if _, _, _, e = s.CommitVaultTransfer(ctx, account+1000, role.ID, h, h, "other", []byte("deposit"), apply); e == nil {
		t.Fatal("other owner accepted")
	}
	if _, _, _, e = s.CommitVaultTransfer(ctx, account, role.ID, strings.Repeat("b", 64), h, "source", []byte("deposit"), apply); e == nil {
		t.Fatal("other source accepted")
	}
	// Fail the audit insert after BOTH updates, proving transaction rollback.
	if _, e = testPool(t, s).Exec(ctx, `ALTER TABLE character_events ADD CONSTRAINT reject_fixture_event CHECK(event_key <> 'fail-after-updates')`); e != nil {
		t.Fatal(e)
	}
	bad := func(c Character, v VaultState) (json.RawMessage, json.RawMessage, error) {
		return []byte(`{"count":0}`), []byte(`[]`), nil
	}
	if _, _, _, e = s.CommitVaultTransfer(ctx, account, role.ID, h, h, "fail-after-updates", []byte("fail"), bad); e == nil {
		t.Fatal("injected database failure missing")
	}
	current, v, yes, e := s.CommitVaultTransfer(ctx, account, role.ID, h, h, "same-drag", []byte("deposit-four"), apply)
	if e != nil || yes {
		t.Fatal(e, yes)
	}
	var state map[string]int
	json.Unmarshal(current.State, &state)
	var items []struct{ Amount int }
	json.Unmarshal(v.Items, &items)
	if state["count"] != 6 || len(items) != 1 || items[0].Amount != 4 {
		t.Fatal("partial rollback", state, items)
	}
	// A later event updates the states; replay must return those current values.
	if _, _, _, e = s.CommitVaultTransfer(ctx, account, role.ID, h, h, "withdraw", []byte("withdraw"), func(Character, VaultState) (json.RawMessage, json.RawMessage, error) {
		return []byte(`{"count":10}`), []byte(`[]`), nil
	}); e != nil {
		t.Fatal(e)
	}
	current, v, _, e = s.CommitVaultTransfer(ctx, account, role.ID, h, h, "same-drag", []byte("deposit-four"), apply)
	if e != nil {
		t.Fatal(e)
	}
	json.Unmarshal(current.State, &state)
	json.Unmarshal(v.Items, &items)
	if state["count"] != 10 || len(items) != 0 {
		t.Fatal("replay restored old snapshot")
	}
	if calls.Load() != 1 {
		t.Fatal("replay executed callback")
	}
	t.Log("PASS atomic bag/vault updates; 8 concurrent retries; owner/source/conflict checks; failure after both updates rolls back; replay returns current states")
}
