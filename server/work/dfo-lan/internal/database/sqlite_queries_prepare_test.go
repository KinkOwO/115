package database

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// sqlConstant matches the statements sqlc generates: a raw string starting with the
// "-- name: X :kind" header it uses for provenance. The trailing backtick is the only
// anchor needed, which keeps this independent of the file's line endings.
var sqlConstant = regexp.MustCompile("(?ms)^const (\\w+) = `(.*?)`")

// Every ported statement must at least parse and plan against the SQLite schema. This is
// the failure mode compilation cannot see: a column typo, a table the port forgot, or
// syntax the SQLite grammar handles differently. Such a statement otherwise fails only
// when a player happens to trigger that code path, which is when it costs the most.
func TestSQLiteQueryTreePreparesAgainstSchema(t *testing.T) {
	ctx := context.Background()
	db, err := openSQLite(ctx, filepath.Join(t.TempDir(), "prepare.sqlite3"), 2, 5000)
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	defer db.Close()
	if err := migrateSQLiteAll(ctx, db); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	files, err := filepath.Glob(filepath.Join("sqlcgensqlite", "*.go"))
	if err != nil {
		t.Fatalf("glob: %v", err)
	}
	if len(files) == 0 {
		t.Fatal("no generated SQLite sources found; the test is looking in the wrong place")
	}

	var checked int
	var failures []string
	for _, path := range files {
		if strings.HasSuffix(path, "_test.go") {
			continue
		}
		source, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		for _, match := range sqlConstant.FindAllStringSubmatch(string(source), -1) {
			name, statement := match[1], match[2]
			// The first line is sqlc's provenance header; everything after it is the SQL.
			if head, rest, found := strings.Cut(statement, "\n"); found && strings.HasPrefix(head, "-- name:") {
				statement = rest
			}
			checked++
			if _, err := db.PrepareContext(ctx, statement); err != nil {
				failures = append(failures, fmt.Sprintf("%s (%s): %v", name, filepath.Base(path), err))
			}
		}
	}

	// Guard against the extraction silently finding nothing and passing vacuously.
	if checked < 200 {
		t.Fatalf("only %d statements were extracted; the generated format has changed", checked)
	}
	if len(failures) > 0 {
		limit := len(failures)
		if limit > 20 {
			limit = 20
		}
		t.Errorf("%d of %d statements do not prepare against the SQLite schema:\n  %s",
			len(failures), checked, strings.Join(failures[:limit], "\n  "))
	}
}
