package database

import (
	"context"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
)

// TestOmenMissesRoundTrip 保底计数（连续未触发次数）必须落库：它是角色存档的一部分，
// 重启归零会让玩家永远等不到那次保底。
//
// 这条同时是 0038_omen_pity.sql 的落地证明 —— 那个 ALTER 只在**新小节**里加列，
// 因为 migrateSQLite 按小节校验和记账，改 0030 会让既有存档直接拒绝打开
// （"migration 0030_omen_state.sql checksum differs"）。
func TestOmenMissesRoundTrip(t *testing.T) {
	ctx := context.Background()
	store, err := Open(ctx, Config{SQLitePath: filepath.Join(t.TempDir(), "omen-misses.db"), MaxConnections: 2})
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer store.Close()

	account, err := store.DevelopmentAccount(ctx, "omen-misses")
	if err != nil {
		t.Fatal(err)
	}
	role, err := store.CreateCharacter(ctx, Character{
		AccountID: account, Name: "OmenMissRole",
		ConfigVersion: strings.Repeat("b", 64), Request: []byte{0, 255},
		State: json.RawMessage(`{"unknown":{"keep":true},"gold":100}`),
	}, 4)
	if err != nil {
		t.Fatal(err)
	}

	const dungeon = 100005014 // 小深渊：唯一带 [coupon drop table] 的副本
	st, err := store.OmenState(ctx, role.ID, dungeon)
	if err != nil {
		t.Fatalf("fresh read: %v", err)
	}
	if st.Held != 0 || st.Misses != 0 || st.OrthaierPending {
		t.Fatalf("fresh state = %+v, want zero", st)
	}

	if err := store.SaveOmenMisses(ctx, role.ID, dungeon, 29); err != nil {
		t.Fatalf("SaveOmenMisses: %v", err)
	}
	if err := store.SaveOmenHeld(ctx, role.ID, dungeon, 2); err != nil {
		t.Fatalf("SaveOmenHeld: %v", err)
	}
	st, err = store.OmenState(ctx, role.ID, dungeon)
	if err != nil || st.Misses != 29 || st.Held != 2 {
		t.Fatalf("state = %+v, %v; want held=2 misses=29", st, err)
	}

	// 两列由不同时刻推进（结算那一刻写 held，每次没触发都写 misses），所以每个都是
	// **单列** upsert：写一个不能把另一个抹掉。
	if err := store.SaveOmenMisses(ctx, role.ID, dungeon, 0); err != nil {
		t.Fatalf("SaveOmenMisses(0): %v", err)
	}
	st, err = store.OmenState(ctx, role.ID, dungeon)
	if err != nil || st.Held != 2 || st.Misses != 0 {
		t.Fatalf("state = %+v, %v; 单列写入把 held 抹掉了", st, err)
	}
	if err := store.SaveOmenHeld(ctx, role.ID, dungeon, 1); err != nil {
		t.Fatalf("SaveOmenHeld(1): %v", err)
	}
	st, err = store.OmenState(ctx, role.ID, dungeon)
	if err != nil || st.Misses != 0 || st.Held != 1 {
		t.Fatalf("state = %+v, %v; 单列写入把 misses 抹掉了", st, err)
	}

	// 负数一律拒（列上有 CHECK(misses>=0)，仓储层先挡住，别依赖驱动的报错文案）。
	if err := store.SaveOmenMisses(ctx, role.ID, dungeon, -1); err == nil {
		t.Fatal("负数计数被接受了")
	}
}
