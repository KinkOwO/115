-- SQLite query fork of sql/postgres/queries/migrations.sql (dual-engine, S3).
--
-- The PostgreSQL file stays untouched and remains authoritative for the PG path.
--
-- NOT PORTED: LockMigrations (:exec) ran pg_advisory_xact_lock(11520261004).
-- Advisory locks do not exist in SQLite: the migration runner serializes writers
-- with BEGIN IMMEDIATE (DSN _txlock=immediate), which is transaction-scoped and
-- released on commit/rollback exactly like the PostgreSQL xact lock. There is no
-- portable statement to emit, so the query is omitted rather than guessed
-- (design doc D12 / section 3.4).
--
-- Ledger columns verified against the SQLite schema
-- (sqlite/migrations/0001_initial.sql, section 0000_migration_ledger):
--   storage_migrations(name, checksum, applied_at, last_applied_at, runs).
-- applied_at keeps the DDL default; RecordMigration still writes only
-- name/checksum and refreshes last_applied_at on conflict. Both timestamps are
-- integer microseconds, so now() is spelled with the microsecond expression used
-- by the DDL default and by core.sql's ArchiveCharacter.

-- name: MigrationChecksum :one
SELECT checksum FROM storage_migrations WHERE name=sqlc.arg(name);

-- name: RecordMigration :exec
INSERT INTO storage_migrations(name,checksum) VALUES(sqlc.arg(name),sqlc.arg(checksum))
ON CONFLICT(name) DO UPDATE SET last_applied_at=(CAST((julianday('now') - 2440587.5) * 86400000000 AS INTEGER)),runs=storage_migrations.runs+1;
