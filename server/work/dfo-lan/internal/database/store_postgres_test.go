package database

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestLoadConfigPostgresOnlyAndLegacyFields(t *testing.T) {
	want := Config{PostgresDSN: "postgres://localhost/dfo", PostgresSchema: "fixture", MaxConnections: 3}
	for _, extra := range []string{"", `,"redis_address":"127.0.0.1:1","redis_password":"unused","redis_prefix":"old:"`} {
		path := filepath.Join(t.TempDir(), "local.json")
		raw := `{"postgres_dsn":"postgres://localhost/dfo","postgres_schema":"fixture","max_connections":3` + extra + `}`
		if err := os.WriteFile(path, []byte(raw), 0600); err != nil {
			t.Fatal(err)
		}
		got, err := LoadConfig(path)
		if err != nil || got != want {
			t.Fatalf("load config: got %+v, err %v", got, err)
		}
	}
}

// A dedicated test DSN and private schema keep player databases untouched.
// Reopening with a legacy config also proves obsolete endpoints are ignored.
func TestPostgresOnlyCharacterPersistence(t *testing.T) {
	dsn := os.Getenv("DFO_TEST_POSTGRES_DSN")
	if dsn == "" {
		t.Skip("DFO_TEST_POSTGRES_DSN requires a dedicated PostgreSQL test database")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	cfg := Config{PostgresDSN: dsn, MaxConnections: 2}
	admin, err := Open(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer admin.Close()
	schema := fmt.Sprintf("postgres_only_%d", time.Now().UnixNano())
	if _, err := admin.db.Exec(ctx, "CREATE SCHEMA "+schema); err != nil {
		t.Fatal(err)
	}
	defer admin.db.Exec(context.Background(), "DROP SCHEMA "+schema+" CASCADE")
	cfg.PostgresSchema = schema
	s, err := Open(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if s != nil {
			s.Close()
		}
	}()
	if err := s.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	if err := s.MigrateCharacterEvents(ctx); err != nil {
		t.Fatal(err)
	}
	account, err := s.DevelopmentAccount(ctx, "postgres-only-fixture")
	if err != nil {
		t.Fatal(err)
	}
	version := strings.Repeat("a", 64)
	role, err := s.CreateCharacter(ctx, Character{AccountID: account, Name: "PGOnly", Request: []byte{0}, ConfigVersion: version,
		State: json.RawMessage(`{"inventory":{"version":"ordinary-bag-v1","items":[{"slot":68,"Template":10310180,"Amount":2}]},"legacy":{"keep":true}}`)}, 24)
	if err != nil {
		t.Fatal(err)
	}
	updated, applied, err := s.CommitCharacterEvent(ctx, account, role.ID, version, "fixture-event", "postgres-only-v1", func(c Character) (json.RawMessage, json.RawMessage, error) {
		var state map[string]json.RawMessage
		if err := json.Unmarshal(c.State, &state); err != nil {
			return nil, nil, err
		}
		state["event_value"] = json.RawMessage(`7`)
		raw, err := json.Marshal(state)
		return raw, json.RawMessage(`{"saved":true}`), err
	})
	if err != nil || !applied {
		t.Fatalf("commit: applied=%v err=%v", applied, err)
	}
	var original, saved map[string]json.RawMessage
	if err := json.Unmarshal(role.State, &original); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(updated.State, &saved); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"inventory", "legacy"} {
		if !sameJSON(t, original[key], saved[key]) {
			t.Fatalf("existing field %s changed", key)
		}
	}
	legacy := map[string]any{"postgres_dsn": dsn, "postgres_schema": schema, "max_connections": 2,
		"redis_address": "127.0.0.1:1", "redis_password": "unused", "redis_prefix": "obsolete:"}
	raw, err := json.Marshal(legacy)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "legacy.json")
	if err := os.WriteFile(path, raw, 0600); err != nil {
		t.Fatal(err)
	}
	cfg, err = LoadConfig(path)
	if err != nil {
		t.Fatal(err)
	}
	s.Close()
	s, err = Open(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	roles, err := s.Characters(ctx, account)
	if err != nil || len(roles) != 1 || roles[0].ID != role.ID || !sameJSON(t, roles[0].State, updated.State) {
		t.Fatalf("reopen did not preserve saved role: roles=%d err=%v", len(roles), err)
	}
	_, applied, err = s.CommitCharacterEvent(ctx, account, role.ID, version, "fixture-event", "postgres-only-v1", func(Character) (json.RawMessage, json.RawMessage, error) {
		t.Error("replayed event recomputed state")
		return nil, nil, nil
	})
	if err != nil || applied {
		t.Fatalf("replay: applied=%v err=%v", applied, err)
	}
	receipt, err := s.CharacterEventReceipt(ctx, account, role.ID, "fixture-event")
	if err != nil || !sameJSON(t, receipt, json.RawMessage(`{"saved":true}`)) {
		t.Fatalf("saved receipt missing after reopen: %s err=%v", receipt, err)
	}
}

func sameJSON(t *testing.T, a, b json.RawMessage) bool {
	t.Helper()
	var av, bv any
	if err := json.Unmarshal(a, &av); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(b, &bv); err != nil {
		t.Fatal(err)
	}
	return reflect.DeepEqual(av, bv)
}
