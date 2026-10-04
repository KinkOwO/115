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
		cmd := exec.CommandContext(ctx, action.Target,
			"start", "-D", action.Path, "-l", filepath.Join(filepath.Dir(action.Path), "postgres.log"),
			"-w", "-t", "30")
		if output, err := cmd.CombinedOutput(); err != nil {
			return false, fmt.Errorf("pg_ctl start failed: %v: %s", err, string(output))
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
