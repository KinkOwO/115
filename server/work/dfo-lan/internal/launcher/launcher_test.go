package launcher

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The plan is the part worth testing without touching the machine: whether a SQLite
// profile tries to stop a PostgreSQL that does not exist, and whether a PostgreSQL
// profile still plans the graceful stop before the force fallback.
func TestStopPlanIsDriverAware(t *testing.T) {
	sqlitePlan, err := StopPlan(StorageConfig{Driver: "sqlite", SQLitePath: "save.sqlite3"})
	if err != nil {
		t.Fatalf("sqlite plan: %v", err)
	}
	if len(sqlitePlan) == 0 {
		t.Fatal("sqlite plan is empty; the server processes must still be stopped")
	}
	for _, action := range sqlitePlan {
		if strings.HasPrefix(action.Kind, "pg-") {
			t.Errorf("sqlite plan contains the PostgreSQL action %q (%s)", action.Kind, action.Detail)
		}
	}

	// A configuration that names no engine falls back to SQLite (2026-10-05 业主口径
	// 「默认 sqlite」), which has no service: no pg-* action may be planned for it.
	fallbackPlan, err := StopPlan(StorageConfig{})
	if err != nil {
		t.Fatalf("fallback plan: %v", err)
	}
	for _, action := range fallbackPlan {
		if strings.HasPrefix(action.Kind, "pg-") {
			t.Errorf("the empty configuration planned the PostgreSQL action %q (%s)", action.Kind, action.Detail)
		}
	}

	// PostgreSQL is selected by naming it (or a DSN), never by omission.
	postgresPlan, err := StopPlan(StorageConfig{Driver: "postgres"})
	if err != nil {
		t.Fatalf("postgres plan: %v", err)
	}
	kinds := map[string]int{}
	for _, action := range postgresPlan {
		kinds[action.Kind]++
	}
	if kinds["pg-force"] != 1 {
		t.Errorf("postgres plan has %d pg-force actions, want exactly 1 (the fallback)", kinds["pg-force"])
	}
	if kinds["kill"] == 0 {
		t.Error("postgres plan stops no server processes")
	}

	// With a real pg_ctl and data directory present, the graceful stop must be planned.
	bin := t.TempDir()
	data := filepath.Join(t.TempDir(), "pgdata")
	if err := os.MkdirAll(data, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(bin, "pg_ctl.exe"), []byte("stub"), 0o644); err != nil {
		t.Fatal(err)
	}
	withCtl, err := StopPlan(StorageConfig{Driver: "postgres", PostgresBin: bin, PostgresData: data})
	if err != nil {
		t.Fatalf("plan with pg_ctl: %v", err)
	}
	var graceful *Action
	for i := range withCtl {
		if withCtl[i].Kind == "pg-stop" {
			graceful = &withCtl[i]
		}
	}
	if graceful == nil {
		t.Fatal("a present pg_ctl and data directory did not plan a graceful stop")
	}
	if graceful.Path != data {
		t.Errorf("pg-stop carries data directory %q, want %q", graceful.Path, data)
	}

	// An unknown driver must be refused rather than silently treated as PostgreSQL.
	if _, err := StopPlan(StorageConfig{Driver: "mysql"}); err == nil {
		t.Error("an unknown driver was accepted")
	}
}

// A dry run must be genuinely side-effect free: it may not terminate anything, and it
// must still report the current port state rather than a prediction.
func TestStopDryRunChangesNothing(t *testing.T) {
	var logged []string
	report, err := Stop(context.Background(), StorageConfig{Driver: "sqlite"}, true,
		func(format string, args ...any) { logged = append(logged, format) })
	if err != nil {
		t.Fatalf("dry run: %v", err)
	}
	if !report.DryRun {
		t.Error("the report does not mark itself as a dry run")
	}
	if len(logged) == 0 {
		t.Error("a dry run logged nothing about what it would do")
	}
	for _, line := range logged {
		if !strings.HasPrefix(line, "would ") && !strings.Contains(line, "would ") {
			t.Errorf("dry run logged a non-conditional action: %q", line)
		}
	}
	if _, err := Stop(context.Background(), StorageConfig{Driver: "mysql"}, true, nil); err == nil {
		t.Error("a dry run accepted an unknown driver")
	}
}

// The config loader must accept the byte-order mark the existing files carry and treat
// a missing file as the empty configuration, whose driver is the shared SQLite fallback.
func TestLoadStorageConfig(t *testing.T) {
	root := t.TempDir()
	cfg, err := LoadStorageConfig(root)
	if err != nil {
		t.Fatalf("missing config: %v", err)
	}
	if cfg.DriverName() != "sqlite" {
		t.Errorf("missing config defaulted to %q, want sqlite (2026-10-05 default)", cfg.DriverName())
	}

	dir := filepath.Join(root, "server", "work", "dfo-lan", "runtime", "storage")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	body := "\ufeff{\"driver\":\"sqlite\",\"sqlite_path\":\"runtime/storage/x.sqlite3\"}"
	if err := os.WriteFile(filepath.Join(dir, "local.json"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err = LoadStorageConfig(root)
	if err != nil {
		t.Fatalf("sqlite config: %v", err)
	}
	if cfg.DriverName() != "sqlite" || cfg.SQLitePath == "" {
		t.Errorf("loaded %+v, want the sqlite profile", cfg)
	}
}

// Driver selection must match the server's engineForConfig key for key. The case that
// matters is the mixed configuration: PostgreSQL is running, so the server must not be
// reading a leftover SQLite file (2026-10-05, pgsql 端无法登录).
func TestDriverNameMatchesTheServerRule(t *testing.T) {
	cases := []struct {
		name string
		cfg  StorageConfig
		want string
	}{
		{name: "explicit sqlite", cfg: StorageConfig{Driver: "Sqlite "}, want: "sqlite"},
		{name: "explicit postgres", cfg: StorageConfig{Driver: "postgres"}, want: "postgres"},
		{name: "unknown driver is reported as written", cfg: StorageConfig{Driver: "mysql"}, want: "mysql"},
		{
			name: "a DSN outranks a leftover sqlite_path",
			cfg:  StorageConfig{PostgresDSN: "postgres://u@127.0.0.1:25438/dfo_lan", SQLitePath: "leftover.sqlite3"},
			want: "postgres",
		},
		{name: "DSN alone", cfg: StorageConfig{PostgresDSN: "postgres://u@127.0.0.1:25438/dfo_lan"}, want: "postgres"},
		{name: "sqlite_path alone", cfg: StorageConfig{SQLitePath: "save.sqlite3"}, want: "sqlite"},
		{name: "neither falls back to sqlite (2026-10-05 default)", cfg: StorageConfig{}, want: "sqlite"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.cfg.DriverName(); got != tc.want {
				t.Fatalf("DriverName() = %q, want %q (config %+v)", got, tc.want, tc.cfg)
			}
		})
	}

	// The mixed configuration must also produce the PostgreSQL stop plan: no pg-*
	// action was ever planned for a SQLite profile, which is how the disagreement
	// used to hide - the database was never stopped because the file needs no stop.
	plan, err := StopPlan(StorageConfig{PostgresDSN: "postgres://u@127.0.0.1:25438/dfo_lan", SQLitePath: "leftover.sqlite3"})
	if err != nil {
		t.Fatalf("mixed plan: %v", err)
	}
	found := false
	for _, action := range plan {
		if strings.HasPrefix(action.Kind, "pg-") {
			found = true
		}
	}
	if !found {
		t.Error("the mixed configuration planned no PostgreSQL action; it was treated as a SQLite profile")
	}
}
