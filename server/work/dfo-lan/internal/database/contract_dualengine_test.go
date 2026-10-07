package database

import (
	"context"
	"path/filepath"
	"testing"
)

// The engine contract: behaviour the Store must show. It used to run one body against
// both engines, which is what turned "SQLite works in its own tests" into "the two
// engines are interchangeable". SQLite is the only engine since 2026-10-05 (owner
// decision, see root AGENTS.md §0.6), so the body now runs against it alone - the
// contract itself is unchanged and still worth pinning.
func TestEngineContract(t *testing.T) {
	cases := []struct {
		name string
		open func(t *testing.T) *Store
	}{
		{"sqlite", openContractSQLite},
	}
	for _, engineCase := range cases {
		t.Run(engineCase.name, func(t *testing.T) {
			runEngineContract(t, engineCase.open(t))
		})
	}
}

func openContractSQLite(t *testing.T) *Store {
	t.Helper()
	store, err := Open(context.Background(), Config{
		Driver:         DriverSQLite,
		SQLitePath:     filepath.Join(t.TempDir(), "contract.sqlite3"),
		MaxConnections: 2,
	})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(store.Close)
	return store
}

func runEngineContract(t *testing.T, store *Store) {
	ctx := context.Background()

	// --- identity and idempotency ----------------------------------------
	account, err := store.DevelopmentAccount(ctx, "contract-account")
	if err != nil {
		t.Fatalf("DevelopmentAccount: %v", err)
	}
	if account <= 0 {
		t.Fatalf("DevelopmentAccount returned %d", account)
	}
	again, err := store.DevelopmentAccount(ctx, "contract-account")
	if err != nil {
		t.Fatalf("DevelopmentAccount (repeat): %v", err)
	}
	if again != account {
		t.Errorf("DevelopmentAccount is not idempotent: %d then %d", account, again)
	}

	exists, err := store.NameExists(ctx, "contract-absent")
	if err != nil {
		t.Fatalf("NameExists: %v", err)
	}
	if exists {
		t.Error("NameExists reported a character that was never created")
	}
	characters, err := store.Characters(ctx, account)
	if err != nil {
		t.Fatalf("Characters: %v", err)
	}
	if len(characters) != 0 {
		t.Errorf("Characters = %d rows on a fresh account", len(characters))
	}

	// --- the engine-specific five, through the shared surface -------------
	name, err := store.queries.DatabaseName(ctx)
	if err != nil {
		t.Fatalf("DatabaseName: %v", err)
	}
	if name == "" {
		t.Error("DatabaseName is empty")
	}
	schema, err := store.queries.FixtureSchema(ctx)
	if err != nil {
		t.Fatalf("FixtureSchema: %v", err)
	}
	if schema == "" {
		t.Error("FixtureSchema is empty")
	}
	if err := store.queries.LockMigrations(ctx); err != nil {
		t.Errorf("LockMigrations: %v", err)
	}

	// Mail ids must advance monotonically and share one number space, which is the
	// invariant the PostgreSQL sequence and the SQLite high-water mark both provide.
	first, err := store.queries.NextMailID(ctx)
	if err != nil {
		t.Fatalf("NextMailID: %v", err)
	}
	second, err := store.queries.NextMailID(ctx)
	if err != nil {
		t.Fatalf("NextMailID (second): %v", err)
	}
	if second != first+1 {
		t.Errorf("NextMailID went %d -> %d; ids must advance by one", first, second)
	}

	// --- mutual exclusion, acquired and released -------------------------
	release, err := store.HoldAdminGuard(ctx)
	if err != nil {
		t.Fatalf("HoldAdminGuard: %v", err)
	}
	release()
	// After a release the guard must be acquirable again; a guard that leaks would wedge
	// administrative writes until the process restarted.
	releaseAgain, err := store.HoldAdminGuard(ctx)
	if err != nil {
		t.Fatalf("HoldAdminGuard after release: %v", err)
	}
	releaseAgain()

	// --- the startup path's migration entry points are safe to call -------
	for _, migrate := range []struct {
		name string
		fn   func(context.Context) error
	}{
		{"MigrateWorld", store.MigrateWorld},
		{"MigrateFatigue", store.MigrateFatigue},
	} {
		if err := migrate.fn(ctx); err != nil {
			t.Errorf("%s: %v", migrate.name, err)
		}
	}
}
