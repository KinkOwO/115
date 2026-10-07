package database

import (
	"context"
	"path/filepath"
	"testing"
)

// The whole stack on SQLite: Store -> engine -> adapter -> database, with no
// PostgreSQL anywhere. This is what makes the PostgreSQL dependency removable, so it
// exercises the paths the server actually uses at startup, not just the query layer.
func TestOpenSQLiteStoreEndToEnd(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "store.sqlite3")
	store, err := Open(ctx, Config{Driver: DriverSQLite, SQLitePath: path, MaxConnections: 2})
	if err != nil {
		t.Fatalf("Open with the sqlite driver: %v", err)
	}
	defer store.Close()

	// The startup path calls these per-domain entry points unconditionally. On SQLite
	// the schema was applied by Open, so they must be honest no-ops rather than
	// attempts to run PostgreSQL DDL.
	for _, migrate := range []struct {
		name string
		fn   func(context.Context) error
	}{
		{"MigrateFatigue", store.MigrateFatigue},
		{"MigrateCharacterEvents", store.MigrateCharacterEvents},
		{"MigrateWorld", store.MigrateWorld},
	} {
		if err := migrate.fn(ctx); err != nil {
			t.Errorf("%s on the SQLite engine: %v", migrate.name, err)
		}
	}

	// Store-level operations, exactly as the server performs them.
	exists, err := store.NameExists(ctx, "Nobody")
	if err != nil {
		t.Fatalf("NameExists: %v", err)
	}
	if exists {
		t.Error("NameExists found a character in a fresh database")
	}

	account, err := store.DevelopmentAccount(ctx, "sqlite-store")
	if err != nil {
		t.Fatalf("DevelopmentAccount: %v", err)
	}
	if account == 0 {
		t.Fatal("DevelopmentAccount returned id 0")
	}
	// DevelopmentAccount is an INSERT ... ON CONFLICT ... RETURNING, so this already
	// proves a write reached SQLite through the adapter.

	rows, err := store.Characters(ctx, account)
	if err != nil {
		t.Fatalf("Characters: %v", err)
	}
	if len(rows) != 0 {
		t.Errorf("Characters = %d rows on a fresh account, want 0", len(rows))
	}

	// And the engine really is SQLite. SQLite is the only engine since 2026-10-05 (owner
	// decision, see root AGENTS.md §0.6), so this positive check replaces the old assertion
	// that the PostgreSQL-only pool accessor had to refuse.
	if _, ok := store.engine.(*sqliteEngine); !ok {
		t.Error("the store is not backed by the SQLite engine")
	}
}

// Opening the same database twice must be idempotent: the ledger records every section
// and a second open re-reads it instead of replaying DDL.
func TestOpenSQLiteStoreIsIdempotent(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "reopen.sqlite3")

	first, err := Open(ctx, Config{Driver: DriverSQLite, SQLitePath: path, MaxConnections: 2})
	if err != nil {
		t.Fatalf("first open: %v", err)
	}
	if _, err := first.DevelopmentAccount(ctx, "keeper"); err != nil {
		t.Fatalf("DevelopmentAccount: %v", err)
	}
	first.Close()

	second, err := Open(ctx, Config{Driver: DriverSQLite, SQLitePath: path, MaxConnections: 2})
	if err != nil {
		t.Fatalf("second open: %v", err)
	}
	defer second.Close()

	// The row written before the reopen must still be there, which proves the second
	// open reused the database instead of rebuilding it.
	rows, err := second.queries.Accounts(ctx)
	if err != nil {
		t.Fatalf("Accounts: %v", err)
	}
	if len(rows) != 1 || rows[0].Username != "keeper" {
		t.Errorf("Accounts after reopen = %+v, want the single keeper row", rows)
	}
}

// An unknown driver must fail loudly rather than silently falling back to PostgreSQL,
// because a typo in a storage config would otherwise connect to the wrong database.
func TestOpenRejectsUnknownDriver(t *testing.T) {
	if _, err := Open(context.Background(), Config{Driver: "mysql"}); err == nil {
		t.Fatal("Open accepted an unknown driver")
	}
}
