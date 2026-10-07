package database

import (
	"context"
	"path/filepath"
	"testing"
)

// Cross-validation with the 20261004 upgrade package: the two implementations are expected
// to share one tree and one local.json, so the points where they must agree are pinned
// here rather than left to a report.
//
// Agreed by construction after this change:
//   - engine selection: an explicit driver wins, and a configuration that names only
//     sqlite_path (what that package's migration tool writes) selects SQLite;
//   - option names: busy_timeout_ms / max_read_connections are accepted alongside this
//     implementation's sqlite_busy_timeout_ms / max_connections;
//   - save identity: a database created here carries the same PRAGMA application_id the
//     other backend requires, so it is accepted as a DFO save.
func TestSQLiteInteropWithUpgradePackageConfig(t *testing.T) {
	ctx := context.Background()

	// The package's shape: sqlite_path plus its own option names, and no driver field.
	path := filepath.Join(t.TempDir(), "game.db")
	store, err := Open(ctx, Config{
		SQLitePath:         path,
		BusyTimeoutMS:      7000,
		MaxReadConnections: 2,
	})
	if err != nil {
		t.Fatalf("a configuration in the upgrade package's shape was rejected: %v", err)
	}
	defer store.Close()

	// It really is the SQLite engine, not a fallback that happened to open.
	if _, ok := store.engine.(*sqliteEngine); !ok {
		t.Error("storage opened as something other than the SQLite engine; sqlite_path alone must select SQLite")
	}
	if _, err := store.DevelopmentAccount(ctx, "interop"); err != nil {
		t.Fatalf("the SQLite store is not usable: %v", err)
	}

	// The save must carry the identity the other backend checks for.
	var applicationID int64
	if err := store.engine.(*sqliteEngine).db.QueryRowContext(ctx, "PRAGMA application_id").Scan(&applicationID); err != nil {
		t.Fatalf("read application_id: %v", err)
	}
	if applicationID != sqliteSaveApplicationID {
		t.Errorf("application_id = %d, want %d (the upgrade package's DFO save id)",
			applicationID, sqliteSaveApplicationID)
	}

	// And an explicit driver still works, including this implementation's own option names.
	explicit := filepath.Join(t.TempDir(), "explicit.db")
	other, err := Open(ctx, Config{
		Driver:              DriverSQLite,
		SQLitePath:          explicit,
		SQLiteBusyTimeoutMS: 4000,
		MaxConnections:      3,
	})
	if err != nil {
		t.Fatalf("the explicit driver form was rejected: %v", err)
	}
	defer other.Close()
}

// A save that exists without the identity stamp must still open: the stamp is something
// this implementation adds, not something it demands, so a database produced by an earlier
// build or by hand is not locked out.
func TestSQLiteOpensASaveWithoutTheIdentityStamp(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "unstamped.db")

	// Built the low-level way, which is how a database written before the stamp existed
	// would look.
	db, err := openSQLite(ctx, path, 2, 5000)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	if err := migrateSQLiteAll(ctx, db); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	if err := db.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}

	store, err := Open(ctx, Config{SQLitePath: path})
	if err != nil {
		t.Fatalf("an unstamped save was refused: %v", err)
	}
	defer store.Close()
	if _, err := store.DevelopmentAccount(ctx, "unstamped"); err != nil {
		t.Fatalf("the unstamped save is not usable: %v", err)
	}
}
