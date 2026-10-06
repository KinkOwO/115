package main

import (
	"context"
	"dfolan/internal/database"
	"dfolan/internal/dungeon"
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestIspinsUnlimitedReceiptFailureKeepsSettlementIntegration(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	dbFixture, e := database.OpenTestFixture(ctx)
	if e != nil {
		t.Fatal(e)
	}
	defer func() {
		if err := dbFixture.Close(); err != nil {
			t.Error(err)
		}
	}()
	s := dbFixture.Storage()
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
	role, e := s.CreateCharacter(ctx, database.Character{AccountID: account, WireID: 7, Name: "PolicyFixture", ConfigVersion: v, Request: []byte{0}, State: json.RawMessage(`{"keep":true}`)}, 24)
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
