package database

import (
	"bytes"
	"context"
	"dfolan/internal/catalog"
	"dfolan/internal/database/sqlcgen"
	"dfolan/internal/inventory"
	"encoding/json"
	"os"
	"strings"
	"sync"
	"testing"
)

func TestTruthSQLiteTransactionReplayAndReconnect(t *testing.T) {
	ctx := context.Background()
	f, err := OpenTestFixture(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	s := f.Storage()
	if err = s.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile("../catalog/testdata/oath_truth.cos")
	if err != nil {
		t.Fatal(err)
	}
	r, err := catalog.ParseOathTruth(string(raw))
	if err != nil {
		t.Fatal(err)
	}
	r.Source = strings.Repeat("a", 64)
	version := strings.Repeat("b", 64)
	r.Alternatives = map[uint32][]uint32{10421371: {10421372}}
	account, err := s.DevelopmentAccount(ctx, "truth-sqlite")
	if err != nil {
		t.Fatal(err)
	}
	state, err := inventory.SaveBag(json.RawMessage(`{"level":115,"future":{"keep":true}}`), inventory.Bag{Version: "ordinary-bag-v1", Items: []inventory.BagItem{{Slot: 100, Template: 10401346, Amount: 60}, {Slot: 101, Template: 10421371, Amount: 20}}})
	if err != nil {
		t.Fatal(err)
	}
	role, err := s.CreateCharacter(ctx, Character{AccountID: account, Name: "truth", ConfigVersion: version, State: state, Request: []byte{0}}, 24)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.LoadVault(ctx, account, role.ID, 8, version); err != nil {
		t.Fatal(err)
	}
	if err = s.queries.SavePrimaryVaultItems(ctx, sqlcgen.SavePrimaryVaultItemsParams{CharacterID: role.ID, Items: json.RawMessage(`[{"slot":0,"template":10401346,"amount":40,"private":"keep"}]`)}); err != nil {
		t.Fatal(err)
	}
	if err = s.queries.EnsureAccountVault(ctx, account); err != nil {
		t.Fatal(err)
	}
	if err = s.queries.SaveAccountVault(ctx, sqlcgen.SaveAccountVaultParams{AccountID: account, Slots: 8, Gold: 500000, Items: json.RawMessage(`[{"slot":0,"template":10421372,"amount":19,"opaque":17}]`)}); err != nil {
		t.Fatal(err)
	}
	if _, _, err = s.UpgradeOathTruth(ctx, account, role.ID, version, r, 1, 1); err == nil {
		t.Fatal("insufficient shared material accepted")
	}
	truth, err := s.OathTruth(ctx, account, role.ID)
	if err != nil || truth.Level != 0 {
		t.Fatal("failed transaction persisted progress", err)
	}
	primary, err := s.LoadVault(ctx, account, role.ID, 8, version)
	if err != nil || !bytes.Contains(primary.Items, []byte(`"amount":40`)) {
		t.Fatal("partial deduction survived rollback", err)
	}
	if err = s.queries.SaveAccountVault(ctx, sqlcgen.SaveAccountVaultParams{AccountID: account, Slots: 8, Gold: 500000, Items: json.RawMessage(`[{"slot":0,"template":10421372,"amount":21,"opaque":17}]`)}); err != nil {
		t.Fatal(err)
	}
	// Concurrent copies of the same native request must open one stage and
	// spend once, including both vaults, without an extra migration.
	var wg sync.WaitGroup
	results := make(chan bool, 2)
	errors := make(chan error, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, applied, e := s.UpgradeOathTruth(ctx, account, role.ID, version, r, 1, 1)
			results <- applied
			errors <- e
		}()
	}
	wg.Wait()
	close(results)
	close(errors)
	count := 0
	for e := range errors {
		if e != nil {
			t.Fatal(e)
		}
	}
	for applied := range results {
		if applied {
			count++
		}
	}
	if count != 1 {
		t.Fatalf("applied %d times", count)
	}
	saved, err := s.SelectOathTruth(ctx, account, role.ID, version, r, 1, 1)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(saved.State, []byte(`"keep":true`)) {
		t.Fatal("unknown character field lost")
	}
	shared, err := s.LoadAccountVault(ctx, account, role.ID)
	if err != nil || shared.Gold != 500000 || !bytes.Contains(shared.Items, []byte(`"amount":1`)) || !bytes.Contains(shared.Items, []byte(`"opaque":17`)) {
		t.Fatal("wrong shared cost or opaque fields", err)
	}
	reopened, err := f.Reopen(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	truth, err = reopened.OathTruth(ctx, account, role.ID)
	if err != nil || truth.Level != 1 || truth.Options[1] != 1 {
		t.Fatal("reconnect dropped truth", err)
	}
	if _, applied, err := reopened.UpgradeOathTruth(ctx, account, role.ID, version, r, 1, 1); err != nil || applied {
		t.Fatal("replay across reconnect charged again", err)
	}
	if _, err = reopened.SelectOathTruth(ctx, account+1, role.ID, version, r, 1, 0); err == nil {
		t.Fatal("other account changed truth")
	}
	if _, err = reopened.SelectOathTruth(ctx, account, role.ID, version, r, 2, 0); err == nil {
		t.Fatal("locked slot changed")
	}
	if _, _, err = reopened.UpgradeOathTruth(ctx, account, role.ID, version, r, 5, 1); err == nil {
		t.Fatal("source-locked stage opened")
	}
}
