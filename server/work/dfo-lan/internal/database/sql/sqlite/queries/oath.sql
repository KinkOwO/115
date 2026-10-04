-- SQLite query fork of sql/postgres/queries/oath.sql (dual-engine, S3).
--
-- The PostgreSQL file stays untouched and remains authoritative for the PG path.
--
-- Dialect rules applied:
--   * FOR UPDATE is deleted from LockOathOptionRevision; SQLite has no row locks
--     and the engine serializes writers with BEGIN IMMEDIATE (_txlock=immediate).
--     Name and predicate are unchanged.
--   * selected_option is written through CAST(sqlc.arg(x) AS INTEGER) because the
--     PostgreSQL file casts that parameter to bigint (int64) in three statements,
--     while the column itself is INTEGER; a bare parameter in those INSERT ...
--     VALUES positions would inherit the column's int32 and diverge from the PG
--     signature. revision is a BIGINT column and is left bare (int64 either way).
--   * now() -> (CAST((julianday('now') - 2440587.5) * 86400000000 AS INTEGER)),
--     integer microseconds, matching the DDL default and the driver's
--     _inttotime=1 + _time_integer_format=unix_micro representation.
--   * ON CONFLICT ... DO UPDATE ... EXCLUDED is supported as written; the
--     DO UPDATE clause carries no parameter, so nothing can be lost there.

-- name: EquippedOathSelection :one
SELECT selected_option FROM character_oath_options WHERE character_id=sqlc.arg(character_id) AND core_instance_key=sqlc.arg(core_instance_key);

-- name: SelectEquippedOathOption :exec
INSERT INTO character_oath_options(character_id,core_instance_key,selected_option,revision,updated_at)
VALUES(sqlc.arg(character_id),sqlc.arg(core_instance_key),CAST(sqlc.arg(selected_option) AS INTEGER),1,(CAST((julianday('now') - 2440587.5) * 86400000000 AS INTEGER)))
ON CONFLICT(character_id,core_instance_key) DO UPDATE SET selected_option=EXCLUDED.selected_option,
revision=character_oath_options.revision+1,updated_at=(CAST((julianday('now') - 2440587.5) * 86400000000 AS INTEGER));

-- name: OathOption :one
SELECT character_id,core_instance_key,selected_option,revision FROM character_oath_options
WHERE character_id=sqlc.arg(character_id) AND core_instance_key=sqlc.arg(core_instance_key);

-- name: LockOathOptionRevision :one
-- Lock query; FOR UPDATE removed, engine-wide write serialization instead.
SELECT revision FROM character_oath_options WHERE character_id=sqlc.arg(character_id) AND core_instance_key=sqlc.arg(core_instance_key);

-- name: InsertOathOption :exec
INSERT INTO character_oath_options(character_id,core_instance_key,selected_option,revision,updated_at)
VALUES(sqlc.arg(character_id),sqlc.arg(core_instance_key),CAST(sqlc.arg(selected_option) AS INTEGER),sqlc.arg(revision),(CAST((julianday('now') - 2440587.5) * 86400000000 AS INTEGER)));

-- name: UpdateOathOption :exec
UPDATE character_oath_options SET selected_option=CAST(sqlc.arg(selected_option) AS INTEGER),revision=sqlc.arg(revision),updated_at=(CAST((julianday('now') - 2440587.5) * 86400000000 AS INTEGER))
WHERE character_id=sqlc.arg(character_id) AND core_instance_key=sqlc.arg(core_instance_key);
