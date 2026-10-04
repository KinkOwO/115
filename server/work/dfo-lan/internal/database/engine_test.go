package database

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
)

// The engine seam's contract, proved on SQLite against the real driver: a callback
// that succeeds commits, a callback that fails rolls back, and both paths see the
// same querySet the Store uses. PostgreSQL satisfies the same interface through
// pgx.BeginFunc, so this is the behaviour both engines must provide.
func TestSQLiteEngineInTx(t *testing.T) {
	ctx := context.Background()
	dbPath := filepath.Join(t.TempDir(), "engine.sqlite3")
	db, err := openSQLite(ctx, dbPath, 4, 5000)
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := migrateSQLiteAll(ctx, db); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	var eng engine = newSQLiteEngine(db)
	defer eng.close()

	// Commit path: work done through the transaction's querySet survives.
	if err := inTx(ctx, eng, func(tx txHandle) error {
		q := tx.queries()
		_, err := q.DevelopmentAccount(ctx, "committed-account")
		return err
	}); err != nil {
		t.Fatalf("inTx (commit path): %v", err)
	}

	// Rollback path: an error from the callback must undo everything it did.
	sentinel := errors.New("deliberate failure")
	err = inTx(ctx, eng, func(tx txHandle) error {
		q := tx.queries()
		if _, err := q.DevelopmentAccount(ctx, "rolled-back-account"); err != nil {
			return err
		}
		// A second statement after the first, so the rollback covers more than one.
		if _, err := q.DevelopmentAccount(ctx, "rolled-back-account-2"); err != nil {
			return err
		}
		return sentinel
	})
	if !errors.Is(err, sentinel) {
		t.Fatalf("inTx returned %v, want the callback's own error", err)
	}

	// The committed account is visible; the rolled-back ones are not. This is read
	// outside the transaction on purpose, so it proves durability rather than
	// visibility within the transaction.
	rows, err := eng.queries().Accounts(ctx)
	if err != nil {
		t.Fatalf("Accounts: %v", err)
	}
	var names []string
	for _, row := range rows {
		names = append(names, row.Username)
	}
	if len(rows) != 1 || rows[0].Username != "committed-account" {
		t.Errorf("accounts after the two transactions = %v, want exactly [committed-account]", names)
	}
}

// The seam must also work for the five engine-specific methods when they run inside
// a transaction: NextMailID shares the caller's transaction instead of opening a
// nested one, which SQLite would reject.
func TestSQLiteEngineInTxWithEngineSpecificMethod(t *testing.T) {
	ctx := context.Background()
	dbPath := filepath.Join(t.TempDir(), "engine-tx.sqlite3")
	db, err := openSQLite(ctx, dbPath, 4, 5000)
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := migrateSQLiteAll(ctx, db); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	var eng engine = newSQLiteEngine(db)
	defer eng.close()

	var first int64
	if err := inTx(ctx, eng, func(tx txHandle) error {
		q := tx.queries()
		id, err := q.NextMailID(ctx)
		first = id
		return err
	}); err != nil {
		t.Fatalf("NextMailID inside a transaction: %v", err)
	}
	second, err := eng.queries().NextMailID(ctx)
	if err != nil {
		t.Fatalf("NextMailID outside a transaction: %v", err)
	}
	if first != 1 || second != 2 {
		t.Errorf("NextMailID = %d then %d; want 1 then 2 across both contexts", first, second)
	}
}
