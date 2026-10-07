package database

import (
	"context"
	"encoding/json"
	"io/fs"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"dfolan/internal/database/sqlcgensqlite"
)

// End-to-end proof of the SQLite core slice (design doc S3a/S4): real driver, real
// DSN parameters, real generated queries. Each assertion maps to a claim the plan
// makes; if one fails the plan is wrong, not the test.
func TestSQLiteCoreSliceRoundTrip(t *testing.T) {
	ctx := context.Background()
	db, err := openSQLite(ctx, filepath.Join(t.TempDir(), "core.sqlite3"), 4, 5000)
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	defer db.Close()

	var version string
	if err := db.QueryRowContext(ctx, "SELECT sqlite_version()").Scan(&version); err != nil {
		t.Fatalf("sqlite_version: %v", err)
	}
	t.Logf("sqlite_version = %s", version)
	parts := strings.Split(version, ".")
	if len(parts) < 2 {
		t.Fatalf("unparsable sqlite version %q", version)
	}
	major, _ := strconv.Atoi(parts[0])
	minor, _ := strconv.Atoi(parts[1])
	if major < 3 || (major == 3 && minor < 39) {
		t.Fatalf("sqlite %s is below the required 3.39 (IS DISTINCT FROM)", version)
	}

	// The DSN must actually be in effect; a silently ignored parameter here would
	// mean missing foreign keys or unbounded lock waits in production.
	for _, tc := range []struct{ pragma, want string }{
		{"journal_mode", "wal"},
		{"foreign_keys", "1"},
		{"busy_timeout", "5000"},
	} {
		var got string
		if err := db.QueryRowContext(ctx, "PRAGMA "+tc.pragma).Scan(&got); err != nil {
			t.Fatalf("PRAGMA %s: %v", tc.pragma, err)
		}
		if got != tc.want {
			t.Errorf("PRAGMA %s = %q, want %q", tc.pragma, got, tc.want)
		}
	}

	if err := migrateSQLiteAll(ctx, db); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	// Applying the same sections again must be a no-op, not an error.
	if err := migrateSQLiteAll(ctx, db); err != nil {
		t.Fatalf("migrate is not idempotent: %v", err)
	}
	var ledgerRows int
	if err := db.QueryRowContext(ctx, "SELECT count(*) FROM storage_migrations").Scan(&ledgerRows); err != nil {
		t.Fatal(err)
	}
	if ledgerRows != len(sqliteMigrationSections) {
		t.Errorf("ledger rows = %d, want %d (one per section)", ledgerRows, len(sqliteMigrationSections))
	}
	// The real driver must have created every table the SQLite schema declares. The
	// parser is the same one the section-coverage gate uses. (Until 2026-10-05 this
	// compared against the PostgreSQL schema, which was the reference the SQLite fork
	// was mirrored from; the engine and that tree are gone.)
	liteRaw, err := fs.ReadFile(sqliteMigrationSQL, sqliteInitialMigrationFile)
	if err != nil {
		t.Fatalf("read SQLite schema: %v", err)
	}
	liteTables, _ := parseDDL(liteRaw)
	if len(liteTables) == 0 {
		t.Fatal("parsed no SQLite tables; the parser or the schema layout changed")
	}
	var tableCount int
	if err := db.QueryRowContext(ctx,
		"SELECT count(*) FROM sqlite_master WHERE type='table' AND name NOT LIKE 'sqlite_%'").Scan(&tableCount); err != nil {
		t.Fatal(err)
	}
	if tableCount != len(liteTables) {
		t.Errorf("SQLite created %d tables, the schema declares %d", tableCount, len(liteTables))
	}

	q := sqlcgensqlite.New(db)

	// Foreign keys are ON, so an orphan character must be refused.
	if _, err := q.CreateCharacter(ctx, sqlcgensqlite.CreateCharacterParams{
		AccountID: 999, WireID: 1, Name: "orphan", ConfigVersion: "cfg",
		CreateRequest: []byte{1}, State: json.RawMessage(`{}`), RosterOrder: 1,
	}); err == nil {
		t.Error("orphan character accepted: _foreign_keys=1 is not in effect")
	}

	account, err := q.DevelopmentAccount(ctx, "sqlite-slice")
	if err != nil {
		t.Fatalf("development account: %v", err)
	}

	state := json.RawMessage(`{"level":26,"nested":{"a":1,"b":[1,2,3]}}`)
	created, err := q.CreateCharacter(ctx, sqlcgensqlite.CreateCharacterParams{
		AccountID: account, WireID: 1, Name: "Slice", Profession: 3,
		CreateRequest: []byte{0xde, 0xad}, ConfigVersion: "cfg-v1",
		// D24: JSON is stored as BLOB, so this stays json.RawMessage in both
		// directions and the bytes cross without any conversion.
		State: state, RosterOrder: 7,
	})
	if err != nil {
		t.Fatalf("create character: %v", err)
	}
	if created.ID == 0 {
		t.Fatal("create character returned id 0")
	}

	// Storage classes, read straight from SQLite.
	var stateClass, timeClass string
	if err := db.QueryRowContext(ctx,
		"SELECT typeof(state), typeof(created_at) FROM characters WHERE id=?", created.ID,
	).Scan(&stateClass, &timeClass); err != nil {
		t.Fatal(err)
	}
	if stateClass != "blob" {
		t.Errorf("state storage class = %q, want blob (D24: BLOB storage for json.RawMessage)", stateClass)
	}
	if timeClass != "integer" {
		t.Errorf("created_at storage class = %q, want integer microseconds", timeClass)
	}
	if delta := time.Since(created.CreatedAt); delta < 0 || delta > 2*time.Minute {
		t.Errorf("created_at round-trip = %v (delta %v); the microsecond time format is not applied", created.CreatedAt, delta)
	}

	rows, err := q.Characters(ctx, account)
	if err != nil {
		t.Fatalf("characters: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("characters = %d, want 1", len(rows))
	}
	if string(rows[0].State) != string(state) {
		t.Errorf("state round-trip changed the bytes:\n got %s\nwant %s", rows[0].State, state)
	}
	if rows[0].FixedSlot != 0 {
		t.Errorf("fixed_slot = %d, want 0", rows[0].FixedSlot)
	}
	if string(rows[0].CreateRequest) != string([]byte{0xde, 0xad}) {
		t.Errorf("create_request round-trip = %x", rows[0].CreateRequest)
	}

	// The composite primary key is the idempotency key for event replay.
	outcome := json.RawMessage(`{"granted":true}`)
	event := sqlcgensqlite.RecordCharacterEventParams{
		CharacterID: created.ID, EventKey: "slice:1", ConfigVersion: "cfg-v1",
		Model: "model-v1", Outcome: outcome,
	}
	if err := q.RecordCharacterEvent(ctx, event); err != nil {
		t.Fatalf("record event: %v", err)
	}
	if err := q.RecordCharacterEvent(ctx, event); err == nil {
		t.Error("duplicate event accepted: the (character_id,event_key) idempotency key is not enforced")
	}
	receipt, err := q.CharacterEventReceipt(ctx, sqlcgensqlite.CharacterEventReceiptParams{
		AccountID: account, CharacterID: created.ID, EventKey: "slice:1",
	})
	if err != nil {
		t.Fatalf("event receipt: %v", err)
	}
	if string(receipt) != string(outcome) {
		t.Errorf("receipt round-trip = %s, want %s", receipt, outcome)
	}

	// Archiving hides the character from the roster but keeps the row.
	if err := q.ArchiveCharacter(ctx, sqlcgensqlite.ArchiveCharacterParams{
		CharacterID: created.ID, AccountID: account,
	}); err != nil {
		t.Fatalf("archive: %v", err)
	}
	if rows, err = q.Characters(ctx, account); err != nil || len(rows) != 0 {
		t.Errorf("archived character still listed (n=%d, err=%v)", len(rows), err)
	}
	var deletedAt *time.Time
	if err := db.QueryRowContext(ctx, "SELECT deleted_at FROM characters WHERE id=?", created.ID).Scan(&deletedAt); err != nil {
		t.Fatal(err)
	}
	if deletedAt == nil {
		t.Error("deleted_at was not written as a timestamp")
	}

	// Identity must not be reused after the row is gone from the roster.
	second, err := q.CreateCharacter(ctx, sqlcgensqlite.CreateCharacterParams{
		AccountID: account, WireID: 2, Name: "Slice2", Profession: 4,
		CreateRequest: []byte{2}, ConfigVersion: "cfg-v1", State: json.RawMessage(`{"level":1}`), RosterOrder: 8,
	})
	if err != nil {
		t.Fatalf("second character: %v", err)
	}
	if second.ID <= created.ID {
		t.Errorf("identity reused: second id %d <= archived id %d", second.ID, created.ID)
	}

	allocation, err := q.CharacterAllocation(ctx, account)
	if err != nil {
		t.Fatalf("allocation: %v", err)
	}
	if allocation.ActiveCount != 1 {
		t.Errorf("active count = %d, want 1 (the archived row must not count)", allocation.ActiveCount)
	}
	if allocation.NextWireID != 3 {
		t.Errorf("next wire id = %d, want 3", allocation.NextWireID)
	}
	if allocation.NextRosterOrder != 9 {
		t.Errorf("next roster order = %d, want 9", allocation.NextRosterOrder)
	}

	// AccountCharacterStates carries no deleted filter, and reaches the JSON
	// override on a bare column reference.
	states, err := q.AccountCharacterStates(ctx, account)
	if err != nil {
		t.Fatalf("account character states: %v", err)
	}
	if len(states) != 2 {
		t.Errorf("account character states = %d, want 2 (archived rows included)", len(states))
	}
}
