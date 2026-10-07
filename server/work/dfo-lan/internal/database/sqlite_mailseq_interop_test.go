package database

import (
	"context"
	"path/filepath"
	"testing"
)

// A save created by the 20261004 upgrade package carries its own mail counter. This
// implementation must keep that counter in step, or alternating between the two would hand
// out the same mail id twice - a save-corrupting outcome rather than a cosmetic one.
func TestMailIDsStayAlignedWithTheUpgradePackageCounter(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "game.db")
	store, err := Open(ctx, Config{SQLitePath: path})
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer store.Close()
	db := store.engine.(*sqliteEngine).db

	// Reproduce the other implementation's table, as its importer leaves it: the counter
	// starts at the next id to hand out.
	if _, err := db.ExecContext(ctx,
		"CREATE TABLE IF NOT EXISTS mailbox_id_sequence(name TEXT PRIMARY KEY, value INTEGER NOT NULL CHECK(value>0))"); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx,
		"INSERT INTO mailbox_id_sequence(name,value) VALUES('mailbox_id_seq',1) ON CONFLICT(name) DO NOTHING"); err != nil {
		t.Fatal(err)
	}

	first, err := store.queries.NextMailID(ctx)
	if err != nil {
		t.Fatalf("NextMailID: %v", err)
	}
	var theirValue int64
	if err := db.QueryRowContext(ctx,
		"SELECT value FROM mailbox_id_sequence WHERE name='mailbox_id_seq'").Scan(&theirValue); err != nil {
		t.Fatalf("read their counter: %v", err)
	}
	// Their next allocation must not repeat the id we just handed out.
	if theirValue <= first {
		t.Errorf("their counter = %d after we issued %d; the next id would repeat", theirValue, first)
	}

	// A counter already ahead of us must never be pulled backwards.
	if _, err := db.ExecContext(ctx,
		"UPDATE mailbox_id_sequence SET value = 5000 WHERE name='mailbox_id_seq'"); err != nil {
		t.Fatal(err)
	}
	if _, err := store.queries.NextMailID(ctx); err != nil {
		t.Fatalf("NextMailID: %v", err)
	}
	if err := db.QueryRowContext(ctx,
		"SELECT value FROM mailbox_id_sequence WHERE name='mailbox_id_seq'").Scan(&theirValue); err != nil {
		t.Fatal(err)
	}
	if theirValue != 5000 {
		t.Errorf("their counter moved backwards to %d; it must only move forward", theirValue)
	}
}

// A save without the other implementation's table must be unaffected: the alignment step is
// a no-op there, so ids keep coming from SQLite's own high-water mark.
func TestMailIDsUnaffectedWithoutTheUpgradePackageTable(t *testing.T) {
	ctx := context.Background()
	store, err := Open(ctx, Config{SQLitePath: filepath.Join(t.TempDir(), "plain.db")})
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer store.Close()

	first, err := store.queries.NextMailID(ctx)
	if err != nil {
		t.Fatalf("NextMailID: %v", err)
	}
	second, err := store.queries.NextMailID(ctx)
	if err != nil {
		t.Fatalf("NextMailID: %v", err)
	}
	if second != first+1 {
		t.Errorf("ids went %d -> %d; they must advance by one", first, second)
	}
}
