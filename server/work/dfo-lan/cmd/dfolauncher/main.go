// dfolauncher is the Go replacement for the Python orchestration scripts. It starts
// with "stop"; launch, server-only, configure and prepare-pvf follow the same shape, so
// the .cmd entry points can switch over one subcommand at a time and fall back to
// Python until each has been verified.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"dfolan/internal/launcher"
)

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}
	switch os.Args[1] {
	case "stop":
		os.Exit(runStop(os.Args[2:]))
	case "check":
		os.Exit(runCheck(os.Args[2:]))
	case "start-storage":
		os.Exit(runStartStorage(os.Args[2:]))
	case "-h", "--help", "help":
		usage()
		os.Exit(0)
	default:
		fmt.Fprintf(os.Stderr, "unknown subcommand %q\n\n", os.Args[1])
		usage()
		os.Exit(2)
	}
}

func usage() {
	fmt.Fprint(os.Stderr, `dfolauncher — DFO 115us orchestration

Usage:
  dfolauncher stop  [--root <path>] [--dry-run]
  dfolauncher start-storage [--root <path>] [--dry-run]
  dfolauncher check [--root <path>] [--server-only|--client-only] [--source-build] [--json-mode]

Flags:
  --root      repository root (default: the current directory, which is where the
              .cmd entry points cd to)
  --dry-run   print every action without performing it
`)
}

func runStop(args []string) int {
	flags := flag.NewFlagSet("stop", flag.ContinueOnError)
	root := flags.String("root", ".", "repository root")
	dryRun := flags.Bool("dry-run", false, "print the actions without performing them")
	if err := flags.Parse(args); err != nil {
		return 2
	}

	absolute, err := filepathAbs(*root)
	if err != nil {
		fmt.Fprintf(os.Stderr, "resolve root: %v\n", err)
		return 1
	}
	settings, err := launcher.LoadStorageConfig(absolute)
	if err != nil {
		fmt.Fprintf(os.Stderr, "storage config: %v\n", err)
		return 1
	}

	logf := func(format string, args ...any) { fmt.Printf(format+"\n", args...) }
	fmt.Println("=== Stopping DFO 115us Environment ===")
	if settings.DriverName() == "sqlite" {
		fmt.Printf("Storage: sqlite profile (%s), no PostgreSQL service to stop.\n", settings.SQLitePath)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	report, err := launcher.Stop(ctx, settings, *dryRun, logf)
	if err != nil {
		fmt.Fprintf(os.Stderr, "stop: %v\n", err)
		return 1
	}

	fmt.Println("Status summary:")
	fmt.Printf("  PostgreSQL (%d): %s\n", launcher.PostgresPort, state(report.PostgresUp, *dryRun))
	fmt.Printf("  Gateway    (%d):  %s\n", launcher.GatewayPort, state(report.GatewayUp, *dryRun))
	if *dryRun {
		fmt.Println("Dry run: nothing was stopped.")
		return 0
	}
	if !report.PostgresUp && !report.GatewayUp {
		fmt.Println("Environment fully stopped.")
		return 0
	}
	fmt.Println("Notice: some ports are still active.")
	return 1
}


// runCheck verifies the dependencies the selected mode needs and starts nothing, which
// is what makes it safe to run before a session.
func runCheck(args []string) int {
	flags := flag.NewFlagSet("check", flag.ContinueOnError)
	root := flags.String("root", ".", "repository root")
	serverOnly := flags.Bool("server-only", false, "verify only what a server-only run needs")
	clientOnly := flags.Bool("client-only", false, "verify only what a client-only run needs")
	sourceBuild := flags.Bool("source-build", false, "verify the source build binary")
	jsonMode := flags.Bool("json-mode", false, "verify the explicit JSON-mode binary")
	repairProfile := flags.String("repair-profile", "", "verify an explicit launcher profile")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	absolute, err := filepathAbs(*root)
	if err != nil {
		fmt.Fprintf(os.Stderr, "resolve root: %v\n", err)
		return 1
	}
	report, err := launcher.Check(absolute, launcher.CheckOptions{
		ServerOnly:  *serverOnly,
		ClientOnly:  *clientOnly,
		SourceBuild: *sourceBuild,
		JSONMode:      *jsonMode,
		RepairProfile: *repairProfile,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "check: %v\n", err)
		return 1
	}
	fmt.Printf("Client dir: %s\n", report.ClientDir)
	fmt.Printf("Binary:     %s\n", report.Binary)
	fmt.Printf("Data mode:  %s\n", report.DataMode)
	fmt.Printf("Storage:    %s\n", report.Storage)
	if report.ProfilePath != "" {
		fmt.Printf("Profile:    %s (%d environment entries)\n", report.ProfilePath, len(report.ProfileEnv))
	}
	for _, dependency := range report.Dependencies {
		state := "ok"
		if !dependency.Found {
			state = "MISSING"
		}
		fmt.Printf("  %-24s %-8s %s\n", dependency.Label, state, dependency.Path)
	}
	if missing := report.Missing(); len(missing) > 0 {
		fmt.Fprintf(os.Stderr, "%d required path(s) missing\n", len(missing))
		return 1
	}
	fmt.Println("Paths OK.")
	return 0
}

// runStartStorage brings storage up for the configured driver. It exists because the
// shipped launcher is a GUI with no CLI mode, so the development entries still need a
// Go path; see docs/runtime-without-tools-plan.md.
func runStartStorage(args []string) int {
	flags := flag.NewFlagSet("start-storage", flag.ContinueOnError)
	root := flags.String("root", ".", "repository root")
	dryRun := flags.Bool("dry-run", false, "print the actions without performing them")
	timeout := flags.Duration("timeout", 40*time.Second, "how long to wait for the service")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	absolute, err := filepathAbs(*root)
	if err != nil {
		fmt.Fprintf(os.Stderr, "resolve root: %v\n", err)
		return 1
	}
	settings, err := launcher.LoadStorageConfig(absolute)
	if err != nil {
		fmt.Fprintf(os.Stderr, "storage config: %v\n", err)
		return 1
	}
	logf := func(format string, args ...any) { fmt.Printf(format+"\n", args...) }
	if *dryRun {
		plan, err := launcher.StartStoragePlan(settings)
		if err != nil {
			fmt.Fprintf(os.Stderr, "start-storage: %v\n", err)
			return 1
		}
		if len(plan) == 0 {
			logf("storage: sqlite profile, nothing to start")
			return 0
		}
		for _, action := range plan {
			logf("would run %s", action.Detail)
		}
		return 0
	}
	ctx, cancel := context.WithTimeout(context.Background(), *timeout+15*time.Second)
	defer cancel()
	if _, err := launcher.StartStorage(ctx, settings, *timeout, logf); err != nil {
		fmt.Fprintf(os.Stderr, "start-storage: %v\n", err)
		return 1
	}
	return 0
}

func state(up, dryRun bool) string {
	switch {
	case !up:
		return "Stopped"
	case dryRun:
		return "ACTIVE"
	default:
		return "ACTIVE (warning)"
	}
}

func filepathAbs(path string) (string, error) {
	if path == "" {
		path = "."
	}
	return filepath.Abs(path)
}
