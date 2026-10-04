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

// 征兆存档是**两列互相独立**的状态：held 在结算那一刻写，orthaire_pending 在通关
// 那一刻清。所以这个测试的重点不是「能存能读」，而是**写一列不会把另一列抹掉** ——
// 用「读整行、改一列、整行回写」的写法就会在这里挂掉，而且是在实机上以
// 「征兆无故归零」的形式挂掉。
func TestOmenStateColumnsAreIndependent(t *testing.T) {
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
	schema := fmt.Sprintf("omen_state_%d", time.Now().UnixNano())
	if _, err = admin.db.Exec(ctx, "CREATE SCHEMA "+schema); err != nil {
		t.Fatal(err)
	}
	defer admin.db.Exec(ctx, "DROP SCHEMA "+schema+" CASCADE")
	cfg.PostgresSchema = schema
	store, err := Open(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	for _, migrate := range []func(context.Context) error{store.Migrate, store.MigrateOmenState} {
		if err = migrate(ctx); err != nil {
			t.Fatal(err)
		}
	}
	account, err := store.DevelopmentAccount(ctx, "omen-state-fixture")
	if err != nil {
		t.Fatal(err)
	}
	role, err := store.CreateCharacter(ctx, Character{
		AccountID:     account,
		Name:          "OmenFixture",
		Profession:    0,
		ConfigVersion: strings.Repeat("ab", 32),
		State:         json.RawMessage(`{}`),
		Request:       []byte{0},
	}, 24)
	if err != nil {
		t.Fatal(err)
	}

	const dungeon = int64(100005014)
	// 刚建的角色还没有记录 —— 必须读成「没有征兆、也没有待出的隐藏 BOSS」，而不是报错。
	if st, err := store.OmenState(ctx, role.ID, dungeon); err != nil || st.Held != 0 || st.OrthaierPending {
		t.Fatalf("fresh character = %+v err=%v, want the zero value", st, err)
	}

	if err := store.SaveOmenHeld(ctx, role.ID, dungeon, 3); err != nil {
		t.Fatal(err)
	}
	// 置隐藏 BOSS 标记不能动 held。
	if err := store.SetOmenOrthaierPending(ctx, role.ID, dungeon, true); err != nil {
		t.Fatal(err)
	}
	if st, err := store.OmenState(ctx, role.ID, dungeon); err != nil || st.Held != 3 || !st.OrthaierPending {
		t.Fatalf("after pending = %+v err=%v, want held 3 with the flag set", st, err)
	}
	// 反过来也一样：结算归零不能顺手把还没兑现的隐藏 BOSS 抹掉。
	if err := store.SaveOmenHeld(ctx, role.ID, dungeon, 0); err != nil {
		t.Fatal(err)
	}
	if st, err := store.OmenState(ctx, role.ID, dungeon); err != nil || st.Held != 0 || !st.OrthaierPending {
		t.Fatalf("after reset = %+v err=%v, want held 0 with the flag still set", st, err)
	}
	// 通关兑现后才清。
	if err := store.SetOmenOrthaierPending(ctx, role.ID, dungeon, false); err != nil {
		t.Fatal(err)
	}
	if st, err := store.OmenState(ctx, role.ID, dungeon); err != nil || st.Held != 0 || st.OrthaierPending {
		t.Fatalf("after consume = %+v err=%v, want the zero value", st, err)
	}

	// 另一个副本必须各攒各的：两个深渊的征兆阶段表是各自副本挂的。
	if st, err := store.OmenState(ctx, role.ID, 100005067); err != nil || st.Held != 0 {
		t.Fatalf("other dungeon = %+v err=%v, want an independent 0", st, err)
	}

	// 非法键一律拒绝，而不是写出一行垃圾。
	if err := store.SaveOmenHeld(ctx, role.ID, dungeon, -1); err == nil {
		t.Fatal("negative held must be refused")
	}
	if err := store.SaveOmenHeld(ctx, 0, dungeon, 1); err == nil {
		t.Fatal("character 0 must be refused")
	}
	if err := store.SetOmenOrthaierPending(ctx, role.ID, 0, true); err == nil {
		t.Fatal("dungeon 0 must be refused")
	}
	if _, err := store.OmenState(ctx, role.ID, 0); err == nil {
		t.Fatal("dungeon 0 read must be refused")
	}
}
