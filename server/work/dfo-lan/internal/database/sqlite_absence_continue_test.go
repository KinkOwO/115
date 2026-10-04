package database

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"testing"

	"dfolan/internal/savecontract"

	"github.com/jackc/pgx/v5"
)

// These tests pin the real-machine symptom, not just the adapter mapping.
//
// On the SQLite engine both "accept a quest" and "move a piece of equipment" take a
// path that looks a row up and carries on when it is absent: there is no
// character_quests row before the first accept, and no character-event row before the
// first move for that idempotency key. The adapter hands that absence over as
// ErrNotFound, so a caller that only tested the pgx sentinel took its error branch and
// refused the action - PostgreSQL kept working, which is exactly what was reported.

func TestIsNoRowsCoversEveryEngineSpelling(t *testing.T) {
	for name, err := range map[string]error{
		"ErrNotFound":      ErrNotFound,
		"sql.ErrNoRows":    sql.ErrNoRows,
		"pgx.ErrNoRows":    pgx.ErrNoRows,
		"wrapped-notfound": fmt.Errorf("character lookup: %w", ErrNotFound),
	} {
		if !isNoRows(err) {
			t.Errorf("isNoRows(%s) = false, want true", name)
		}
	}
	for name, err := range map[string]error{
		"nil":         nil,
		"other error": errors.New("boom"),
	} {
		if isNoRows(err) {
			t.Errorf("isNoRows(%s) = true, want false", name)
		}
	}
}

func sqliteAbsenceStore(t *testing.T) (*Store, int64, Character, string) {
	t.Helper()
	ctx := context.Background()
	store, err := Open(ctx, Config{SQLitePath: filepath.Join(t.TempDir(), "absence-continue.db")})
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(store.Close)

	account, err := store.DevelopmentAccount(ctx, "absence-continue")
	if err != nil {
		t.Fatalf("DevelopmentAccount: %v", err)
	}
	version := savecontract.Identity()
	role, err := store.CreateCharacter(ctx, Character{
		AccountID:     account,
		WireID:        1,
		Name:          "Absence",
		ConfigVersion: version,
		Request:       []byte{0},
		State:         json.RawMessage(`{"level":1}`),
	}, 8)
	if err != nil {
		t.Fatalf("CreateCharacter: %v", err)
	}
	return store, account, role, version
}

// Accepting a quest must not need a prior character_quests row.
func TestSQLiteAcceptsQuestWithoutAPriorRow(t *testing.T) {
	store, account, role, version := sqliteAbsenceStore(t)
	ctx := context.Background()

	state, err := store.AcceptQuestGroups(ctx, account, role.ID, 3145, version, 1, 100, nil, 0, "single-clear-map-remaining-v1")
	if err != nil {
		t.Fatalf("accepting a quest with no prior row must continue, got %v", err)
	}
	if state.ID != 3145 || state.Status != "accepted" {
		t.Fatalf("accepted state = %+v, want ID 3145 accepted", state)
	}
	stored, err := store.Quests(ctx, account, role.ID)
	if err != nil {
		t.Fatalf("Quests: %v", err)
	}
	if len(stored) != 1 || uint16(stored[0].ID) != 3145 {
		t.Fatalf("stored quests = %+v, want exactly 3145", stored)
	}
}

// A first equipment move must not need a prior character-event ledger row, and
// replaying the same key must stay idempotent.
func TestSQLiteAppliesFirstCharacterEvent(t *testing.T) {
	store, account, role, version := sqliteAbsenceStore(t)
	ctx := context.Background()

	apply := func(Character) (json.RawMessage, json.RawMessage, error) {
		return json.RawMessage(`{"level":1,"moved":true}`), json.RawMessage(`{"applied":true}`), nil
	}
	saved, applied, err := store.CommitCharacterEvent(ctx, account, role.ID, version, "absence-first-move", "ordinary-equipment-move-v1", apply)
	if err != nil {
		t.Fatalf("first move with an empty ledger must continue, got %v", err)
	}
	if !applied {
		t.Fatal("first move reported applied=false, want the transition to run")
	}
	if saved.ID != role.ID {
		t.Fatalf("saved character id = %d, want %d", saved.ID, role.ID)
	}

	replay, appliedAgain, err := store.CommitCharacterEvent(ctx, account, role.ID, version, "absence-first-move", "ordinary-equipment-move-v1", apply)
	if err != nil {
		t.Fatalf("replaying the same key must stay idempotent, got %v", err)
	}
	if appliedAgain {
		t.Fatal("replay re-applied the transition, want the stored event to be reused")
	}
	if replay.ID != role.ID {
		t.Fatalf("replayed character id = %d, want %d", replay.ID, role.ID)
	}
}
