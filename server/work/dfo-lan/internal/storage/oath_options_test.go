package storage

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"
)

func TestOathOptionsRejectInvalidKeysWithoutDatabase(t *testing.T) {
	var s *Store
	ctx := context.Background()
	if _, _, err := s.OathOption(ctx, 0, "core"); err == nil {
		t.Fatal("character 0 must be rejected before database access")
	}
	if _, err := s.SaveOathOption(ctx, 1, "", 0, 0); err == nil {
		t.Fatal("empty instance key must be rejected before database access")
	}
	if _, err := s.SaveOathOption(ctx, 1, "core", -1, 0); err == nil {
		t.Fatal("negative option must be rejected before database access")
	}
}

func TestOathOptionRevisionWrites(t *testing.T) {
	if os.Getenv("CASH_INTEGRATION") != "1" {
		t.Skip("isolated schema integration")
	}
	ctx := context.Background()
	cfg, err := LoadConfig("../../runtime/storage/local.json")
	if err != nil {
		t.Fatal(err)
	}
	admin, err := Open(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer admin.Close()
	schema := fmt.Sprintf("oath_options_%d", time.Now().UnixNano())
	if _, err = admin.DB.Exec(ctx, "CREATE SCHEMA "+schema); err != nil {
		t.Fatal(err)
	}
	defer admin.DB.Exec(context.Background(), "DROP SCHEMA "+schema+" CASCADE")
	cfg.PostgresSchema, cfg.RedisPrefix = schema, schema+":"
	store, err := Open(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	for _, migrate := range []func(context.Context) error{store.Migrate, store.MigrateCharacterEvents, store.MigrateOathOptions} {
		if err = migrate(ctx); err != nil {
			t.Fatal(err)
		}
	}
	account, err := store.DevelopmentAccount(ctx, "oath-options-fixture")
	if err != nil {
		t.Fatal(err)
	}
	role, err := store.CreateCharacter(ctx, Character{AccountID: account, Name: "OathOptionFixture", Profession: 0, ConfigVersion: strings.Repeat("ab", 32), State: json.RawMessage(`{}`), Request: []byte{0}}, 24)
	if err != nil {
		t.Fatal(err)
	}
	state, err := store.SaveOathOption(ctx, role.ID, "core-instance-1", 1, 0)
	if err != nil || state.Revision != 1 {
		t.Fatalf("insert = %+v err=%v", state, err)
	}
	if _, err = store.SaveOathOption(ctx, role.ID, "core-instance-1", 2, 0); err == nil {
		t.Fatal("stale insert must conflict")
	}
	state, err = store.SaveOathOption(ctx, role.ID, "core-instance-1", 2, 1)
	if err != nil || state.Revision != 2 {
		t.Fatalf("update = %+v err=%v", state, err)
	}
	got, ok, err := store.OathOption(ctx, role.ID, "core-instance-1")
	if err != nil || !ok || got.SelectedOption != 2 || got.Revision != 2 {
		t.Fatalf("read = %+v found=%v err=%v", got, ok, err)
	}
}
