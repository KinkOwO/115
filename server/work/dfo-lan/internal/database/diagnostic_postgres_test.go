package database

import (
	"errors"
	"fmt"
	"testing"
)

func TestSQLCDiagnosticIsReadOnlyAndPropagatesErrors(t *testing.T) {
	s, ctx := sqlcTestStore(t)
	if err := s.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	calls := 0
	if err := s.DiagnosticQuery(ctx, `SELECT 42::bigint AS answer`, func(fields []string, values []any) error {
		calls++
		if len(fields) != 1 || fields[0] != "answer" || values[0] != int64(42) {
			return fmt.Errorf("diagnostic values: %v %v", fields, values)
		}
		return nil
	}); err != nil || calls != 1 {
		t.Fatalf("read query: %d %v", calls, err)
	}
	for _, query := range []string{`INSERT INTO accounts(username) VALUES('must-not-write') RETURNING id`, `DELETE FROM accounts RETURNING id`, `CREATE TABLE diagnostic_must_not_exist(id bigint)`} {
		if err := s.DiagnosticQuery(ctx, query, func([]string, []any) error { return nil }); err == nil {
			t.Fatalf("diagnostic write accepted: %s", query)
		}
	}
	reject := errors.New("output failed")
	if err := s.DiagnosticQuery(ctx, `SELECT 1`, func([]string, []any) error { return reject }); !errors.Is(err, reject) {
		t.Fatal(err)
	}
	if err := s.DiagnosticQuery(ctx, `SELECT no_such_column`, func([]string, []any) error { return nil }); err == nil {
		t.Fatal("query error lost")
	}
	var count int
	if err := s.db.QueryRow(ctx, `SELECT count(*) FROM accounts`).Scan(&count); err != nil || count != 0 {
		t.Fatalf("diagnostic mutated accounts: %d %v", count, err)
	}
}
