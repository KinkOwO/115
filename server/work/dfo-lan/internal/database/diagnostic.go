package database

import (
	"context"
	"database/sql"
	"errors"
)

// diagnostic.go 是「按名字的 SQL 之外唯一允许裸 SQL 的入口」：dfo-tool dbq 这个维护
// 工具靠它执行运维者手写的语句，并且只把列名与值递出去。
//
// PostgreSQL 时期这里走连接池 + 只读事务；PostgreSQL 支持已于 2026-10-05 移除
// （业主决定，见根 AGENTS.md §0.6），SQLite 是唯一引擎，所以直接走引擎自己的
// *sql.DB —— 库文件本身就是隔离单位，不需要专门的连接。

// sqliteDB 取出引擎的 SQLite 句柄。引擎只可能是 sqliteEngine（Open 保证）。
func (s *Store) sqliteDB() (*sql.DB, error) {
	if e, ok := s.engine.(*sqliteEngine); ok && e.db != nil {
		return e.db, nil
	}
	return nil, errors.New("diagnostic access requires the SQLite engine")
}

// DiagnosticQuery is the explicit dbq exception to named SQL. Only column names and
// values cross this tool boundary.
func (s *Store) DiagnosticQuery(ctx context.Context, query string, consume func([]string, []any) error) error {
	if query == "" || consume == nil {
		return errors.New("diagnostic query and consumer required")
	}
	db, err := s.sqliteDB()
	if err != nil {
		return err
	}
	// 只读事务：一条诊断语句不许改动存档。SQLite 的 query_only 让这一层在引擎内部
	// 也成立，而不是只靠调用方自觉。
	tx, err := db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return err
	}
	defer tx.Rollback()
	rows, err := tx.QueryContext(ctx, query)
	if err != nil {
		return err
	}
	defer rows.Close()
	fields, err := rows.Columns()
	if err != nil {
		return err
	}
	for rows.Next() {
		values := make([]any, len(fields))
		scan := make([]any, len(fields))
		for i := range values {
			scan[i] = &values[i]
		}
		if err := rows.Scan(scan...); err != nil {
			return err
		}
		if err := consume(fields, values); err != nil {
			return err
		}
	}
	return rows.Err()
}

// DiagnosticExec 是**给测试与维护脚本**用的显式写入口（`db` 字段不再导出后，
// 包外无法再直接 `store.DB.Exec`）。命名 SQL 仍是生产路径；这里只允许调用方
// 自己保证语句安全，供"造一条畸形存档"这类测试准备使用。
func (s *Store) DiagnosticExec(ctx context.Context, statement string) error {
	if statement == "" {
		return errors.New("diagnostic statement required")
	}
	db, err := s.sqliteDB()
	if err != nil {
		return err
	}
	_, err = db.ExecContext(ctx, statement)
	return err
}
