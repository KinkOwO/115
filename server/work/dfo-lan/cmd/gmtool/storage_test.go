package main

import (
	"os"
	"strings"
	"testing"
)

func writeFile(path, body string) error {
	return os.WriteFile(path, []byte(body), 0o644)
}

// A SQLite profile has no service to start. Reporting the database file (instead of trying
// pg_ctl, which does not exist for a SQLite-only deployment) is what keeps the Web GM
// usable - it used to print a misleading "找不到 pg_ctl.exe" warning.
func TestStartStorageNamesTheSQLiteFile(t *testing.T) {
	for _, cfg := range []storageConfig{
		{Driver: "sqlite", SQLitePath: "C:/saves/dfolan.sqlite3"},
		// The shape the 20261004 upgrade package's migration tool writes: no driver at all.
		{SQLitePath: "C:/saves/dfolan.sqlite3"},
	} {
		note, err := startStorage(cfg)
		if err != nil {
			t.Fatalf("startStorage(%+v): %v", cfg, err)
		}
		if !strings.Contains(note, "SQLite") || !strings.Contains(note, cfg.SQLitePath) {
			t.Fatalf("startStorage(%+v) note = %q, want it to name the SQLite file", cfg, note)
		}
	}
}

// The removed engine must be refused, and the refusal must say so: silently opening the
// leftover sqlite_path instead is exactly the mixed-profile bug that reaches the player as
// "my save is gone". PostgreSQL support was removed on 2026-10-05 (root AGENTS.md §0.6).
func TestStartStorageRefusesTheRemovedEngineEvenWithASQLitePath(t *testing.T) {
	_, err := startStorage(storageConfig{Driver: "postgres", SQLitePath: "C:/saves/leftover.sqlite3"})
	if err == nil || !strings.Contains(err.Error(), "removed") {
		t.Fatalf("driver=postgres with a leftover path: %v, want the removal notice", err)
	}
}

// An incomplete or unknown profile must say what is wrong instead of starting nothing.
func TestStartStorageReportsIncompleteProfiles(t *testing.T) {
	if _, err := startStorage(storageConfig{Driver: "postgres"}); err == nil ||
		!strings.Contains(err.Error(), "PostgreSQL support was removed") {
		t.Fatalf("driver=postgres: %v, want the removal notice", err)
	}
	if _, err := startStorage(storageConfig{Driver: "sqlite"}); err == nil ||
		!strings.Contains(err.Error(), "sqlite_path") {
		t.Fatalf("driver=sqlite without a path: %v, want it to name sqlite_path", err)
	}
	if _, err := startStorage(storageConfig{Driver: "mysql"}); err == nil ||
		!strings.Contains(err.Error(), "mysql") {
		t.Fatalf("unknown driver: %v, want it to name the driver", err)
	}
}

// BOM 容错：Windows 编辑器写的 BOM 曾让服务端在这个文件上启动失败（database.LoadConfig），
// GM 侧读同一份文件，不能再分道扬镳。
func TestLoadStorageConfigToleratesTheByteOrderMark(t *testing.T) {
	path := t.TempDir() + "/local.json"
	body := "\ufeff{\"driver\":\"sqlite\",\"sqlite_path\":\"C:/saves/dfolan.sqlite3\"}"
	if err := writeFile(path, body); err != nil {
		t.Fatal(err)
	}
	cfg, err := loadStorageConfig(path)
	if err != nil {
		t.Fatalf("loadStorageConfig: %v", err)
	}
	if driver, err := cfg.engine(); err != nil || driver != "sqlite" {
		t.Fatalf("engine = %q (%v), want sqlite", driver, err)
	}
}
