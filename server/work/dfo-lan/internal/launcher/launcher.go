// Package launcher is the Go replacement for the Python orchestration scripts. It
// starts with the stop path: the smallest self-contained piece, which also establishes
// the shape the remaining subcommands (launch, server-only, configure, prepare-pvf)
// will follow.
//
// Planning is deliberately separate from execution. StopPlan is pure, so the decisions
// that matter - which processes to kill, whether a database service exists at all - are
// testable without terminating anything on the machine running the tests.
package launcher

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// StorageConfig is the part of runtime/storage/local.json the launcher needs.
type StorageConfig struct {
	Driver       string `json:"driver"`
	SQLitePath   string `json:"sqlite_path"`
	PostgresDSN  string `json:"postgres_dsn"`
	PostgresBin  string `json:"postgres_bin"`
	PostgresData string `json:"postgres_data"`
}

// DriverName reports the effective driver, mirroring the server's
// engineForConfig (internal/database): an explicit driver wins, a named DSN means
// PostgreSQL, and only a configuration that names nothing but sqlite_path is SQLite.
//
// The two rules must stay identical. When they drift, the launcher starts one engine while
// the server reads the other - a running PostgreSQL and an empty SQLite file, which reaches
// the player as "my account is gone" (2026-10-05, pgsql 端无法登录).
func (c StorageConfig) DriverName() string {
	if driver := strings.ToLower(strings.TrimSpace(c.Driver)); driver != "" {
		return driver
	}
	if strings.TrimSpace(c.PostgresDSN) != "" {
		return "postgres"
	}
	if strings.TrimSpace(c.SQLitePath) != "" {
		return "sqlite"
	}
	return "postgres"
}

// postgresImages are the server processes the launcher owns. They are killed by image
// name because the launcher no longer holds their process handles.
var serverImages = []string{
	"wireprobe-dungeon39.exe",
	"wireprobe-handoff-source.exe",
	"wireprobe-pvf.exe",
	"wireprobe-channel-identity-candidate.exe",
	"wireprobe.exe",
	"wireprobe-character.exe",
	"probe.exe",
	"probe-rebuilt.exe",
}

const (
	PostgresPort = 25438
	GatewayPort  = 7001

	// portProbeTimeout is short on purpose: a dependency check must not hang on a
	// port that is filtered rather than closed.
	portProbeTimeout = 500 * time.Millisecond
)

// Action is one planned step. Keeping them as data (rather than executing inline) is
// what lets --dry-run show exactly what a real run would do.
type Action struct {
	Kind   string // "kill" | "pg-stop" | "pg-force"
	Target string
	Path   string // pg-stop: the data directory to stop
	Detail string
}

// StopPlan decides what stopping this environment requires. It performs no side effects.
func StopPlan(cfg StorageConfig) ([]Action, error) {
	switch cfg.DriverName() {
	case "postgres":
	case "sqlite":
		// A sqlite profile has no service to stop: the database is a file the engine
		// opens and closes itself.
	default:
		return nil, fmt.Errorf("unsupported storage driver %q", cfg.Driver)
	}

	var plan []Action
	for _, image := range serverImages {
		plan = append(plan, Action{Kind: "kill", Target: image, Detail: "game server or probe process"})
	}
	if cfg.DriverName() != "postgres" {
		return plan, nil
	}

	if cfg.PostgresBin != "" && cfg.PostgresData != "" {
		pgCtl := filepath.Join(cfg.PostgresBin, "pg_ctl.exe")
		if _, err := os.Stat(pgCtl); err == nil {
			if _, err := os.Stat(cfg.PostgresData); err == nil {
				plan = append(plan, Action{
					Kind:   "pg-stop",
					Target: pgCtl,
					Path:   cfg.PostgresData,
					Detail: "pg_ctl stop -D " + cfg.PostgresData + " -m fast",
				})
			}
		}
	}
	// Needed whenever a server survives the graceful stop, including when the config
	// names no data directory at all.
	plan = append(plan, Action{
		Kind:   "pg-force",
		Target: "postgres.exe",
		Detail: fmt.Sprintf("only if port %d is still listening", PostgresPort),
	})
	return plan, nil
}

// StopReport records what actually happened, so the caller can print an honest summary.
type StopReport struct {
	Executed   []Action
	PostgresUp bool
	GatewayUp  bool
	DryRun     bool
}

// PortListening reports whether something accepts connections on the loopback port.
func PortListening(port int, timeout time.Duration) bool {
	conn, err := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", port), timeout)
	if err != nil {
		return false
	}
	_ = conn.Close()
	return true
}

// Stop executes the plan. With dryRun it only reports it.
func Stop(ctx context.Context, cfg StorageConfig, dryRun bool, logf func(string, ...any)) (StopReport, error) {
	report := StopReport{DryRun: dryRun}
	plan, err := StopPlan(cfg)
	if err != nil {
		return report, err
	}
	if logf == nil {
		logf = func(string, ...any) {}
	}

	for _, action := range plan {
		switch action.Kind {
		case "kill":
			if dryRun {
				logf("would stop %s (%s)", action.Target, action.Detail)
				continue
			}
			logf("stopping %s (%s)", action.Target, action.Detail)
			killByImage(ctx, action.Target)
		case "pg-stop":
			if dryRun {
				logf("would run %s", action.Detail)
				continue
			}
			logf("stopping PostgreSQL (fast checkpoint)")
			stopPostgres(ctx, action)
		case "pg-force":
			if PortListening(PostgresPort, 500*time.Millisecond) {
				if dryRun {
					logf("port %d is open: would force-terminate %s", PostgresPort, action.Target)
					continue
				}
				logf("port %d still open, force-terminating %s", PostgresPort, action.Target)
				killByImage(ctx, action.Target)
			}
		}
		report.Executed = append(report.Executed, action)
	}

	// Reported from the same probes the summary uses, so a dry run states current
	// reality rather than a prediction it cannot verify.
	report.PostgresUp = PortListening(PostgresPort, 500*time.Millisecond)
	report.GatewayUp = PortListening(GatewayPort, 500*time.Millisecond)
	return report, nil
}

func killByImage(ctx context.Context, image string) {
	cmd := exec.CommandContext(ctx, "taskkill", "/F", "/IM", image)
	// A process that is not running is not an error worth surfacing.
	_ = cmd.Run()
}

func stopPostgres(ctx context.Context, action Action) {
	cmd := exec.CommandContext(ctx, action.Target,
		"stop", "-D", action.Path, "-m", "fast", "-w", "-t", "15")
	_ = cmd.Run()
}

// LoadStorageConfig reads runtime/storage/local.json below root. A missing file yields
// the PostgreSQL default, matching the Python behaviour of continuing without config.
func LoadStorageConfig(root string) (StorageConfig, error) {
	path := filepath.Join(root, "server", "work", "dfo-lan", "runtime", "storage", "local.json")
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return StorageConfig{}, nil
		}
		return StorageConfig{}, err
	}
	var cfg StorageConfig
	if err := json.Unmarshal(stripBOM(data), &cfg); err != nil {
		return StorageConfig{}, fmt.Errorf("storage config %s: %w", path, err)
	}
	return cfg, nil
}

// stripBOM accepts the byte-order mark the existing configuration files carry.
func stripBOM(data []byte) []byte {
	return []byte(strings.TrimPrefix(string(data), "\ufeff"))
}
