-- name: LockMigrations :exec
-- Transaction-scoped; released automatically even when initialization fails.
SELECT pg_advisory_xact_lock(11520261004);

-- name: MigrationChecksum :one
SELECT checksum FROM storage_migrations WHERE name=sqlc.arg(name);

-- name: RecordMigration :exec
INSERT INTO storage_migrations(name,checksum) VALUES(sqlc.arg(name),sqlc.arg(checksum))
ON CONFLICT(name) DO UPDATE SET last_applied_at=now(),runs=storage_migrations.runs+1;
