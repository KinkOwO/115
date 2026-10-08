package database

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"path/filepath"
	"strconv"
	"strings"

	// Registers the "sqlite" driver with database/sql. Pure Go, so CGO stays off.
	_ "modernc.org/sqlite"
)

// SQLite engine (dual-engine plan, S3/S4). See docs/sqlite-dual-engine-design.md.
//
// The dependency is pure Go (modernc.org/sqlite), so CGO stays disabled and the
// binary stays portable; its bundled libc version must match what the driver's
// go.mod pins, which is why go.mod names it explicitly rather than via `go get -u`.
//
// The driver is imported for its registration side effect only: every statement
// goes through database/sql, and nothing outside this package may see sql.DB
// (archtest persistence boundary).

//go:embed sql/sqlite/migrations/*.sql
var sqliteMigrationSQL embed.FS

const sqliteInitialMigrationFile = "sql/sqlite/migrations/0001_initial.sql"

// sqliteDSN builds the connection string described in the design doc §5.1.
//
// Values are joined by hand rather than through url.Values: the driver parses
// the raw query itself, and percent-encoding the parentheses in values such as
// journal_mode(WAL) would change what SQLite receives. Shorthand keys are
// validated by the driver (a typo fails the connection instead of being ignored),
// which is the behavior we want for a save-compatibility-critical setting.
func sqliteDSN(path string, busyTimeoutMS int) string {
	slashed := filepath.ToSlash(path)
	if !strings.HasPrefix(slashed, "/") {
		slashed = "/" + slashed // C:/x -> /C:/x so the URI becomes file:///C:/x
	}
	params := strings.Join([]string{
		// Writers serialize through BEGIN IMMEDIATE; this is what replaces every
		// PostgreSQL row lock (FOR UPDATE / FOR NO KEY UPDATE / FOR SHARE).
		"_txlock=immediate",
		// Bounded wait instead of an immediate SQLITE_BUSY.
		"_busy_timeout=" + strconv.Itoa(busyTimeoutMS),
		// PostgreSQL enforced foreign keys; SQLite has them OFF by default.
		"_foreign_keys=1",
		"_journal_mode=WAL",
		"_synchronous=NORMAL",
		// Times are integer microseconds, matching PostgreSQL timestamptz
		// microsecond precision and the SQLite DDL's DEFAULT expression.
		"_timezone=UTC",
		"_inttotime=1",
		"_time_integer_format=unix_micro",
		// Make a double-quoted string literal a parse error instead of silently
		// treating it as a string.
		"_dqs=0",
	}, "&")
	return "file://" + slashed + "?" + params
}

// openSQLite opens (creating if needed) a SQLite database with the engine-wide
// settings above and a bounded pool: database/sql imposes no limit by default and
// every connection carries its own page cache, so an unbounded pool would both
// waste memory and amplify write contention.
func openSQLite(ctx context.Context, path string, maxOpenConns, busyTimeoutMS int) (*sql.DB, error) {
	if strings.TrimSpace(path) == "" {
		return nil, errors.New("sqlite path is required")
	}
	if maxOpenConns < 1 {
		maxOpenConns = 1
	}
	db, err := sql.Open("sqlite", sqliteDSN(path, busyTimeoutMS))
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(maxOpenConns)
	db.SetMaxIdleConns(maxOpenConns)
	if err := db.PingContext(ctx); err != nil {
		db.Close()
		return nil, err
	}
	return db, nil
}

// migrationSection returns one named section of an initial migration file, or the
// whole of a standalone incremental file. Shared by both engines so the ledger
// identity, the section delimiters and the checksum rule cannot drift apart.
func migrationSection(fsys fs.FS, initial, name string) ([]byte, error) {
	raw, err := fs.ReadFile(fsys, initial)
	if err != nil {
		return nil, err
	}
	text := strings.ReplaceAll(string(raw), "\r\n", "\n")
	header := "-- migration: " + name + "\n"
	_, section, found := strings.Cut(text, header)
	if !found {
		// Future incremental files remain ordinary SQL. Never execute the
		// complete initial file through one module's initialization call.
		if name == filepath.Base(initial) {
			return nil, fmt.Errorf("initial schema must execute through its migration sections")
		}
		return fs.ReadFile(fsys, strings.TrimSuffix(initial, filepath.Base(initial))+name)
	}
	query, remainder, closed := strings.Cut(section, "\n-- end migration\n")
	if !closed || strings.Contains(query, "\n-- migration: ") || strings.Contains(remainder, header) {
		return nil, fmt.Errorf("invalid or duplicated initial migration section %s", name)
	}
	return []byte(query), nil
}

func sqliteMigrationSection(name string) ([]byte, error) {
	return migrationSection(sqliteMigrationSQL, sqliteInitialMigrationFile, name)
}

// sqliteMigrationChecksum mirrors the PostgreSQL rule: Git's Windows checkout
// policy must not change a migration's identity.
func sqliteMigrationChecksum(query []byte) string {
	normalized := strings.ReplaceAll(string(query), "\r\n", "\n")
	return fmt.Sprintf("%x", sha256.Sum256([]byte(normalized)))
}

// migrateSQLite applies one SQLite migration section inside a single transaction
// and records it in the same ledger shape the PostgreSQL path uses. The write
// transaction serializes with BEGIN IMMEDIATE (DSN _txlock), so a concurrent
// migration cannot interleave.
func migrateSQLite(ctx context.Context, db *sql.DB, name string) error {
	query, err := sqliteMigrationSection(name)
	if err != nil {
		return err
	}
	checksum := sqliteMigrationChecksum(query)
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	ledger, err := sqliteMigrationSection("0000_migration_ledger.sql")
	if err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, string(ledger)); err != nil {
		return err
	}
	var previous string
	err = tx.QueryRowContext(ctx, "SELECT checksum FROM storage_migrations WHERE name=?", name).Scan(&previous)
	switch {
	case err == nil:
		if previous != checksum {
			return fmt.Errorf("migration %s checksum differs; preserve applied SQL and add a new migration", name)
		}
		return tx.Commit()
	case !isNoRows(err):
		return err
	}
	if strings.TrimSpace(string(query)) != "" {
		if _, err := tx.ExecContext(ctx, string(query)); err != nil {
			return fmt.Errorf("migration %s: %w", name, err)
		}
	}
	if _, err := tx.ExecContext(ctx,
		"INSERT INTO storage_migrations(name,checksum) VALUES(?,?)", name, checksum); err != nil {
		return err
	}
	return tx.Commit()
}

// sqliteMigrationSections lists every section applied to a new SQLite database,
// in order. It is explicit rather than discovered so that adding a section to the
// file stays a deliberate act, and TestSQLiteMigrationSectionsCoverFile keeps the
// list and the file from drifting apart. 0028 is listed although it carries no
// statement (its PostgreSQL body is data repair), so both ledgers record the same
// section names and can be compared directly.
var sqliteMigrationSections = []string{
	"0001_core.sql",
	"0002_character_events.sql",
	"0003_character_notices.sql",
	"0004_tutorial.sql",
	"0005_warp_favorites.sql",
	"0006_adventure.sql",
	"0007_equipment_skill.sql",
	"0008_skill_locks.sql",
	"0009_profile_skins.sql",
	"0010_gamepad.sql",
	"0011_shop_purchases.sql",
	"0012_roster_backgrounds.sql",
	"0013_skin_selection.sql",
	"0014_skin_lists.sql",
	"0015_skin_cargo.sql",
	"0016_unified_options.sql",
	"0017_account_materials.sql",
	"0018_grants.sql",
	"0019_world.sql",
	"0020_birth.sql",
	"0021_fatigue.sql",
	"0022_premiums.sql",
	"0023_quests.sql",
	"0024_quest_objectives.sql",
	"0025_quest_rewards.sql",
	"0026_vaults.sql",
	"0027_account_vault.sql",
	"0028_secondary_vault_upgrade.sql",
	"0029_cash_shop.sql",
	"0030_omen_state.sql",
	"0031_oath_progress.sql",
	"0032_oath_options.sql",
	"0033_bleeding_mine.sql",
	"0034_tower_grief.sql",
	"0035_tower_progress.sql",
	"0036_mailbox.sql",
	"0037_gm_mail.sql",
	"0039_character_slots_bonus.sql",
}

// migrateSQLiteAll brings a fresh (or already migrated) SQLite database up to the
// current schema. Each section is applied in its own transaction and recorded in
// the ledger, so re-running is a checksum comparison rather than a replay.
func migrateSQLiteAll(ctx context.Context, db *sql.DB) error {
	for _, name := range sqliteMigrationSections {
		if err := migrateSQLite(ctx, db, name); err != nil {
			return err
		}
	}
	return nil
}

// S3b continues here: the engine-neutral query interface and its two adapters.

// sqliteSaveApplicationID is the identifier the 20261004 upgrade package stamps into a DFO
// SQLite save via PRAGMA application_id. Writing the same value means a database created by
// either implementation is recognised as a DFO save by the other; reading a save does not
// require the stamp, so the reverse direction already worked.
const sqliteSaveApplicationID = 1152026104

// stampSQLiteSaveIdentity records that this file is a DFO save. It is idempotent, so it is
// safe to run against an existing database as well as a fresh one.
func stampSQLiteSaveIdentity(ctx context.Context, db *sql.DB) error {
	_, err := db.ExecContext(ctx, fmt.Sprintf("PRAGMA application_id = %d", sqliteSaveApplicationID))
	return err
}
