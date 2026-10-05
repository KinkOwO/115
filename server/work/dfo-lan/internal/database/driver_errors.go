package database

import (
	"database/sql"
	"errors"
)

// driverNoRows reports the "no rows" condition as the DRIVER spells it.
//
// This is the only helper that should look at raw driver sentinels. storageError
// uses it to translate absence into the engine-independent ErrNotFound; everything
// else asks isNoRows instead.
//
// SQLite is the only engine since 2026-10-05 (owner decision, see root AGENTS.md §0.6),
// so database/sql's sentinel is the only spelling left to recognise here; the pgx
// variant went with the engine.
func driverNoRows(err error) bool {
	return errors.Is(err, sql.ErrNoRows)
}

// isNoRows reports whether err means "there is no such stored record".
//
// It is deliberately the ONLY place callers should ask this question.
//
// Even on one engine, absence reaches a caller in two shapes:
//
//   - a raw database/sql result surfaces sql.ErrNoRows;
//   - SQLite runs every adapter result through storageError, which replaces the
//     driver sentinel with the package's ErrNotFound before the caller ever sees it.
//
// So a caller that tests only one of the two spellings takes the "error" branch on a
// legitimate miss. That single mistake once broke every "look the row up; if it is
// absent, carry on" path on SQLite - equipping and unequipping gear (the character-event
// ledger has no row for a first move) and accepting quests (no prior character_quests
// row) among them - while the other engine kept working. Callers' control flow - fall
// back to a default, report not-found, or surface the error - is identical either way,
// so one predicate keeps a driver-specific check from being left behind at any of these
// call sites (see docs/sqlite-dual-engine-design.md S2).
func isNoRows(err error) bool {
	return driverNoRows(err) || errors.Is(err, ErrNotFound)
}
