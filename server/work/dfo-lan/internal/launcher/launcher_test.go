package launcher

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The plan is the part worth testing without touching the machine: a SQLite profile
// stops the server processes and nothing else.
//
// PostgreSQL support was removed on 2026-10-05 (owner decision, see root AGENTS.md
// §0.6), so no pg-* action can exist any more — and the removed engine must now be
// refused exactly like any other unknown driver, not silently treated as SQLite.
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
			t.Errorf("sqlite plan contains the removed PostgreSQL action %q (%s)", action.Kind, action.Detail)
		}
	}

	// A configuration that names no engine falls back to SQLite (2026-10-05 业主口径
	// 「默认 sqlite」), which has no service: the plan is just the kills.
	fallbackPlan, err := StopPlan(StorageConfig{})
	if err != nil {
		t.Fatalf("fallback plan: %v", err)
	}
	for _, action := range fallbackPlan {
		if strings.HasPrefix(action.Kind, "pg-") {
			t.Errorf("the empty configuration planned the removed PostgreSQL action %q (%s)", action.Kind, action.Detail)
		}
	}

	// An unknown driver is refused rather than silently treated as SQLite.
	if _, err := StopPlan(StorageConfig{Driver: "mysql"}); err == nil {
		t.Error("an unknown driver was accepted")
	}
	// The removed engine is now just another unsupported driver.
	if _, err := StopPlan(StorageConfig{Driver: "postgres"}); err == nil {
		t.Error("the removed PostgreSQL driver was still accepted")
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
		{name: "a removed engine is still reported as written so the plans can refuse it", cfg: StorageConfig{Driver: "postgres"}, want: "postgres"},
		{name: "unknown driver is reported as written", cfg: StorageConfig{Driver: "mysql"}, want: "mysql"},
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

	// A profile that still carries the removed PostgreSQL keys (the 2026-10-04 upgrade
	// package wrote them) must load fine and be treated as SQLite: the leftover keys are
	// ignored, and no pg-* action can be planned for it.
	root := t.TempDir()
	dir := filepath.Join(root, "server", "work", "dfo-lan", "runtime", "storage")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	stale := `{"driver":"sqlite","sqlite_path":"save.sqlite3",` +
		`"postgres_dsn":"postgres://u@127.0.0.1:25438/dfo_lan","postgres_bin":"x","postgres_data":"y"}`
	if err := os.WriteFile(filepath.Join(dir, "local.json"), []byte(stale), 0o644); err != nil {
		t.Fatal(err)
	}
	loaded, err := LoadStorageConfig(root)
	if err != nil {
		t.Fatalf("a profile with leftover PostgreSQL keys must still load: %v", err)
	}
	if got := loaded.DriverName(); got != "sqlite" {
		t.Fatalf("DriverName() = %q, want sqlite (leftover PostgreSQL keys are ignored)", got)
	}
	plan, err := StopPlan(loaded)
	if err != nil {
		t.Fatalf("plan: %v", err)
	}
	for _, action := range plan {
		if strings.HasPrefix(action.Kind, "pg-") {
			t.Errorf("a sqlite profile planned the removed PostgreSQL action %q", action.Kind)
		}
	}
}
