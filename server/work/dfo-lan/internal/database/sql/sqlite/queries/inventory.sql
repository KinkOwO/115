-- SQLite query fork of sql/postgres/queries/inventory.sql (dual-engine, S3).
--
-- The PostgreSQL file stays untouched and remains authoritative for the PG path.
--
-- Dialect rules applied (docs/sqlite-query-port-guide.md, D22-D29):
--   * FOR UPDATE is deleted from every lock query; SQLite has no row locks and the
--     engine serializes writers with BEGIN IMMEDIATE (DSN _txlock=immediate).
--     Names and ownership predicates are unchanged, so callers are unchanged.
--   * now() -> (CAST((julianday('now') - 2440587.5) * 86400000000 AS INTEGER)),
--     integer microseconds, matching the DDL defaults and the driver's
--     _inttotime=1 + _time_integer_format=unix_micro representation.
--   * Projections and parameters are left bare so they inherit the schema types:
--     coalesce(v.slots,0) is int32 because account_vaults.slots is INTEGER,
--     coalesce(v.gold,0) is int64 because gold is BIGINT, and
--     coalesce(v.items,CAST('[]' AS BLOB)) is json.RawMessage because
--     account_vaults.items is a json column. Wrapping them in CAST(... AS INTEGER)
--     would flatten every width to int64, so no CAST is used here.
--   * JSON columns are BLOB, so JSON parameters are passed straight through with no
--     CAST (D24). The '[]' fallback literal is a BLOB for the same reason.
--   * ON CONFLICT DO NOTHING / DO UPDATE and INSERT ... SELECT are supported as
--     written; the INSERT ... SELECT parameter takes the target column's width.

-- name: AccountMaterials :one
SELECT counts FROM account_material_storage WHERE account_id=sqlc.arg(account_id);

-- name: InitializeAccountMaterials :exec
INSERT INTO account_material_storage(account_id) VALUES(sqlc.arg(account_id)) ON CONFLICT(account_id) DO NOTHING;

-- name: LockAccountMaterials :one
-- Lock query; the FOR UPDATE is replaced by engine-wide BEGIN IMMEDIATE write
-- serialization. Name and predicate are unchanged.
SELECT counts FROM account_material_storage WHERE account_id=sqlc.arg(account_id);

-- name: SaveAccountMaterials :exec
UPDATE account_material_storage SET counts=sqlc.arg(counts),updated_at=(CAST((julianday('now') - 2440587.5) * 86400000000 AS INTEGER)) WHERE account_id=sqlc.arg(account_id);

-- name: LoadAccountVault :one
SELECT coalesce(v.slots,0) AS slots,coalesce(v.gold,0) AS gold,
coalesce(v.items,CAST('[]' AS BLOB)) AS items FROM characters c LEFT JOIN account_vaults v ON v.account_id=c.account_id
WHERE c.id=sqlc.arg(character_id) AND c.account_id=sqlc.arg(account_id) AND c.deleted_at IS NULL;

-- name: EnsureAccountVault :exec
INSERT INTO account_vaults(account_id) VALUES(sqlc.arg(account_id)) ON CONFLICT DO NOTHING;

-- name: LockAccountVault :one
-- Lock query; FOR UPDATE removed, engine-wide write serialization instead.
SELECT slots,gold,items FROM account_vaults WHERE account_id=sqlc.arg(account_id);

-- name: SaveAccountVaultItems :exec
UPDATE account_vaults SET items=sqlc.arg(items),updated_at=(CAST((julianday('now') - 2440587.5) * 86400000000 AS INTEGER)) WHERE account_id=sqlc.arg(account_id);

-- name: SaveAccountVault :exec
UPDATE account_vaults SET slots=sqlc.arg(slots),gold=sqlc.arg(gold),items=sqlc.arg(items),updated_at=(CAST((julianday('now') - 2440587.5) * 86400000000 AS INTEGER))
WHERE account_id=sqlc.arg(account_id);

-- name: SaveAccountVaultSlots :exec
UPDATE account_vaults SET slots=sqlc.arg(slots),updated_at=(CAST((julianday('now') - 2440587.5) * 86400000000 AS INTEGER)) WHERE account_id=sqlc.arg(account_id);

-- name: AccountVaultEvent :one
SELECT character_id,operation FROM account_vault_events WHERE account_id=sqlc.arg(account_id) AND event_key=sqlc.arg(event_key);

-- name: RecordAccountVaultEvent :exec
INSERT INTO account_vault_events(account_id,event_key,character_id,operation)
VALUES(sqlc.arg(account_id),sqlc.arg(event_key),sqlc.arg(character_id),sqlc.arg(operation));

-- name: EnsurePrimaryVault :exec
INSERT INTO character_vaults(character_id,slots,config_version)
SELECT id,sqlc.arg(slots),sqlc.arg(config_version) FROM characters
WHERE id=sqlc.arg(character_id) AND account_id=sqlc.arg(account_id) AND deleted_at IS NULL
ON CONFLICT DO NOTHING;

-- name: OwnedPrimaryVault :one
SELECT v.slots,v.items,v.config_version FROM character_vaults v JOIN characters c ON c.id=v.character_id
WHERE c.id=sqlc.arg(character_id) AND c.account_id=sqlc.arg(account_id) AND c.deleted_at IS NULL;

-- name: LockPrimaryVault :one
-- Lock query; FOR UPDATE removed, engine-wide write serialization instead.
SELECT slots,items,config_version FROM character_vaults WHERE character_id=sqlc.arg(character_id);

-- name: SavePrimaryVaultItems :exec
UPDATE character_vaults SET items=sqlc.arg(items),updated_at=(CAST((julianday('now') - 2440587.5) * 86400000000 AS INTEGER)) WHERE character_id=sqlc.arg(character_id);

-- name: SavePrimaryVaultSlots :exec
UPDATE character_vaults SET slots=sqlc.arg(slots),updated_at=(CAST((julianday('now') - 2440587.5) * 86400000000 AS INTEGER)) WHERE character_id=sqlc.arg(character_id);

-- name: EnsureSecondaryVault :exec
INSERT INTO character_secondary_vaults(character_id,slots,config_version)
SELECT id,sqlc.arg(slots),sqlc.arg(config_version) FROM characters
WHERE id=sqlc.arg(character_id) AND account_id=sqlc.arg(account_id) AND deleted_at IS NULL
ON CONFLICT DO NOTHING;

-- name: OwnedSecondaryVault :one
SELECT v.slots,v.items,v.config_version FROM character_secondary_vaults v JOIN characters c ON c.id=v.character_id
WHERE c.id=sqlc.arg(character_id) AND c.account_id=sqlc.arg(account_id) AND c.deleted_at IS NULL;

-- name: LockSecondaryVault :one
-- Lock query; FOR UPDATE removed, engine-wide write serialization instead.
SELECT slots,items,config_version FROM character_secondary_vaults WHERE character_id=sqlc.arg(character_id);

-- name: SaveSecondaryVaultItems :exec
UPDATE character_secondary_vaults SET items=sqlc.arg(items),updated_at=(CAST((julianday('now') - 2440587.5) * 86400000000 AS INTEGER)) WHERE character_id=sqlc.arg(character_id);

-- name: SaveSecondaryVaultSlots :exec
UPDATE character_secondary_vaults SET slots=sqlc.arg(slots),updated_at=(CAST((julianday('now') - 2440587.5) * 86400000000 AS INTEGER)) WHERE character_id=sqlc.arg(character_id);
