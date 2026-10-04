package database

import (
	"context"
	"crypto/sha256"
	"embed"
	"fmt"
	"strings"

	"dfolan/internal/database/sqlcgen"

)

// The initial schema is one file, shared with sqlc. Historical sections keep
// their ledger identities and original execution gates, so consolidation does
// not invalidate an existing database or automatically run optional repairs.
//
//go:embed sql/postgres/migrations/*.sql
var migrationSQL embed.FS

const initialMigrationFile = "sql/postgres/migrations/0001_initial.sql"

func migrationQuery(name string) ([]byte, error) {
	initial, err := migrationSQL.ReadFile(initialMigrationFile)
	if err != nil {
		return nil, err
	}
	text := strings.ReplaceAll(string(initial), "\r\n", "\n")
	header := "-- migration: " + name + "\n"
	_, section, found := strings.Cut(text, header)
	if !found {
		// Future incremental files remain ordinary SQL. Never execute the
		// complete initial file through one module's initialization call.
		if name == "0001_initial.sql" {
			return nil, fmt.Errorf("initial schema must execute through its migration sections")
		}
		return migrationSQL.ReadFile("sql/postgres/migrations/" + name)
	}
	query, remainder, closed := strings.Cut(section, "\n-- end migration\n")
	if !closed || strings.Contains(query, "\n-- migration: ") || strings.Contains(remainder, header) {
		return nil, fmt.Errorf("invalid or duplicated initial migration section %s", name)
	}
	return []byte(query), nil
}

func (s *Store) execMigration(ctx context.Context, name string) error {
	// The SQLite engine applies every section in openSQLiteStore, and the ledger makes
	// that idempotent. PostgreSQL keeps its ordered per-domain walk because its schema
	// grew incrementally; on SQLite the same call is honestly a no-op.
	if _, ok := s.engine.(*sqliteEngine); ok {
		return nil
	}
	query, err := migrationQuery(name)
	if err != nil {
		return err
	}
	// Git's Windows checkout policy must not change a migration's identity.
	checksum := fmt.Sprintf("%x", sha256.Sum256([]byte(strings.ReplaceAll(string(query), "\r\n", "\n"))))
	tx, err := s.engine.begin(ctx)
	if err != nil {
		return err
	}
	defer tx.rollback(ctx)
	q := tx.queries()
	if err := q.LockMigrations(ctx); err != nil {
		return err
	}
	ledger, err := migrationQuery("0000_migration_ledger.sql")
	if err != nil {
		return err
	}
	if err := tx.exec(ctx, string(ledger)); err != nil {
		return err
	}
	previous, err := q.MigrationChecksum(ctx, name)
	if err != nil && !isNoRows(err) {
		return err
	}
	if err == nil {
		if previous != checksum {
			return fmt.Errorf("migration %s checksum differs; preserve applied SQL and add a new migration", name)
		}
		// These legacy repairs used to run at every module initialization.
		// Imported old saves and newly missing tower rows must retain that behavior.
		switch name {
		case "0015_skin_cargo.sql", "0023_quests.sql", "0028_secondary_vault_upgrade.sql", "0035_tower_progress.sql":
		default:
			return tx.commit(ctx)
		}
	}
	if err := tx.exec(ctx, string(query)); err != nil {
		return fmt.Errorf("migration %s: %w", name, err)
	}
	if err := q.RecordMigration(ctx, sqlcgen.RecordMigrationParams{Name: name, Checksum: checksum}); err != nil {
		return err
	}
	return tx.commit(ctx)
}

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
