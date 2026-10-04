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

// The skill lock set is replaced as a whole under the character row lock, and a
// replayed frame key must not apply the same delta twice. The merge itself
// lives in the protocol package; this covers the durable half.
func TestSkillLockPersistence(t *testing.T) {
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
	schema := fmt.Sprintf("skill_locks_%d", time.Now().UnixNano())
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
	for _, migrate := range []func(context.Context) error{store.Migrate, store.MigrateCharacterEvents, store.MigrateSkillLocks} {
		if err = migrate(ctx); err != nil {
			t.Fatal(err)
		}
	}
	account, err := store.DevelopmentAccount(ctx, "skill-lock-fixture")
	if err != nil {
		t.Fatal(err)
	}
	role, err := store.CreateCharacter(ctx, Character{
		AccountID:     account,
		Name:          "LockFixture",
		Profession:    0,
		ConfigVersion: strings.Repeat("ab", 32),
		State:         json.RawMessage(`{}`),
		Request:       []byte{0},
	}, 24)
	if err != nil {
		t.Fatal(err)
	}
	apply := func(next ...uint16) func([]uint16) ([]uint16, error) {
		return func([]uint16) ([]uint16, error) { return next, nil }
	}

	locks, applied, err := store.CommitSkillLocks(ctx, account, role.ID, "frame-1", "skill-lock-v1", apply(58))
	if err != nil || !applied {
		t.Fatalf("first frame: locks=%v applied=%v err=%v", locks, applied, err)
	}
	if len(locks) != 1 || locks[0] != 58 {
		t.Fatalf("first frame locks = %v, want [58]", locks)
	}

	replay, applied, err := store.CommitSkillLocks(ctx, account, role.ID, "frame-1", "skill-lock-v1", apply(999))
	if err != nil || applied {
		t.Fatalf("replay applied=%v err=%v", applied, err)
	}
	if len(replay) != 1 || replay[0] != 58 {
		t.Fatalf("replay locks = %v, want the stored [58]", replay)
	}

	locks, applied, err = store.CommitSkillLocks(ctx, account, role.ID, "frame-2", "skill-lock-v1", apply(58, 517))
	if err != nil || !applied || len(locks) != 2 {
		t.Fatalf("second frame: locks=%v applied=%v err=%v", locks, applied, err)
	}

	// Replacing is not additive: an unlock frame drops the skills it omits.
	locks, _, err = store.CommitSkillLocks(ctx, account, role.ID, "frame-3", "skill-lock-v1", apply(517))
	if err != nil || len(locks) != 1 || locks[0] != 517 {
		t.Fatalf("replacement locks = %v err=%v", locks, err)
	}
	stored, err := store.SkillLocks(ctx, role.ID)
	if err != nil || len(stored) != 1 || stored[0] != 517 {
		t.Fatalf("stored locks = %v err=%v", stored, err)
	}

	for _, bad := range [][]uint16{{0}, {1024}, {58, 58}} {
		if _, _, err := store.CommitSkillLocks(ctx, account, role.ID, fmt.Sprintf("bad-%v", bad), "skill-lock-v1", apply(bad...)); err == nil {
			t.Fatalf("accepted invalid lock set %v", bad)
		}
	}
}
