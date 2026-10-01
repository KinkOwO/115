package cashshop_test

import (
	"context"
	"dfolan/internal/cashshop"
	"dfolan/internal/game/protocol"
	"dfolan/internal/storage"
	"encoding/hex"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"
)

func TestPurchasePipelineIntegration(t *testing.T) {
	if os.Getenv("CASH_INTEGRATION") != "1" {
		t.Skip("CASH_INTEGRATION=1 uses a disposable PostgreSQL schema")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	cfg, err := storage.LoadConfig("../../runtime/storage/local.json")
	if err != nil {
		t.Fatal(err)
	}
	admin, err := storage.Open(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer admin.Close()
	schema := fmt.Sprintf("cash_pipeline_%d", time.Now().UnixNano())
	if _, err = admin.DB.Exec(ctx, "CREATE SCHEMA "+schema); err != nil {
		t.Fatal(err)
	}
	defer func() {
		cleanup, stop := context.WithTimeout(context.Background(), 10*time.Second)
		defer stop()
		if _, err := admin.DB.Exec(cleanup, "DROP SCHEMA "+schema+" CASCADE"); err != nil {
			t.Error(err)
		}
	}()
	cfg.PostgresSchema, cfg.RedisPrefix = schema, schema+":"
	store, err := storage.Open(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	for _, migrate := range []func(context.Context) error{store.Migrate, store.MigrateGrants, store.MigrateCashShop} {
		if err = migrate(ctx); err != nil {
			t.Fatal(err)
		}
	}
	account, err := store.DevelopmentAccount(ctx, "pipeline-fixture")
	if err != nil {
		t.Fatal(err)
	}
	source := strings.Repeat("a", 64)
	role, err := store.CreateCharacter(ctx, storage.Character{AccountID: account, Name: "CashPipeline", Request: []byte{0}, ConfigVersion: source, State: []byte(`{}`)}, 24)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = store.DB.Exec(ctx, `INSERT INTO account_currency(account_id,cera) VALUES($1,10000)`, account); err != nil {
		t.Fatal(err)
	}
	raw, err := hex.DecodeString("000001000029e3330001000000000000")
	if err != nil {
		t.Fatal(err)
	}
	cart, err := protocol.DecodeCeraCart(raw)
	if err != nil {
		t.Fatal(err)
	}
	p := cashshop.Product{ID: 3400489, Template: 590722921, Units: 1, Cera: 3180, Enabled: true}
	svc := cashshop.Service{Catalog: cashshop.Catalog{Source: source, Products: map[uint32]cashshop.Product{p.ID: p}}, Ledger: store}
	now := time.Date(2026, 9, 16, 0, 0, 0, 0, time.UTC)
	r, applied, err := svc.Purchase(ctx, account, role.ID, "pipeline-order-0001", cart, now)
	if err != nil || !applied || r.Before != 10000 || r.After != 6820 || r.Charged != 3180 || len(r.Deliveries) != 1 {
		t.Fatalf("purchase: %+v applied=%v err=%v", r, applied, err)
	}
	// A separate pool reads persisted results, not the writer's in-memory state.
	reopened, err := storage.Open(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	delivered, err := reopened.CashInventory(ctx, account, role.ID)
	if err != nil || len(delivered) != 1 || delivered[0].Template != 590722921 || delivered[0].Amount != 1 || delivered[0].ID != r.Deliveries[0].ID {
		t.Fatalf("persisted delivery: %+v %v", delivered, err)
	}
	svc.Ledger = reopened
	replay, applied, err := svc.Purchase(ctx, account, role.ID, "pipeline-order-0001", cart, now)
	if err != nil || applied || replay.After != 6820 {
		t.Fatalf("replay: %+v %v %v", replay, applied, err)
	}
	p.Enabled = false
	svc.Catalog.Products[p.ID] = p
	if _, _, err = svc.Purchase(ctx, account, role.ID, "pipeline-order-0002", cart, now); err == nil {
		t.Fatal("disabled product purchased")
	}
	balance, err := reopened.AccountCera(ctx, account)
	if err != nil || balance != 6820 {
		t.Fatalf("balance=%d err=%v", balance, err)
	}
	delivered, err = reopened.CashInventory(ctx, account, role.ID)
	if err != nil || len(delivered) != 1 {
		t.Fatal("rejected/replayed order delivered twice", err)
	}
	t.Log("PASS captured CMD64 -> server price 3180 -> balance 10000/6820 -> durable template590722921 x1; reconnect replay no debit; disabled product no debit")
}
