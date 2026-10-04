package database

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// migrationEntryPoint matches the startup migration runners on Store.
var migrationEntryPoint = regexp.MustCompile(`(?ms)^func \(s \*Store\) (Migrate\w*)\(ctx context\.Context\) error \{(.*?)\n\}`)

// rawConnectionUse are the ways a runner could reach the driver directly instead of going
// through the engine-neutral seam.
var rawConnectionUse = regexp.MustCompile(`rawPool|s\.db|pool\.Exec|pool\.Query|DiagnosticExec|Begin\(ctx\)`)

// Startup migration entry points must stay engine-neutral. Every section runner goes
// through execMigration, which SQLite turns into a no-op because its schema is applied
// whole at Open; a new runner that reaches for the PostgreSQL pool directly would instead
// fail at SQLite startup, and only there - which is late and confusing.
//
// The one runner with a different signature, MigrateSaveIdentity, is a data migration and
// must pass by using generated queries rather than by being exempted.
func TestEveryMigrationEntryPointIsEngineNeutral(t *testing.T) {
	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatalf("glob: %v", err)
	}
	var checked int
	for _, path := range files {
		if strings.HasSuffix(path, "_test.go") {
			continue
		}
		source, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		for _, match := range migrationEntryPoint.FindAllStringSubmatch(string(source), -1) {
			name, body := match[1], match[2]
			checked++
			funnels := strings.Contains(body, "execMigration")
			usesQueries := strings.Contains(body, "s.queries.")
			if !funnels && !usesQueries {
				t.Errorf("%s (%s) neither funnels through execMigration nor uses generated queries; "+
					"on SQLite it would run PostgreSQL SQL at startup", name, filepath.Base(path))
				continue
			}
			if !funnels && rawConnectionUse.MatchString(body) {
				t.Errorf("%s (%s) reaches the driver directly: %s", name, filepath.Base(path),
					rawConnectionUse.FindString(body))
			}
		}
	}
	// A guard against the extraction silently matching nothing and passing vacuously.
	if checked < 30 {
		t.Fatalf("only %d migration entry points were found; the signature has changed", checked)
	}
}

// The data migration that carries a different signature must be covered explicitly: it is
// the one startup migration that is NOT a no-op on SQLite, so if it ever starts reaching
// for the pool, the save-identity normalisation would stop working there.
func TestSaveIdentityMigrationUsesGeneratedQueriesOnly(t *testing.T) {
	source, err := os.ReadFile("source_rebaseline.go")
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	// Matched by regex rather than by scanning for a brace, because this file's line
	// endings are CRLF.
	body := regexp.MustCompile(`(?ms)^func \(s \*Store\) MigrateSaveIdentity\(.*?\n\}`).
		FindString(string(source))
	if body == "" {
		t.Fatal("MigrateSaveIdentity not found; if it was renamed, this gate must follow it")
	}
	if hit := rawConnectionUse.FindString(body); hit != "" {
		t.Errorf("MigrateSaveIdentity reaches the driver directly (%s); the save-identity "+
			"normalisation would break on SQLite", hit)
	}
	if !strings.Contains(body, "s.queries.") {
		t.Error("MigrateSaveIdentity no longer uses generated queries")
	}
}
