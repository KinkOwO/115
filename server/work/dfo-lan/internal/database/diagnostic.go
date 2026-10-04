package database

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
)

// DiagnosticQuery is the explicit dbq exception to named SQL. PostgreSQL
// enforces a read-only transaction, and only column names and values cross
// this tool boundary. The dedicated connection is closed afterwards so a
// diagnostic cannot leave session settings or advisory locks in the pool.
func (s *Store) DiagnosticQuery(ctx context.Context, query string, consume func([]string, []any) error) error {
	if query == "" || consume == nil {
		return errors.New("diagnostic query and consumer required")
	}
	pool, err := s.rawPool()
	if err != nil {
		return err
	}
	conn, err := pool.Acquire(ctx)
	if err != nil {
		return err
	}
	defer conn.Release()
	defer conn.Conn().Close(context.Background())
	tx, err := conn.BeginTx(ctx, pgx.TxOptions{AccessMode: pgx.ReadOnly})
	if err != nil {
		return err
	}
	defer tx.Rollback(context.Background())
	rows, err := tx.Query(ctx, query)
	if err != nil {
		return err
	}
	defer rows.Close()
	descriptions := rows.FieldDescriptions()
	fields := make([]string, len(descriptions))
	for i, field := range descriptions {
		fields[i] = field.Name
	}
	for rows.Next() {
		values, err := rows.Values()
		if err != nil {
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
// 自己保证语句安全，供"建/删临时 schema、造一条畸形存档"这类测试准备使用。
func (s *Store) DiagnosticExec(ctx context.Context, statement string) error {
	if statement == "" {
		return errors.New("diagnostic statement required")
	}
	pool, err := s.rawPool()
	if err != nil {
		return err
	}
	_, err = pool.Exec(ctx, statement)
	return err
}
