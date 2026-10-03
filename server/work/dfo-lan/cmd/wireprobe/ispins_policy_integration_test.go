package main

import (
	"context"
	"dfolan/internal/dungeon"
	"dfolan/internal/storage"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"
)

func TestIspinsUnlimitedReceiptFailureKeepsSettlementIntegration(t *testing.T) {
	if os.Getenv("ISPINS_WEEKLY_INTEGRATION") != "1" {
		t.Skip("isolated PostgreSQL schema")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	cfg, e := storage.LoadConfig("../../runtime/storage/local.json")
	if e != nil {
		t.Fatal(e)
	}
	admin, e := storage.Open(ctx, cfg)
	if e != nil {
		t.Fatal(e)
	}
	defer admin.Close()
	schema := fmt.Sprintf("ispins_policy_test_%d", time.Now().UnixNano())
	if _, e = admin.DB.Exec(ctx, "CREATE SCHEMA "+schema); e != nil {
		t.Fatal(e)
	}
	defer admin.DB.Exec(context.Background(), "DROP SCHEMA "+schema+" CASCADE")
	cfg.PostgresSchema = schema
	s, e := storage.Open(ctx, cfg)
	if e != nil {
		t.Fatal(e)
	}
	defer s.Close()
	if e = s.Migrate(ctx); e != nil {
		t.Fatal(e)
	}
	if e = s.MigrateCharacterEvents(ctx); e != nil {
		t.Fatal(e)
	}
	account, e := s.DevelopmentAccount(ctx, "ispins-policy-fixture")
	if e != nil {
		t.Fatal(e)
	}
	v := strings.Repeat("a", 64)
	role, e := s.CreateCharacter(ctx, storage.Character{AccountID: account, WireID: 7, Name: "PolicyFixture", ConfigVersion: v, Request: []byte{0}, State: json.RawMessage(`{"keep":true}`)}, 24)
	if e != nil {
		t.Fatal(e)
	}
	d := &dungeon.Session{RunID: strings.Repeat("4", 32)}
	d.MarkSceneCompleted()
	w := &worldSession{store: s, account: account, role: role, activeDungeon: d, ispins: &ispinsRun{stage: 3, cleared: [4]bool{true, true, true}}}
	w.role.ConfigVersion = strings.Repeat("b", 64)
	t.Setenv("DFO_ISPINS_MODE", "unlimited")
	if plan, e := w.completeIspinsStage(); e != nil || len(plan) == 0 {
		t.Fatal("bookkeeping failure blocked confirmed unlimited settlement", e)
	}
	w.ispins.cleared[3] = false
	t.Setenv("DFO_ISPINS_MODE", "weekly")
	if plan, e := w.completeIspinsStage(); e == nil || len(plan) != 0 {
		t.Fatal("weekly bookkeeping failure allowed a clear", e)
	}
	w.role.ConfigVersion = v
	if plan, e := w.completeIspinsStage(); e != nil || len(plan) == 0 {
		t.Fatal("weekly committed clear", e)
	}
	if used, e := s.IspinsWeeklyUsed(ctx, account, role.ID, time.Now()); e != nil || !used {
		t.Fatal(used, e)
	}
}
