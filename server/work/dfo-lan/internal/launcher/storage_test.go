package launcher

import (
	"os"
	"path/filepath"
	"testing"
)

// The SQLite driver has no service, so the plan must be empty rather than a no-op that
// pretends something was started.
func TestStartStoragePlanForSQLiteIsEmpty(t *testing.T) {
	plan, err := StartStoragePlan(StorageConfig{Driver: "sqlite", SQLitePath: "s.sqlite3"})
	if err != nil {
		t.Fatalf("sqlite plan: %v", err)
	}
	if len(plan) != 0 {
		t.Errorf("sqlite plan = %v, want nothing to start", plan)
	}
	if _, err := StartStoragePlan(StorageConfig{Driver: "mysql"}); err == nil {
		t.Error("an unknown driver was accepted")
	}
}

func TestStartStoragePlanForPostgres(t *testing.T) {
	// A PostgreSQL profile must name its engine (or carry a DSN); the shared fallback for a
	// configuration that names nothing is SQLite since 2026-10-05.
	// Incomplete configuration must be refused with a message that names what is missing.
	if _, err := StartStoragePlan(StorageConfig{Driver: "postgres"}); err == nil {
		t.Error("a PostgreSQL profile without postgres_bin/postgres_data was accepted")
	}
	if _, err := StartStoragePlan(StorageConfig{Driver: "postgres", PostgresBin: "x", PostgresData: "y"}); err == nil {
		t.Error("a profile naming a missing pg_ctl was accepted")
	}
	// The fallback is the SQLite no-op, not an error: there is no service to start.
	if plan, err := StartStoragePlan(StorageConfig{}); err != nil || len(plan) != 0 {
		t.Errorf("empty configuration planned %v, %v; want the sqlite no-op", plan, err)
	}

	bin := t.TempDir()
	data := t.TempDir()
	if err := os.WriteFile(filepath.Join(bin, "pg_ctl.exe"), []byte("stub"), 0o644); err != nil {
		t.Fatal(err)
	}
	// pg_ctl alone is not enough: an uninitialised data directory must be refused before
	// anything is started.
	if _, err := StartStoragePlan(StorageConfig{Driver: "postgres", PostgresBin: bin, PostgresData: data}); err == nil {
		t.Error("a data directory without PG_VERSION was accepted")
	}
	if err := os.WriteFile(filepath.Join(data, "PG_VERSION"), []byte("16"), 0o644); err != nil {
		t.Fatal(err)
	}
	plan, err := StartStoragePlan(StorageConfig{Driver: "postgres", PostgresBin: bin, PostgresData: data})
	if err != nil {
		t.Fatalf("plan: %v", err)
	}
	if len(plan) != 1 || plan[0].Kind != "pg-start" {
		t.Fatalf("plan = %v, want one pg-start action", plan)
	}
	if plan[0].Path != data {
		t.Errorf("pg-start carries %q, want the data directory %q", plan[0].Path, data)
	}
}

// Starting storage for a sqlite profile must do nothing at all: no service, no error.
func TestStartStorageSQLiteIsANoOp(t *testing.T) {
	var logged []string
	started, err := StartStorage(t.Context(), StorageConfig{Driver: "sqlite"}, 0,
		func(format string, args ...any) { logged = append(logged, format) })
	if err != nil {
		t.Fatalf("start storage: %v", err)
	}
	if started {
		t.Error("the sqlite profile reported that it started a service")
	}
	if len(logged) != 1 {
		t.Errorf("logged %v, want a single statement that there is nothing to start", logged)
	}
	if _, err := StartStorage(t.Context(), StorageConfig{Driver: "mysql"}, 0, nil); err == nil {
		t.Error("an unknown driver was accepted")
	}
}
