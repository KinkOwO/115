package database

import (
	"context"
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"
)

func TestFixtureRequiresDedicatedDSN(t *testing.T) {
	t.Setenv("DFO_TEST_POSTGRES_DSN", "")
	if f, err := OpenTestFixture(context.Background()); err == nil || f != nil {
		t.Fatalf("fixture without explicit DSN opened: %v %v", f, err)
	}
}

func TestSQLCFixtureReopenAndCleanup(t *testing.T) {
	if os.Getenv("DFO_TEST_POSTGRES_DSN") == "" {
		t.Skip("dedicated test database required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	f, err := OpenTestFixture(ctx)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := f.Close(); err != nil {
			t.Error(err)
		}
	})
	if err := f.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	account, err := f.DevelopmentAccount(ctx, "fixture-reopen")
	if err != nil {
		t.Fatal(err)
	}
	role, err := f.CreateCharacter(ctx, Character{AccountID: account, Name: "FixtureReopen", ConfigVersion: strings.Repeat("a", 64), Request: []byte{0, 255}, State: json.RawMessage(`{"unknown":true}`)}, 4)
	if err != nil {
		t.Fatal(err)
	}
	if err := f.SeedCharacterState(ctx, role.ID, json.RawMessage(`{"unknown":true,"seed":1}`)); err != nil {
		t.Fatal(err)
	}
	reopened, err := f.Reopen(ctx)
	if err != nil {
		t.Fatal(err)
	}
	rows, err := reopened.Characters(ctx, account)
	if err != nil || len(rows) != 1 || !sameJSON(t, rows[0].State, json.RawMessage(`{"unknown":true,"seed":1}`)) {
		t.Fatalf("fixture reopen: %+v %v", rows, err)
	}
	reopened.Close()
	schema := f.schema
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	admin, err := Open(ctx, Config{PostgresDSN: os.Getenv("DFO_TEST_POSTGRES_DSN")})
	if err != nil {
		t.Fatal(err)
	}
	defer admin.Close()
	var remaining bool
	if err := testPool(t, admin).QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM pg_namespace WHERE nspname=$1)`, schema).Scan(&remaining); err != nil || remaining {
		t.Fatalf("fixture schema not removed: %v %v", remaining, err)
	}
}
