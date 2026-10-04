package database

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"
)

// The per-character read-notice ledger backs NOTI402/426: a seen mark must
// survive a reopen, "unseen" must delete the row so the popup can come back,
// and the two trees stay independent.
func TestCharacterNoticeSeenPersistence(t *testing.T) {
	if os.Getenv("DFO_TEST_POSTGRES_DSN") == "" {
		t.Skip("isolated schema integration")
	}
	ctx := context.Background()
	cfg, err := loadPostgresTestConfig()
	if err != nil {
		t.Fatal(err)
	}
	admin, err := Open(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer admin.Close()
	schema := fmt.Sprintf("char_notice_%d", time.Now().UnixNano())
	if _, err = testPool(t, admin).Exec(ctx, "CREATE SCHEMA "+schema); err != nil {
		t.Fatal(err)
	}
	defer testPool(t, admin).Exec(ctx, "DROP SCHEMA "+schema+" CASCADE")
	cfg.PostgresSchema = schema
	store, err := Open(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	for _, migrate := range []func(context.Context) error{store.Migrate, store.MigrateCharacterNotices} {
		if err = migrate(ctx); err != nil {
			t.Fatal(err)
		}
	}
	account, err := store.DevelopmentAccount(ctx, "notice-fixture")
	if err != nil {
		t.Fatal(err)
	}
	role, err := store.CreateCharacter(ctx, Character{
		AccountID:     account,
		Name:          "NoticeFixture",
		Profession:    0,
		ConfigVersion: strings.Repeat("ab", 32),
		State:         json.RawMessage(`{}`),
		Request:       []byte{0},
	}, 24)
	if err != nil {
		t.Fatal(err)
	}
	if err = store.MarkCharacterNotice(ctx, account, role.ID, 2, 62, true); err != nil {
		t.Fatal(err)
	}
	if err = store.MarkCharacterNotice(ctx, account, role.ID, 2, 1, true); err != nil {
		t.Fatal(err)
	}
	if err = store.MarkCharacterNotice(ctx, account, role.ID, 1, 7, true); err != nil {
		t.Fatal(err)
	}
	tree2, err := store.CharacterNoticeSeen(ctx, account, role.ID, 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(tree2) != 2 || tree2[0] != 1 || tree2[1] != 62 {
		t.Fatalf("tree2 = %v, want ascending [1 62]", tree2)
	}
	tree1, err := store.CharacterNoticeSeen(ctx, account, role.ID, 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(tree1) != 1 || tree1[0] != 7 {
		t.Fatalf("tree1 = %v, want [7]", tree1)
	}
	// seen=false removes the row, so the teaching frame can re-pop.
	if err = store.MarkCharacterNotice(ctx, account, role.ID, 2, 62, false); err != nil {
		t.Fatal(err)
	}
	tree2, err = store.CharacterNoticeSeen(ctx, account, role.ID, 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(tree2) != 1 || tree2[0] != 1 {
		t.Fatalf("tree2 after removal = %v, want [1]", tree2)
	}
}
