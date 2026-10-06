package database

import (
	"context"
	"database/sql"
	"strings"
)

// sqlite_schema.go 收集与 SQLite 自身 schema 打交道的小辅助函数。
//
// 它们原先住在 sqliteconvert.go（PostgreSQL → SQLite 单向量化器）里。那个转换器随
// PostgreSQL 支持一起删除（2026-10-05 业主口径，见根 AGENTS.md §0.6），但备份/还原
// 复核行数仍然要枚举表名与列名，所以这几个纯粹的 SQLite 辅助函数搬到这里保留。

// engineLocalTables holds bookkeeping that belongs to an engine rather than to the
// player's save. It used to matter when copying between two engines; with one engine it
// still marks the tables that are not part of the save (so row-count comparisons do not
// report the migration ledger as player data).
var engineLocalTables = map[string]bool{
	"storage_migrations": true,
}

// sqliteTableNames 列出库里的用户表（按名排序，跳过 SQLite 自己的 sqlite_* 表）。
func sqliteTableNames(ctx context.Context, db *sql.DB) ([]string, error) {
	rows, err := db.QueryContext(ctx,
		"SELECT name FROM sqlite_master WHERE type='table' AND name NOT LIKE 'sqlite_%' ORDER BY name")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var names []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		names = append(names, name)
	}
	return names, rows.Err()
}

// quoteIdent 把标识符包成 SQLite 接受的双引号形式（内部双引号翻倍）。
func quoteIdent(name string) string {
	return `"` + strings.ReplaceAll(name, `"`, `""`) + `"`
}
