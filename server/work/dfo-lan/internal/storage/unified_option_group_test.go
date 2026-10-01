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
	t.Run("shared settings and hotkey persistence", func(t *testing.T) {
		values := []UnifiedOptionEntry{{Position: 2, Value: 63}, {Position: 5, Value: 65535}}
		if err := store.SaveAccountUnifiedOptions(ctx, account, values); err != nil {
			t.Fatal(err)
		}
		got, err := store.AccountUnifiedOptions(ctx, account)
		if err != nil || len(got) != 1 || got[2] != 63 {
			t.Fatalf("account sentinel filtering: %v, %v", got, err)
		}
		if err := store.SaveCharacterUnifiedOptions(ctx, account, role.ID, values); err != nil {
			t.Fatal(err)
		}
		got, err = store.CharacterUnifiedOptions(ctx, role.ID)
		if err != nil || len(got) != 2 || got[5] != 65535 {
			t.Fatalf("character sentinel preservation: %v, %v", got, err)
		}
		for _, subtype := range []byte{3, 4} {
			if err := store.SaveAccountHotkeys(ctx, account, subtype, values); err != nil {
				t.Fatal(err)
			}
			if err := store.SaveCharacterHotkeys(ctx, account, role.ID, subtype, values); err != nil {
				t.Fatal(err)
			}
			for _, read := range []func() (map[uint16]uint16, error){
				func() (map[uint16]uint16, error) { return store.AccountHotkeys(ctx, account, subtype) },
				func() (map[uint16]uint16, error) { return store.CharacterHotkeys(ctx, role.ID, subtype) },
			} {
				got, err := read()
				if err != nil || len(got) != 2 || got[2] != 63 || got[5] != 65535 {
					t.Fatalf("hotkey restore subtype %d: %v, %v", subtype, got, err)
				}
			}
			if err := store.SaveCharacterHotkeys(ctx, account+1, role.ID, subtype, values); err == nil {
				t.Fatal("accepted hotkeys from a different account")
			}
		}
		if err := store.SaveCharacterUnifiedOptions(ctx, account+1, role.ID, values); err == nil {
			t.Fatal("accepted settings from a different account")
		}
	})
	t.Run("failed write rolls back earlier entries", func(t *testing.T) {
		// Constraint exists only in this test's isolated schema. Fail after one upsert.
		if _, err := store.DB.Exec(ctx, `ALTER TABLE account_unified_options ADD CONSTRAINT test_reject_value CHECK(value <> 4242)`); err != nil {
			t.Fatal(err)
		}
		values := []UnifiedOptionEntry{{Position: 2, Value: 99}, {Position: 3, Value: 4242}}
		if err := store.SaveAccountUnifiedOptions(ctx, account, values); err == nil {
			t.Fatal("expected the second entry to fail")
		}
		got, err := store.AccountUnifiedOptions(ctx, account)
		if err != nil || got[2] != 63 || len(got) != 1 {
			t.Fatalf("partial update escaped rollback: %v, %v", got, err)
		}
	})
}
