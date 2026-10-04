package database

import (
	"database/sql"
	"errors"

	"github.com/jackc/pgx/v5"
)

// isNoRows reports whether err is the "no rows" result of whichever storage
// engine is in use.
//
// It is deliberately the ONLY place that knows how each driver spells this.
// Callers' control flow — fall back to a default, report not-found, or surface
// the error — is identical on both engines, so a single predicate means a change
// of engine cannot leave a driver-specific check behind at one of the forty-odd
// call sites (see docs/sqlite-dual-engine-design.md S2).
//
// The SQLite spelling is accepted here before that engine exists, so adding the
// engine later needs no caller change at all.
func isNoRows(err error) bool {
	return errors.Is(err, pgx.ErrNoRows) || errors.Is(err, sql.ErrNoRows)
}
