package database

import (
	"database/sql"
	"errors"

	"github.com/jackc/pgx/v5"
)

// driverNoRows reports the "no rows" condition as the DRIVER spells it.
//
// This is the only helper that should look at raw driver sentinels. storageError
// uses it to translate absence into the engine-independent ErrNotFound; everything
// else asks isNoRows instead.
func driverNoRows(err error) bool {
	return errors.Is(err, pgx.ErrNoRows) || errors.Is(err, sql.ErrNoRows)
}

// isNoRows reports whether err means "there is no such stored record", whichever
// storage engine is in use.
//
// It is deliberately the ONLY place callers should ask this question.
//
// Absence reaches a caller in two different shapes:
//
//   - PostgreSQL hands the raw pgx.ErrNoRows straight through;
//   - SQLite runs every adapter result through storageError, which replaces the
//     driver sentinel with the package's ErrNotFound before the caller ever sees it.
//
// So a caller that only tests pgx.ErrNoRows takes the "error" branch on SQLite even
// though the lookup legitimately found nothing. That single mistake broke every
// "look the row up; if it is absent, carry on" path on the SQLite engine - equipping
// and unequipping gear (the character-event ledger has no row for a first move) and
// accepting quests (no prior character_quests row) among them, while PostgreSQL kept
// working. Callers' control flow - fall back to a default, report not-found, or
// surface the error - is identical on both engines, so one predicate keeps a change
// of engine from leaving a driver-specific check behind at any of these call sites
// (see docs/sqlite-dual-engine-design.md S2).
func isNoRows(err error) bool {
	return driverNoRows(err) || errors.Is(err, ErrNotFound)
}
