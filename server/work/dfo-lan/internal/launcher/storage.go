package launcher

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

// StartStoragePlan decides how to bring storage up, using the same plan/execute split as
// StopPlan so the decision is testable without starting anything.
//
// The SQLite driver needs no service at all: the engine opens the file itself, which is
// why the plan is empty rather than "start nothing and pretend it worked".
func StartStoragePlan(cfg StorageConfig) ([]Action, error) {
	switch cfg.DriverName() {
	case "sqlite":
		return nil, nil
	case "postgres":
	default:
		return nil, fmt.Errorf("unsupported storage driver %q", cfg.Driver)
	}
	if cfg.PostgresBin == "" || cfg.PostgresData == "" {
		return nil, fmt.Errorf("a PostgreSQL profile must name postgres_bin and postgres_data")
	}
	pgCtl := filepath.Join(cfg.PostgresBin, "pg_ctl.exe")
	if _, err := os.Stat(pgCtl); err != nil {
		return nil, fmt.Errorf("pg_ctl is missing at %s", pgCtl)
	}
	if _, err := os.Stat(filepath.Join(cfg.PostgresData, "PG_VERSION")); err != nil {
		return nil, fmt.Errorf("no PostgreSQL data directory at %s", cfg.PostgresData)
	}
	return []Action{{
		Kind:   "pg-start",
		Target: pgCtl,
		Path:   cfg.PostgresData,
		Detail: "pg_ctl start -D " + cfg.PostgresData + " -w -t 30",
	}}, nil
}

// StartStorage brings storage up if it is not already listening. It returns whether a
// service was started, so the caller can report honestly rather than assuming.
func StartStorage(ctx context.Context, cfg StorageConfig, timeout time.Duration, logf func(string, ...any)) (bool, error) {
	if logf == nil {
		logf = func(string, ...any) {}
	}
	plan, err := StartStoragePlan(cfg)
	if err != nil {
		return false, err
	}
	if len(plan) == 0 {
		logf("storage: sqlite profile, no service to start")
		return false, nil
	}
	// Already up is success: an externally managed server is not an error.
	if PortListening(PostgresPort, portProbeTimeout) {
		logf("storage: PostgreSQL already listening on %d", PostgresPort)
		return false, nil
	}
	for _, action := range plan {
		logf("storage: %s", action.Detail)
		if err := startPostgres(ctx, action, filepath.Join(filepath.Dir(action.Path), "postgres.log")); err != nil {
			return false, err
		}
	}
	// Waiting for the port is the only real proof the service came up, and it is what
	// the Python launcher checked too.
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if PortListening(PostgresPort, portProbeTimeout) {
			return true, nil
		}
		select {
		case <-ctx.Done():
			return false, ctx.Err()
		case <-time.After(250 * time.Millisecond):
		}
	}
	return false, fmt.Errorf("PostgreSQL did not start within %s; inspect launcher-postgres.log", timeout)
}

// startPostgres runs `pg_ctl start` **without ever reading its output through a pipe**.
//
// Why this is not `CombinedOutput()` (2026-10-05, live hang):
//
// On Windows, `pg_ctl start` launches postgres through a `cmd.exe` wrapper that stays alive
// as long as the server does, and that wrapper inherits pg_ctl's stdout/stderr. With
// `CombinedOutput()` those are *pipes we must read to EOF*, so the call blocks until postgres
// exits - i.e. forever. The observed symptom was exactly the owner's report: the console sat
// on "拉起 PostgreSQL" while `postgres.log` already said `database system is ready to accept
// connections`, and the only way out was Ctrl+C (which then killed the server through the
// console group and left a stale postmaster.pid behind).
//
// Giving the child *files* instead of pipes removes the deadlock: nothing of ours stays in the
// wrapper's handle table, so `Run` returns as soon as `pg_ctl -w` reports the server ready.
// Readiness is still proved the same way as before - by the port answering (below).
func startPostgres(ctx context.Context, action Action, logPath string) error {
	cmd := exec.CommandContext(ctx, action.Target,
		"start", "-D", action.Path, "-l", logPath, "-w", "-t", "30")
	logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return fmt.Errorf("open %s: %w", logPath, err)
	}
	defer logFile.Close()
	cmd.Stdout = logFile
	cmd.Stderr = logFile
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("pg_ctl start failed: %w (see %s)", err, logPath)
	}
	return nil
}
