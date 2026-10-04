package database

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

// Save compatibility is the highest-priority constraint in this repository, so the
// converter is verified end to end on the dedicated test cluster: existing PostgreSQL
// progress must arrive in SQLite intact, and the JSON check must actually run rather
// than pass vacuously.
func TestConvertPostgresToSQLite(t *testing.T) {
	if os.Getenv("DFO_TEST_POSTGRES_DSN") == "" {
		t.Skip("DFO_TEST_POSTGRES_DSN must explicitly select the dedicated test database")
	}
	ctx := context.Background()
	fixture, err := OpenTestFixture(ctx)
	if err != nil {
		t.Fatalf("open test fixture: %v", err)
	}
	defer fixture.Close()

	// The fixture embeds the store, so the converter reads exactly what a real
	// PostgreSQL deployment would serve.
	source := fixture.fixtureStore

	// The fixture isolates a schema but leaves migrating it to the caller.
	if err := fixture.Migrate(ctx); err != nil {
		t.Fatalf("migrate the fixture schema: %v", err)
	}

	account, err := fixture.DevelopmentAccount(ctx, "convert-me")
	if err != nil {
		t.Fatalf("seed account: %v", err)
	}
	// A JSON value with distinctive bytes: if the converter re-serialised JSON instead of
	// copying it, the byte comparison below is what notices.
	const seededJSON = `{"version":"convert-v1","hp":1234}`
	if _, err := testPool(t, source).Exec(ctx,
		`INSERT INTO characters(account_id, wire_id, name, profession, create_request, config_version, state, roster_order)
		 VALUES($1, 1, 'Converted', 1, '\x01'::bytea, 'cfg', $2::jsonb, 1)`,
		account, seededJSON); err != nil {
		t.Fatalf("seed character with JSON state: %v", err)
	}

	destPath := filepath.Join(t.TempDir(), "converted.sqlite3")
	report, err := ConvertPostgresToSQLite(ctx, source, destPath)
	if err != nil {
		t.Fatalf("convert: %v", err)
	}
	if report.TotalRows() == 0 {
		t.Fatal("the conversion copied no rows")
	}
	for _, table := range report.Tables {
		if !table.Verified {
			t.Errorf("%s: copied %d rows but reported unverified", table.Name, table.Rows)
		}
	}
	checked := 0
	for _, table := range report.Tables {
		checked += table.JSONChecked
	}
	if checked == 0 {
		t.Error("no JSON column was compared; the byte-level guarantee is untested")
	}

	// Read the result back through the server's own code path, not through raw SQL, so
	// the test proves the converted file is usable and not merely present.
	converted, err := Open(ctx, Config{Driver: DriverSQLite, SQLitePath: destPath, MaxConnections: 1})
	if err != nil {
		t.Fatalf("open the converted database: %v", err)
	}
	defer converted.Close()

	accounts, err := converted.queries.Accounts(ctx)
	if err != nil {
		t.Fatalf("Accounts on the converted database: %v", err)
	}
	found := false
	for _, row := range accounts {
		if row.Username == "convert-me" {
			found = true
		}
	}
	if !found {
		t.Errorf("the converted database does not contain the seeded account (%d accounts present)", len(accounts))
	}
}
