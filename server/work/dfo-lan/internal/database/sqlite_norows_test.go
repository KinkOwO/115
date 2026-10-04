package database

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"dfolan/internal/database/sqlcgen"
)

// Absence must reach callers in the same form on both engines.
//
// pgx reports a missing row as pgx.ErrNoRows while database/sql reports sql.ErrNoRows, and
// the package's callers only understand ErrNotFound. Recognising just the pgx sentinel made
// every "look the row up; if it is absent, carry on" path fail hard on SQLite - which is how
// equipping gear and accepting quests broke, with the gateway logging
// "sql: no rows in result set".
func TestSQLiteMapsNoRowsToNotFound(t *testing.T) {
	ctx := context.Background()
	store, err := Open(ctx, Config{SQLitePath: filepath.Join(t.TempDir(), "norows.db")})
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer store.Close()

	// LockCharacter is deliberate: equipping and quests both take this lock, so if absence
	// is reported wrongly here, both break together - exactly the reported symptom.
	_, err = store.queries.LockCharacter(ctx, sqlcgen.LockCharacterParams{
		AccountID: 1, CharacterID: 999999,
	})
	if err == nil {
		t.Fatal("locking a character that does not exist returned no error")
	}
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("missing row returned %v, want ErrNotFound", err)
	}
	if strings.Contains(err.Error(), "no rows in result set") {
		t.Errorf("the raw driver error escaped the adapter: %v", err)
	}

	// A query that does find its row must still succeed, so the mapping did not swallow
	// real results.
	account, err := store.DevelopmentAccount(ctx, "norows")
	if err != nil {
		t.Fatalf("DevelopmentAccount: %v", err)
	}
	if _, err := store.queries.LockCharacter(ctx, sqlcgen.LockCharacterParams{
		AccountID: account, CharacterID: 999999,
	}); !errors.Is(err, ErrNotFound) {
		t.Errorf("still want ErrNotFound for a missing character, got %v", err)
	}
}
