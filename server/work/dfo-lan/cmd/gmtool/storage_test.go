package main

import (
	"encoding/json"
	"net"
	"os"
	"strings"
	"testing"
)

func writeFile(path, body string) error {
	return os.WriteFile(path, []byte(body), 0o644)
}

func TestStartStorageWithPostgresOnly(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	for _, legacy := range []string{"", `,"redis_address":"invalid","redis_bin":"missing","redis_password":"obsolete"`} {
		var cfg storageConfig
		input := `{"postgres_dsn":"postgres://user:pass@` + listener.Addr().String() + `/dfo_lan"` + legacy + `}`
		if err := json.Unmarshal([]byte(input), &cfg); err != nil {
			t.Fatal(err)
		}
		// No binaries or storage directories exist; an available PostgreSQL port is sufficient.
		note, err := startStorage(cfg, t.TempDir())
		if err != nil {
			t.Fatalf("startStorage: %v", err)
		}
		if note != "" {
			t.Fatalf("unexpected startup: %q", note)
		}
	}
}

// A SQLite profile has no service to start. Reporting the database file (instead of trying
// pg_ctl, which is not installed for a SQLite-only deployment) is what keeps the Web GM
// usable on both engines - it used to print a misleading "找不到 pg_ctl.exe" warning.
func TestStartStorageWithSQLiteProfileNeverTouchesPostgres(t *testing.T) {
	for _, cfg := range []storageConfig{
		{Driver: "sqlite", SQLitePath: "C:/saves/dfolan.sqlite3"},
		// The shape the 20261004 upgrade package's migration tool writes: no driver at all.
		{SQLitePath: "C:/saves/dfolan.sqlite3"},
	} {
		note, err := startStorage(cfg, t.TempDir())
		if err != nil {
			t.Fatalf("startStorage(%+v): %v", cfg, err)
		}
		if !strings.Contains(note, "SQLite") || !strings.Contains(note, cfg.SQLitePath) {
			t.Fatalf("startStorage(%+v) note = %q, want it to name the SQLite file", cfg, note)
		}
	}

	// A leftover sqlite_path must not hide the DSN: that configuration is PostgreSQL, and
	// starting the SQLite file instead is exactly the mixed-profile bug (pgsql 端无法登录).
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	note, err := startStorage(storageConfig{
		PostgresDSN: "postgres://user:pass@" + listener.Addr().String() + "/dfo_lan",
		SQLitePath:  "C:/saves/leftover.sqlite3",
	}, t.TempDir())
	if err != nil {
		t.Fatalf("mixed profile: %v", err)
	}
	if note != "" {
		t.Fatalf("mixed profile reported %q, want the PostgreSQL path (no startup needed)", note)
	}
}

// An incomplete or unknown profile must say what is wrong instead of starting nothing.
func TestStartStorageReportsIncompleteProfiles(t *testing.T) {
	if _, err := startStorage(storageConfig{Driver: "postgres"}, t.TempDir()); err == nil ||
		!strings.Contains(err.Error(), "postgres_dsn") {
		t.Fatalf("driver=postgres without a DSN: %v, want it to name postgres_dsn", err)
	}
	if _, err := startStorage(storageConfig{Driver: "sqlite"}, t.TempDir()); err == nil ||
		!strings.Contains(err.Error(), "sqlite_path") {
		t.Fatalf("driver=sqlite without a path: %v, want it to name sqlite_path", err)
	}
	if _, err := startStorage(storageConfig{Driver: "mysql"}, t.TempDir()); err == nil ||
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
