package database

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// Schema-parity gate for the SQLite fork. The SQLite DDL is hand-maintained, so
// nothing but a mechanical comparison against the PostgreSQL tree can prove a
// table or column was not silently dropped while porting. These tests run as part
// of `go test ./...`, which is what keeps the fork honest as the schema grows.

var (
	createTablePattern = regexp.MustCompile(`(?is)CREATE\s+TABLE\s+(?:IF\s+NOT\s+EXISTS\s+)?(\w+)\s*\(`)
	createIndexPattern = regexp.MustCompile(`(?is)CREATE\s+(?:UNIQUE\s+)?INDEX\s+(?:IF\s+NOT\s+EXISTS\s+)?(\w+)`)
	constraintPattern  = regexp.MustCompile(`(?is)^\s*(PRIMARY|FOREIGN|UNIQUE|CHECK|CONSTRAINT)\b`)
	sectionNamePattern = regexp.MustCompile(`(?m)^-- migration: (\S+)\s*$`)
)

// stripSQLCommentsAndStrings removes comments and string literals so that paren
// and comma scanning cannot be confused by their contents.
func stripSQLCommentsAndStrings(text string) string {
	text = regexp.MustCompile(`(?s)/\*.*?\*/`).ReplaceAllString(text, "")
	text = regexp.MustCompile(`--[^\n]*`).ReplaceAllString(text, "")
	text = regexp.MustCompile(`'(?:[^']|'')*'`).ReplaceAllString(text, "''")
	return text
}

// splitTopLevel splits on commas that are not nested inside parentheses.
func splitTopLevel(body string) []string {
	var parts []string
	depth, start := 0, 0
	for i := 0; i < len(body); i++ {
		switch body[i] {
		case '(':
			depth++
		case ')':
			depth--
		case ',':
			if depth == 0 {
				parts = append(parts, body[start:i])
				start = i + 1
			}
		}
	}
	return append(parts, body[start:])
}

// parseDDL returns each table's column names and the set of index names.
func parseDDL(raw []byte) (map[string][]string, map[string]bool) {
	clean := stripSQLCommentsAndStrings(string(raw))
	tables := map[string][]string{}
	for _, m := range createTablePattern.FindAllStringSubmatchIndex(clean, -1) {
		table := strings.ToLower(clean[m[2]:m[3]])
		bodyStart, depth, i := m[1], 1, m[1]
		for ; i < len(clean) && depth > 0; i++ {
			switch clean[i] {
			case '(':
				depth++
			case ')':
				depth--
			}
		}
		if i-1 <= bodyStart {
			continue
		}
		if _, ok := tables[table]; !ok {
			tables[table] = nil
		}
		for _, part := range splitTopLevel(clean[bodyStart : i-1]) {
			// Table-level constraints are not columns. Matching on a word boundary
			// keeps PG's "CHECK(octet_length(...))" from looking like a column.
			if constraintPattern.MatchString(part) {
				continue
			}
			fields := strings.Fields(part)
			if len(fields) == 0 {
				continue
			}
			column := strings.ToLower(fields[0])
			if !containsString(tables[table], column) {
				tables[table] = append(tables[table], column)
			}
		}
	}
	indexes := map[string]bool{}
	for _, m := range createIndexPattern.FindAllStringSubmatch(clean, -1) {
		indexes[strings.ToLower(m[1])] = true
	}
	return tables, indexes
}

func containsString(list []string, want string) bool {
	for _, item := range list {
		if item == want {
			return true
		}
	}
	return false
}

func TestSQLiteSchemaCoversPostgres(t *testing.T) {
	pgRaw, err := fs.ReadFile(migrationSQL, initialMigrationFile)
	if err != nil {
		t.Fatalf("read PostgreSQL schema: %v", err)
	}
	liteRaw, err := fs.ReadFile(sqliteMigrationSQL, sqliteInitialMigrationFile)
	if err != nil {
		t.Fatalf("read SQLite schema: %v", err)
	}
	pgTables, pgIndexes := parseDDL(pgRaw)
	liteTables, liteIndexes := parseDDL(liteRaw)
	if len(pgTables) == 0 {
		t.Fatal("parsed no PostgreSQL tables; the parser or the schema layout changed")
	}

	var problems []string
	for table, columns := range pgTables {
		liteColumns, ok := liteTables[table]
		if !ok {
			problems = append(problems, "table missing from the SQLite fork: "+table)
			continue
		}
		for _, column := range columns {
			if !containsString(liteColumns, column) {
				problems = append(problems, fmt.Sprintf("%s: column missing from the SQLite fork: %s", table, column))
			}
		}
	}
	for table := range liteTables {
		if _, ok := pgTables[table]; !ok {
			problems = append(problems, "table exists only in the SQLite fork: "+table)
		}
	}
	for index := range pgIndexes {
		if !liteIndexes[index] {
			problems = append(problems, "index missing from the SQLite fork: "+index)
		}
	}
	if len(problems) > 0 {
		t.Fatalf("schema parity broken (%d problem(s)):\n  %s", len(problems), strings.Join(problems, "\n  "))
	}
	t.Logf("schema parity: %d tables and %d indexes mirrored", len(pgTables), len(pgIndexes))
}

// TestSQLiteMigrationSectionsCoverFile keeps the explicit section list and the
// migration file from drifting apart: a section added to the file but not to the
// list would never be applied, and the failure would only show up as a missing
// column at runtime.
func TestSQLiteMigrationSectionsCoverFile(t *testing.T) {
	raw, err := fs.ReadFile(sqliteMigrationSQL, sqliteInitialMigrationFile)
	if err != nil {
		t.Fatalf("read SQLite schema: %v", err)
	}
	var inFile []string
	for _, m := range sectionNamePattern.FindAllStringSubmatch(string(raw), -1) {
		if m[1] == "0000_migration_ledger.sql" {
			continue // applied as the ledger bootstrap, not through the list
		}
		inFile = append(inFile, m[1])
	}
	if len(inFile) != len(sqliteMigrationSections) {
		t.Fatalf("section list has %d entries but the file declares %d", len(sqliteMigrationSections), len(inFile))
	}
	for i, name := range inFile {
		if sqliteMigrationSections[i] != name {
			t.Errorf("section %d: list has %q, file declares %q", i, sqliteMigrationSections[i], name)
		}
	}
}

// TestSQLDialectFilesAreASCII enforces design note D23. sqlc v1.31.1's sqlite
// engine mis-parses a statement whose file contains a non-ASCII byte anywhere -
// including inside a comment - and reports single-character "extraneous input"
// errors that point nowhere near the real cause. Both dialect trees are ASCII
// today; this keeps it that way.
func TestSQLDialectFilesAreASCII(t *testing.T) {
	for _, dir := range []string{"sql/postgres", "sql/sqlite"} {
		err := filepath.WalkDir(dir, func(path string, entry fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if entry.IsDir() || filepath.Ext(path) != ".sql" {
				return nil
			}
			raw, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			for i, b := range raw {
				if b > 126 {
					return fmt.Errorf("%s: non-ASCII byte %#x at offset %d; keep SQL files ASCII (D23)", path, b, i)
				}
			}
			return nil
		})
		if err != nil {
			t.Error(err)
		}
	}
}
