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

// 保底计数只有两种结果：不到阈值就 +1，到了阈值就（在通关时）归零。
// 这一层只负责把这件事做对且可重放 —— 「什么时候读」由 wireprobe 决定。
func TestOathProgressPity(t *testing.T) {
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
	schema := fmt.Sprintf("oath_progress_%d", time.Now().UnixNano())
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
	for _, migrate := range []func(context.Context) error{store.Migrate, store.MigrateOathProgress} {
		if err = migrate(ctx); err != nil {
			t.Fatal(err)
		}
	}
	account, err := store.DevelopmentAccount(ctx, "oath-progress-fixture")
	if err != nil {
		t.Fatal(err)
	}
	role, err := store.CreateCharacter(ctx, Character{
		AccountID:     account,
		Name:          "PityFixture",
		Profession:    0,
		ConfigVersion: strings.Repeat("ab", 32),
		State:         json.RawMessage(`{}`),
		Request:       []byte{0},
	}, 24)
	if err != nil {
		t.Fatal(err)
	}

	const dungeon = int64(100005014)
	// 刚建的角色还没有任何记录 —— 必须读成 0，而不是「找不到」。
	if clears, err := store.OathProgressClears(ctx, role.ID, dungeon); err != nil || clears != 0 {
		t.Fatalf("fresh character = %d err=%v, want 0", clears, err)
	}

	// needed = 2：第 1 场 0→1、第 2 场 1→2（到期）、第 3 场 2→0（兑现）。
	for i, step := range []struct{ before, after int }{{0, 1}, {1, 2}, {2, 0}, {0, 1}} {
		before, after, err := store.BumpOathProgress(ctx, role.ID, dungeon, 2)
		if err != nil {
			t.Fatal(err)
		}
		if before != step.before || after != step.after {
			t.Fatalf("bump %d = (%d,%d), want (%d,%d)", i, before, after, step.before, step.after)
		}
		clears, err := store.OathProgressClears(ctx, role.ID, dungeon)
		if err != nil || clears != step.after {
			t.Fatalf("read after bump %d = %d err=%v, want %d", i, clears, err, step.after)
		}
	}

	// 另一个副本必须独立计数，否则刷低难度的场次会替高难度攒保底。
	if clears, err := store.OathProgressClears(ctx, role.ID, 100005066); err != nil || clears != 0 {
		t.Fatalf("other dungeon = %d err=%v, want an independent 0", clears, err)
	}

	// 非法键一律拒绝，而不是写出一行垃圾。
	if _, _, err := store.BumpOathProgress(ctx, role.ID, dungeon, 0); err == nil {
		t.Fatal("needed=0 must be refused")
	}
	if _, _, err := store.BumpOathProgress(ctx, 0, dungeon, 5); err == nil {
		t.Fatal("character 0 must be refused")
	}
	if _, err := store.OathProgressClears(ctx, 0, dungeon); err == nil {
		t.Fatal("character 0 read must be refused")
	}
}
