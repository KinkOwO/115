// Package db 提供与驱动无关的持久化原语，避免 pgx 等具体驱动类型泄漏进领域接口。
//
// 领域在事务内只通过 Tx/Row/Result 执行语句；internal/storage 用适配器把 pgx.Tx
// 包成这里的 Tx。这样领域声明 Store 接口时不需要 import 任何数据库驱动。
package db

import "context"

// Result 是一次写操作的结果视图。
type Result interface {
	RowsAffected() int64
}

// Row 是单行查询的结果视图。
type Row interface {
	Scan(dest ...any) error
}

// Tx 是事务的中立视图。
type Tx interface {
	Exec(ctx context.Context, sql string, args ...any) (Result, error)
	QueryRow(ctx context.Context, sql string, args ...any) Row
}
