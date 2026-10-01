// pvfaudit compares PVF imports with the effective JSON catalogs without
// starting the gateway, opening PostgreSQL, or changing client resources.
package main

import (
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

func main() { os.Exit(run(os.Args[1:], os.Stdout, os.Stderr)) }

func run(args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("pvfaudit", flag.ContinueOnError)
	flags.SetOutput(stderr)
	archive := flags.String("pvf-archive", "", "explicit read-only inner PVF path")
	checksum := flags.String("pvf-sha256", "", "expected inner PVF SHA256")
	maxBytes := flags.Int64("pvf-max-bytes", gamedata.DefaultMaxBytes, "maximum allowed archive size")
	domains := flags.String("domains", "characters,world,quests,progression", "catalog domains to compare sequentially")
	limit := flags.Int("difference-limit", 100, "maximum field differences retained per domain; total count is not capped")
	output := flags.String("output", "", "new report JSON path; empty writes to stdout; existing files are refused")
	paths := map[string]*string{
		"characters":  flags.String("character-catalog", "configs/characters.skycastle-release.json", "effective profession JSON baseline"),
		"world":       flags.String("world-catalog", "configs/world.generated.json", "effective world JSON baseline including side catalogs"),
		"quests":      flags.String("quest-catalog", "configs/quests.generated.json", "effective quest JSON baseline"),
		"progression": flags.String("progression-catalog", "configs/progression.next25.json", "effective progression JSON baseline"),
	}
	if err := flags.Parse(args); err != nil {
		return 1
	}
	if flags.NArg() != 0 || *limit < 0 {
		fmt.Fprintln(stderr, "unexpected arguments or negative difference limit")
		return 1
	}
	selected, err := selectDomains(*domains, paths)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	var target *os.File
	if *output != "" {
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
