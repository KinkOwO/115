package database

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"testing"
	"time"
)

// CommitCharacterEventTx 必须能工作：它的 apply 为 nil、txApply 非 nil。
//
// 回归点（2026-09-29 实测踩到）：commitCharacterEvent 里那个老的
// `apply == nil` 校验会把这个入口直接判成 "invalid character event"，
// 表现为游戏里买任何东西都弹「你的库存已经满了」。
func TestCommitCharacterEventTxAcceptsNilApply(t *testing.T) {
	if os.Getenv("DFO_TEST_POSTGRES_DSN") == "" {
		t.Skip("DFO_TEST_POSTGRES_DSN requires local PostgreSQL")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	cfg, err := loadPostgresTestConfig()
	if err != nil {
		t.Fatal(err)
	}
	admin, err := Open(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer admin.Close()
	schema := fmt.Sprintf("shop_limit_test_%d", time.Now().UnixNano())
	if _, err := admin.db.Exec(ctx, "CREATE SCHEMA "+schema); err != nil {
		t.Fatal(err)
	}
	defer admin.db.Exec(context.Background(), "DROP SCHEMA "+schema+" CASCADE")
	cfg.PostgresSchema = schema
	cfg.MaxConnections = 8
	s, err := Open(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	for _, m := range []func(context.Context) error{s.Migrate, s.MigrateCharacterEvents, s.MigrateShopPurchases} {
		if err := m(ctx); err != nil {
			t.Fatal(err)
		}
	}

	// 只是走到「校验参数」那一步就会暴露问题：apply 为 nil 时不该提前返回。
	// 用不存在的账号，预期失败于「查不到角色」而非「invalid character event」。
	_, _, e := s.CommitCharacterEventTx(ctx, 1, 1, "0000000000000000000000000000000000000000000000000000000000000000", "probe", "probe-model",
		func(tx *Tx, c Character) (json.RawMessage, json.RawMessage, error) {
			return json.RawMessage(`{}`), json.RawMessage(`{}`), nil
		})
	if e == nil {
		t.Fatal("不存在的角色竟然成功了")
	}
	t.Logf("按预期失败于角色查找：%v", e)
	if e.Error() == "invalid character event" {
		t.Fatalf("CommitCharacterEventTx 被老校验拦下：%v", e)
	}
}

// 不限购的商品，PeriodStart 必须给出零时间（不按周期过滤），
// 否则每次购买都会被当成「上期已买过」而误拒。
func TestPeriodStartZeroMeansNoWindow(t *testing.T) {
	now := time.Now()
	if got := PeriodStart("", now); !got.IsZero() {
		t.Fatalf("空周期应为零时间（不限窗口），得到 %v", got)
	}
	if got := PeriodStart("accumulate", now); !got.IsZero() {
		t.Fatalf("accumulate 应为零时间（永久累计），得到 %v", got)
	}
}
