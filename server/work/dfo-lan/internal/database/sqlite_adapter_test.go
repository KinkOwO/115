package database

import (
	"context"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"dfolan/internal/database/sqlcgen"
)

// End-to-end proof of the SQLite adapter: the same engine-neutral interface the
// Store persists through, exercised against a real SQLite database. Compiling is
// not enough here - the converters are generated, so their correctness is only
// visible when values actually cross.
func TestSQLiteAdapterThroughQuerySet(t *testing.T) {
	ctx := context.Background()
	dbPath := filepath.Join(t.TempDir(), "adapter.sqlite3")
	db, err := openSQLite(ctx, dbPath, 4, 5000)
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	defer db.Close()
	if err := migrateSQLiteAll(ctx, db); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	// Bind through the interface, as the Store will.
	var q querySet = newSQLiteQueries(db)

	// --- the five engine-specific methods --------------------------------
	name, err := q.DatabaseName(ctx)
	if err != nil {
		t.Fatalf("DatabaseName: %v", err)
	}
	if !strings.Contains(name, "adapter.sqlite3") {
		t.Errorf("DatabaseName = %q, want the database file", name)
	}
	if schema, err := q.FixtureSchema(ctx); err != nil || schema != "main" {
		t.Errorf("FixtureSchema = %q, %v; want main", schema, err)
	}
	if err := q.LockMigrations(ctx); err != nil {
		t.Errorf("LockMigrations should be a no-op on SQLite: %v", err)
	}
	first, err := q.NextMailID(ctx)
	if err != nil {
		t.Fatalf("NextMailID: %v", err)
	}
	second, err := q.NextMailID(ctx)
	if err != nil {
		t.Fatalf("NextMailID (second): %v", err)
	}
	if first != 1 || second != 2 {
		t.Errorf("NextMailID returned %d then %d; want 1 then 2 (ids must advance, sharing the message space)", first, second)
	}
	// The pre-allocated ids must be consumed by the next real insert, which is the
	// invariant the PostgreSQL sequence provided. Mail rows reference a character, so
	// create one first.
	mailAccount, err := q.DevelopmentAccount(ctx, "adapter-mail")
	if err != nil {
		t.Fatal(err)
	}
	mailCharacter, err := q.CreateCharacter(ctx, sqlcgen.CreateCharacterParams{
		AccountID: mailAccount, WireID: 1, Name: "Mailer", Profession: 1,
		CreateRequest: []byte{1}, ConfigVersion: "cfg",
		State: json.RawMessage(`{}`), RosterOrder: 1,
	})
	if err != nil {
		t.Fatalf("create character for mail: %v", err)
	}
	if _, err := db.ExecContext(ctx,
		"INSERT INTO character_mail(sender_id,recipient_id,sender_name,body,expires_at) VALUES(?,?,?,?,0)",
		mailCharacter.ID, mailCharacter.ID, "s", "b"); err != nil {
		t.Fatalf("insert mail: %v", err)
	}
	var gotID int64
	if err := db.QueryRowContext(ctx, "SELECT max(id) FROM character_mail").Scan(&gotID); err != nil {
		t.Fatal(err)
	}
	if gotID <= second {
		t.Errorf("message id %d did not continue after the pre-allocated %d", gotID, second)
	}

	// Shared guard: the second holder must be refused, and the error must name the
	// lease file so an operator can clear a crash left it behind.
	ok, err := q.TrySharedAdminGuard(ctx)
	if err != nil || !ok {
		t.Fatalf("first TrySharedAdminGuard = %v, %v; want true", ok, err)
	}
	if ok, err = newSQLiteQueries(db).TrySharedAdminGuard(ctx); ok || err == nil {
		t.Errorf("second TrySharedAdminGuard = %v, %v; want false plus a lease-naming error", ok, err)
	}

	// --- values crossing the generated converters ------------------------
	account, err := q.DevelopmentAccount(ctx, "adapter-suite")
	if err != nil {
		t.Fatalf("DevelopmentAccount: %v", err)
	}
	state := json.RawMessage(`{"level":7}`)
	// CreateCharacterParams carries json.RawMessage in the canonical shape while the
	// SQLite package uses []byte, so this exercises a generated converter.
	created, err := q.CreateCharacter(ctx, sqlcgen.CreateCharacterParams{
		AccountID: account, WireID: 1, Name: "Adapter", Profession: 1,
		CreateRequest: []byte{1}, ConfigVersion: "cfg", State: state, RosterOrder: 1,
	})
	if err != nil {
		t.Fatalf("CreateCharacter: %v", err)
	}
	rows, err := q.Characters(ctx, account)
	if err != nil {
		t.Fatalf("Characters: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("Characters = %d rows, want 1", len(rows))
	}
	if string(rows[0].State) != string(state) {
		t.Errorf("state round-trip through the converter = %s, want %s", rows[0].State, state)
	}
	if rows[0].ID != created.ID || rows[0].WireID != 1 {
		t.Errorf("row = id %d wire %d, want id %d wire 1", rows[0].ID, rows[0].WireID, created.ID)
	}
	allocation, err := q.CharacterAllocation(ctx, account)
	if err != nil {
		t.Fatalf("CharacterAllocation: %v", err)
	}
	// NextWireID is one of the documented divergences: PostgreSQL gives int32, the
	// SQLite expression is int64, so the converter must narrow it back.
	if allocation.ActiveCount != 1 || allocation.NextWireID != 2 {
		t.Errorf("allocation = %+v, want active 1 next wire 2", allocation)
	}
}
