package database

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// testfixtures_test.go 收集**多个测试文件共用**的测试夹具。
//
// 它们原先住在 PostgreSQL 那两个测试文件里（cash_purchase_test.go / sqlc_postgres_test.go）。
// 那些文件随 PostgreSQL 支持一起删除（2026-10-05 业主口径，见根 AGENTS.md §0.6），但夹具
// 本身是引擎中立的，所以搬到这里保留，调用点不用改。

// cashFixture 是一张合法的 Cera 订单，用来钉住历史订单摘要输入与金额校验。
func cashFixture() CashOrder {
	return CashOrder{Key: "fixture-order-0001", Account: 1, Character: 1, Source: strings.Repeat("a", 64), Lines: []CashOrderLine{{Product: 3400489, Template: 590722921, Quantity: 1, Units: 1, UnitPrice: 3180}}}
}

// sqlcTestStore 返回一个建在隔离 SQLite 库上的 Store，以及一个带超时的 context。
//
// 它原先建的是一个临时 PostgreSQL schema。SQLite 是唯一引擎，隔离单位换成"每个测试
// 自己的库文件"：Open 会把全部 schema 分节一次应用完，所以返回的 store 直接可用。
func sqlcTestStore(t *testing.T) (*Store, context.Context) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	t.Cleanup(cancel)
	s, err := Open(ctx, Config{SQLitePath: filepath.Join(t.TempDir(), "sqlc-test.sqlite3")})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(s.Close)
	return s, ctx
}
