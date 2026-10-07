package database

import (
	"context"
)

// execMigration is the per-domain schema hook the Migrate* methods share.
//
// SQLite applies the complete schema in openSQLiteStore (sql/sqlite/migrations/
// 0001_initial.sql, executed section by section) before any domain code runs, and
// that is idempotent, so every Migrate* call is honestly a no-op here — it is kept
// so the (large) call surface in bootstrap/admin/gmtool keeps compiling and the
// per-domain intent stays documented.
//
// PostgreSQL used to run an ordered per-domain walk with a migration ledger and
// per-file checksums at this spot, reading an embedded sql/postgres/migrations tree.
// PostgreSQL support was removed on 2026-10-05 (owner decision, see root AGENTS.md
// §0.6), so that tree, the ledger reader and the per-section checksum logic are gone;
// a historical pgdata stays readable by checking out the commit before that removal
// (see docs/sqlite-operations.md).
func (s *Store) execMigration(_ context.Context, _ string) error { return nil }

// InitializeGame owns the gateway's unconditional persistence initialization.
// Content-dependent schemas and repairs stay at their existing call sites:
// this list deliberately does not discover and run every embedded SQL file.
func (s *Store) InitializeGame(ctx context.Context) error {
	for _, migrate := range []func(context.Context) error{
		s.Migrate, s.MigrateAdventure, s.MigrateBleedingMine, s.MigrateTutorial,
		s.MigrateUnifiedOptions, s.MigrateGamepad, s.MigrateGrants, s.MigratePremiums,
		s.MigrateCharacterEvents, s.MigrateShopPurchases, s.MigrateCharacterNotices,
		s.MigrateProfileSkins, s.MigrateRosterBackgrounds, s.MigrateMailbox,
		s.MigrateOathProgress, s.MigrateOathOptions, s.MigrateEquipmentSkill, s.MigrateOmenState,
	} {
		if err := migrate(ctx); err != nil {
			return err
		}
	}
	return nil
}
