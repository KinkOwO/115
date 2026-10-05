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
	case "launch":
		os.Exit(runLaunch(os.Args[2:]))
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
  dfolauncher launch --check|--dry-run [--root <path>]
                    [--server-only|--client-only|--storage-only]
                    [--json-mode|--repair-profile <path>] [--source-build]
  dfolauncher launch [--root <path>] [--tag <name>]
                    [--json-mode|--repair-profile <path>] [--source-build]

Flags:
  --root      repository root (default: the current directory, which is where the
              .cmd entry points cd to)
  --dry-run   print every action without performing it
  --tag       pin the session tag (default: built from the clock)

launch decides the session before anything starts: it reads the same configuration,
validates the same profile and checks the same dependencies as
scripts/launch_local.py, and --check prints the same four lines. --dry-run adds the
storage -> inner PVF -> gateway -> client command plan.

--server-only really starts the game gateway in Go (Stage 2 of
docs/go-launch-migration-plan.md): the protocol fixture, the gateway argv, ready.json
and run.json are reproduced from channel_probe.py, so no Python is involved.

interactive (the default) and --client-only additionally launch probe.exe with the same
argv, run.json, probe.json, exit-code warnings and client-trace handling as
channel_probe.py (Stage 3). DFO_ENABLE_OBSERVER is not implemented; it is a Python-only
observer and the launcher always injects 0.
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

// runLaunch is the launch subcommand. --check and --dry-run are Stage 1 (read-only);
// --server-only is Stage 2 and really starts the gateway in Go; interactive and
// --client-only are Stage 3: the same gateway session followed by the probe.exe handoff.
func runLaunch(args []string) int {
	flags := flag.NewFlagSet("launch", flag.ContinueOnError)
	root := flags.String("root", ".", "repository root")
	check := flags.Bool("check", false, "check every dependency and start nothing")
	dryRun := flags.Bool("dry-run", false, "print the command plan without running it")
	serverOnly := flags.Bool("server-only", false, "run storage and the game gateway without the client")
	clientOnly := flags.Bool("client-only", false, "run the client against a server elsewhere")
	storageOnly := flags.Bool("storage-only", false, "bring storage up and return")
	jsonMode := flags.Bool("json-mode", false, "explicit legacy JSON mode")
	sourceBuild := flags.Bool("source-build", false, "use bin/wireprobe-handoff-source.exe")
	repairProfile := flags.String("repair-profile", "", "override the default PVF profile")
	tag := flags.String("tag", "", "pin the session tag (default: from the clock)")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	if *jsonMode && *repairProfile != "" {
		// The Python declared these mutually exclusive, and the choice decides both the
		// binary and the data mode of the whole session.
		fmt.Fprintln(os.Stderr, "launch: --json-mode and --repair-profile are mutually exclusive")
		return 2
	}
	options := launcher.LaunchOptions{
		Check:         *check,
		DryRun:        *dryRun,
		ServerOnly:    *serverOnly,
		ClientOnly:    *clientOnly,
		StorageOnly:   *storageOnly,
		JSONMode:      *jsonMode,
		SourceBuild:   *sourceBuild,
		RepairProfile: *repairProfile,
		Tag:           *tag,
	}
	absolute, err := filepathAbs(*root)
	if err != nil {
		fmt.Fprintf(os.Stderr, "resolve root: %v\n", err)
		return 1
	}

	// A real run. server-only and storage-only start the gateway; everything else is the
	// interactive / client-only session, which also starts the gateway and then hands the
	// client to probe.exe (Stage 3).
	if !*check && !*dryRun {
		switch {
		case *serverOnly, *storageOnly:
			if err := launcher.LaunchServer(context.Background(), absolute, options, os.Stdout); err != nil {
				fmt.Fprintf(os.Stderr, "ERROR: %v\n", err)
				return 1
			}
			return 0
		default:
			if err := launcher.LaunchClient(context.Background(), absolute, options, os.Stdout); err != nil {
				fmt.Fprintf(os.Stderr, "ERROR: %v\n", err)
				return 1
			}
			return 0
		}
	}

	report, err := launcher.LaunchPlan(absolute, options)
	// The inner-PVF status is printed before the Python's fatal checks, so it is printed
	// here even when the plan then fails.
	if report.InnerPVF.Message != "" {
		fmt.Println(report.InnerPVF.Message)
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: %v\n", err)
		return 1
	}
	for _, line := range report.Lines(*dryRun) {
		fmt.Println(line)
	}
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
