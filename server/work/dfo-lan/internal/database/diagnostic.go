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
	conn, err := s.db.Acquire(ctx)
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
