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
//
// SQLite is the only engine since 2026-10-05 (owner decision, see root AGENTS.md
// §0.6), so the PostgreSQL fields (postgres_dsn / postgres_bin / postgres_data)
// are gone: a profile still carrying them parses, but they are ignored.
type StorageConfig struct {
	Driver     string `json:"driver"`
	SQLitePath string `json:"sqlite_path"`
}

// DriverName reports the effective driver. SQLite is the only engine, so an
// explicit driver is taken as written (the plan builders reject anything that is
// not sqlite) and an empty configuration means SQLite.
func (c StorageConfig) DriverName() string {
	if driver := strings.ToLower(strings.TrimSpace(c.Driver)); driver != "" {
		return driver
	}
	return "sqlite"
}

// serverImages are the server processes the launcher owns. They are killed by image
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
	GatewayPort = 7001

	// portProbeTimeout is short on purpose: a dependency check must not hang on a
	// port that is filtered rather than closed.
	portProbeTimeout = 500 * time.Millisecond
)

// Action is one planned step. Keeping them as data (rather than executing inline) is
// what lets --dry-run show exactly what a real run would do.
type Action struct {
	Kind   string // "kill" | "lease-clear"
	Target string
	Path   string // lease-clear: the lease file
	Detail string
}

// StopPlan decides what stopping this environment requires. It performs no side effects.
func StopPlan(cfg StorageConfig) ([]Action, error) {
	switch cfg.DriverName() {
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
	// Last, and only after the kills above: a forced stop leaves the SQLite admin
	// lease behind, and the next start is refused until its 60s TTL runs out. If a
	// lease is present, plan to clear it once we know its recorded holder is gone
	// (see adminlease.go - the executor re-checks at run time, because a clean
	// shutdown may already have removed it).
	if path, applicable := AdminLeasePath(cfg); applicable {
		if _, err := os.Stat(path); err == nil {
			plan = append(plan, Action{
				Kind:   "lease-clear",
				Path:   path,
				Detail: "clear the SQLite admin lease if its recorded holder is gone",
			})
		}
	}
	return plan, nil
}

// StopReport records what actually happened, so the caller can print an honest summary.
type StopReport struct {
	Executed  []Action
	GatewayUp bool
	DryRun    bool
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
		case "lease-clear":
			// Re-inspected here rather than trusting the plan: the server may have shut
			// down cleanly during the kills above and removed its own lease already.
			state, present, err := InspectAdminLease(cfg)
			if err != nil {
				logf("admin lease: %v", err)
				continue
			}
			if !present {
				continue
			}
			if dryRun {
				logf("would clear %s if holder pid %d is gone (lease age %s)", state.Path, state.Pid, state.Age.Round(time.Second))
				continue
			}
			if _, _, err := ClearStaleAdminLease(cfg, logf); err != nil {
				logf("admin lease: %v", err)
			}
		}
		report.Executed = append(report.Executed, action)
	}

	// Reported from the same probe the summary uses, so a dry run states current
	// reality rather than a prediction it cannot verify.
	report.GatewayUp = PortListening(GatewayPort, 500*time.Millisecond)
	return report, nil
}

func killByImage(ctx context.Context, image string) {
	cmd := exec.CommandContext(ctx, "taskkill", "/F", "/IM", image)
	// A process that is not running is not an error worth surfacing.
	_ = cmd.Run()
}

// LoadStorageConfig reads runtime/storage/local.json below root. A missing file yields an
// empty configuration, which DriverName reports as SQLite - the single fallback both this
// launcher and the server use (2026-10-05 业主口径「默认 sqlite」). The server then fails
// with an explicit "sqlite storage configuration incomplete" instead of opening a database
// nobody asked for.
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
