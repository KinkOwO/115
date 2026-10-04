package database

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// The SQLite guard is a lease file rather than an advisory lock, so its crash behaviour
// has to be proven rather than assumed: PostgreSQL releases the lock when the holding
// connection dies, and a lease must not be able to wedge administrative writes forever.
func TestSQLiteAdminLease(t *testing.T) {
	ctx := context.Background()
	dbPath := filepath.Join(t.TempDir(), "lease.sqlite3")
	db, err := openSQLite(ctx, dbPath, 2, 5000)
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	defer db.Close()
	if err := migrateSQLiteAll(ctx, db); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	holder := newSQLiteQueries(db)
	other := newSQLiteQueries(db)

	ok, err := holder.TrySharedAdminGuard(ctx)
	if err != nil || !ok {
		t.Fatalf("first acquire = %v, %v; want true", ok, err)
	}

	// A second administrator must be refused, and the error must name the file so an
	// operator can act on it.
	ok, refusal := other.TrySharedAdminGuard(ctx)
	if ok || refusal == nil {
		t.Fatalf("second acquire = %v, %v; want false plus an error", ok, refusal)
	}
	path, pathErr := holder.adminGuardPath(ctx)
	if pathErr != nil {
		t.Fatal(pathErr)
	}
	if !strings.Contains(refusal.Error(), filepath.Base(path)) {
		t.Errorf("error %q does not name the lease file", refusal)
	}

	// An abandoned lease (nobody refreshed it within the TTL) must be reclaimable: this is
	// the property that replaces PostgreSQL releasing the lock when its connection dies.
	stale := time.Now().Add(-2 * AdminLeaseTTL)
	if err := os.Chtimes(path, stale, stale); err != nil {
		t.Fatal(err)
	}
	ok, err = other.TrySharedAdminGuard(ctx)
	if err != nil || !ok {
		t.Fatalf("acquire after the lease went stale = %v, %v; want true", ok, err)
	}

	// A live holder keeps its lease fresh, so a refresh must reset the age well below the
	// TTL even after the file was backdated.
	if err := os.Chtimes(path, stale, stale); err != nil {
		t.Fatal(err)
	}
	if err := other.RefreshAdminGuardLease(ctx); err != nil {
		t.Fatalf("refresh: %v", err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if age := time.Since(info.ModTime()); age > AdminLeaseTTL/2 {
		t.Errorf("refreshed lease age = %s; the refresh did not take effect", age)
	}

	// Releasing frees the guard for the next holder, and refreshing after release must not
	// resurrect it.
	if err := other.ReleaseAdminGuardLease(ctx); err != nil {
		t.Fatalf("release: %v", err)
	}
	if err := other.RefreshAdminGuardLease(ctx); err != nil {
		t.Errorf("refresh after release should be a no-op: %v", err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Error("refreshing after release recreated the lease")
	}
	ok, err = holder.TrySharedAdminGuard(ctx)
	if err != nil || !ok {
		t.Errorf("acquire after release = %v, %v; want true", ok, err)
	}
}

// The engine's guard must release idempotently: the contract calls it once, but a caller
// that releases twice must not panic or hand the same pooled connection back twice.
func TestSQLiteEngineGuardReleaseIsIdempotent(t *testing.T) {
	ctx := context.Background()
	dbPath := filepath.Join(t.TempDir(), "guard.sqlite3")
	db, err := openSQLite(ctx, dbPath, 2, 5000)
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	defer db.Close()
	if err := migrateSQLiteAll(ctx, db); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	var eng engine = newSQLiteEngine(db)

	release, err := eng.holdAdminGuard(ctx)
	if err != nil {
		t.Fatalf("hold: %v", err)
	}
	release()
	release() // must not panic

	// And the guard must be acquirable again afterwards.
	release, err = eng.holdAdminGuard(ctx)
	if err != nil {
		t.Fatalf("re-hold after release: %v", err)
	}
	release()
}

