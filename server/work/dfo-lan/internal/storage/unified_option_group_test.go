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

func TestCharacterUnifiedOptionGroupPersistence(t *testing.T) {
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
	schema := fmt.Sprintf("unified_options_%d", time.Now().UnixNano())
	if _, err = admin.DB.Exec(ctx, "CREATE SCHEMA "+schema); err != nil {
		t.Fatal(err)
	}
	defer admin.DB.Exec(ctx, "DROP SCHEMA "+schema+" CASCADE")
	cfg.PostgresSchema, cfg.RedisPrefix = schema, schema+":"
	store, err := Open(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	if err = store.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	if err = store.MigrateUnifiedOptions(ctx); err != nil {
		t.Fatal(err)
	}
	account, err := store.DevelopmentAccount(ctx, "unified-option-fixture")
	if err != nil {
		t.Fatal(err)
	}
	role, err := store.CreateCharacter(ctx, Character{
		AccountID:     account,
		Name:          "EffectFixture",
		Profession:    0,
		ConfigVersion: strings.Repeat("ab", 32),
		State:         json.RawMessage(`{}`),
		Request:       []byte{0},
	}, 24)
	if err != nil {
		t.Fatal(err)
	}
	entries := []UnifiedOptionEntry{{Position: 2, Value: 63}, {Position: 5, Value: 100}}
	if err = store.SaveCharacterUnifiedOptionGroup(ctx, account, role.ID, 18, entries); err != nil {
		t.Fatal(err)
	}
	got, err := store.CharacterUnifiedOptionGroup(ctx, role.ID, 18)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[2] != 63 || got[5] != 100 {
		t.Fatalf("restored effect options = %v, want map[2:63 5:100]", got)
	}
	if err = store.SaveCharacterUnifiedOptionGroup(ctx, account, role.ID, 18, []UnifiedOptionEntry{{Position: 6, Value: 1}}); err == nil {
		t.Fatal("accepted out-of-range effect option")
	}
	if err = store.SaveCharacterUnifiedOptionGroup(ctx, account+1, role.ID, 18, entries); err == nil {
		t.Fatal("accepted a character not owned by the account")
	}
}
