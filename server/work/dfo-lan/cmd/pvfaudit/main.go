// pvfaudit compares PVF imports with JSON catalogs or audits native parser scope without
// starting the gateway, opening PostgreSQL, or changing client resources.
package main

import (
	"dfolan/internal/catalog"
	"dfolan/internal/catalog/pvf"
	"dfolan/internal/gamedata"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"runtime"
	"strings"
	"time"
)

type report struct {
	Archive    pvf.ArchiveSnapshot    `json:"archive"`
	Results    []gamedata.AuditResult `json:"results"`
	Equivalent bool                   `json:"equivalent"`
	DurationMS int64                  `json:"duration_ms"`
}

type selectionScopeReport struct {
	Archive         pvf.ArchiveSnapshot         `json:"archive"`
	SelectionScope  catalog.SelectionScopeAudit `json:"selection_scope"`
	StorageAccessed bool                        `json:"storage_accessed"`
	RuntimeStarted  bool                        `json:"runtime_started"`
	DurationMS      int64                       `json:"duration_ms"`
}

func main() { os.Exit(run(os.Args[1:], os.Stdout, os.Stderr)) }

func run(args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("pvfaudit", flag.ContinueOnError)
	flags.SetOutput(stderr)
	archive := flags.String("pvf-archive", "", "explicit read-only inner PVF path")
	checksum := flags.String("pvf-sha256", "", "expected inner PVF SHA256")
	maxBytes := flags.Int64("pvf-max-bytes", gamedata.DefaultMaxBytes, "maximum allowed archive size")
	domains := flags.String("domains", "characters,progression", "catalog domains to compare sequentially")
	limit := flags.Int("difference-limit", 100, "maximum field differences retained per domain; total count is not capped")
	selectionScope := flags.Bool("selection-scope", false, "audit all native selection-box scripts without a JSON baseline or template policy; difference-limit caps issue details")
	output := flags.String("output", "", "new report JSON path; empty writes to stdout; existing files are refused")
	paths := map[string]*string{
		"characters":  flags.String("character-catalog", "configs/characters.skycastle-release.json", "effective profession JSON baseline"),
		"world":       flags.String("world-catalog", "", "explicit historical world JSON for offline comparison, including side catalogs"),
		"quests":      flags.String("quest-catalog", "", "explicit historical quest JSON for offline comparison"),
		"progression": flags.String("progression-catalog", "configs/progression.next25.json", "effective progression JSON baseline"),
	}
	if err := flags.Parse(args); err != nil {
		return 1
	}
	if flags.NArg() != 0 || *limit < 0 {
		fmt.Fprintln(stderr, "unexpected arguments or negative difference limit")
		return 1
	}
	var selected []string
	var err error
	if !*selectionScope {
		selected, err = selectDomains(*domains, paths)
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		for _, domain := range selected {
			if *paths[domain] == "" {
				fmt.Fprintf(stderr, "%s requires an explicit historical baseline for offline comparison\n", domain)
				return 1
			}
		}
	} else {
		conflicting := false
		flags.Visit(func(f *flag.Flag) {
			if f.Name == "domains" || strings.HasSuffix(f.Name, "-catalog") {
				conflicting = true
			}
		})
		if conflicting {
			fmt.Fprintln(stderr, "selection-scope cannot be combined with JSON catalog comparison flags")
			return 1
		}
	}
	var target *os.File
	if *output != "" {
		// Output is opened only after the source has passed verification.
		if _, err = os.Stat(*output); err == nil {
			fmt.Fprintf(stderr, "refusing existing report: %s\n", *output)
			return 1
		}
		if !os.IsNotExist(err) {
			fmt.Fprintln(stderr, err)
			return 1
		}
	}
	started := time.Now()
	source, err := gamedata.Open(gamedata.Options{Mode: gamedata.PVF, ArchivePath: *archive, ExpectedChecksum: *checksum, MaxBytes: *maxBytes})
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	if *output != "" {
		target, err = os.OpenFile(*output, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		defer target.Close()
		stdout = target
	}
	defer source.Close()
	if *selectionScope {
		return writeSelectionScopeReport(source, *limit, started, stdout, stderr)
	}
	r := report{Archive: source.Snapshot(), Equivalent: true}
	for _, domain := range selected {
		fmt.Fprintf(stderr, "auditing %s against %s\n", domain, *paths[domain])
		result := gamedata.AuditCatalog(domain, *paths[domain], source, *limit)
		r.Results = append(r.Results, result)
		r.Equivalent = r.Equivalent && result.Equivalent
		fmt.Fprintf(stderr, "%s: equivalent=%t migration_compatible=%t differences=%d error=%s duration_ms=%d\n", domain, result.Equivalent, result.MigrationCompatible, result.Count, result.Error, result.DurationMS)
		source.ReleaseReadCaches()
		runtime.GC()
	}
	r.DurationMS = time.Since(started).Milliseconds()
	encoder := json.NewEncoder(stdout)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(r); err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	if target != nil {
		if err := target.Close(); err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
	}
	if !r.Equivalent {
		return 2
	}
	return 0
}

func writeSelectionScopeReport(source *gamedata.Source, limit int, started time.Time, stdout, stderr io.Writer) int {
	index, err := source.ItemIndex("")
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	audit, err := source.AuditSelectionScope(index, limit)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	r := selectionScopeReport{Archive: source.Snapshot(), SelectionScope: audit, DurationMS: time.Since(started).Milliseconds()}
	encoder := json.NewEncoder(stdout)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(r); err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	fmt.Fprintf(stderr, "native selection scope: candidates=%d parsed=%d fixed=%d unparsed=%d rejected=%d\n", audit.Candidates, audit.Parsed, audit.Fixed, audit.Unparsed, audit.Rejected)
	if audit.Rejected > 0 || audit.Unparsed > 0 {
		return 2
	}
	return 0
}

func selectDomains(value string, paths map[string]*string) ([]string, error) {
	var result []string
	seen := map[string]bool{}
	for _, domain := range strings.Split(value, ",") {
		domain = strings.TrimSpace(domain)
		if _, ok := paths[domain]; !ok {
			return nil, fmt.Errorf("unknown audit domain %q", domain)
		}
		if seen[domain] {
			return nil, fmt.Errorf("duplicate audit domain %q", domain)
		}
		seen[domain] = true
		result = append(result, domain)
	}
	return result, nil
}
