-- name: AccountMaterials :one
SELECT counts FROM account_material_storage WHERE account_id=sqlc.arg(account_id);

-- name: InitializeAccountMaterials :exec
INSERT INTO account_material_storage(account_id) VALUES(sqlc.arg(account_id)) ON CONFLICT(account_id) DO NOTHING;

-- name: LockAccountMaterials :one
SELECT counts FROM account_material_storage WHERE account_id=sqlc.arg(account_id) FOR UPDATE;

-- name: SaveAccountMaterials :exec
UPDATE account_material_storage SET counts=sqlc.arg(counts),updated_at=now() WHERE account_id=sqlc.arg(account_id);

-- name: LoadAccountVault :one
SELECT coalesce(v.slots,0)::integer AS slots,coalesce(v.gold,0)::bigint AS gold,
coalesce(v.items,'[]'::jsonb)::jsonb AS items FROM characters c LEFT JOIN account_vaults v ON v.account_id=c.account_id
WHERE c.id=sqlc.arg(character_id) AND c.account_id=sqlc.arg(account_id) AND c.deleted_at IS NULL;

-- name: EnsureAccountVault :exec
INSERT INTO account_vaults(account_id) VALUES(sqlc.arg(account_id)) ON CONFLICT DO NOTHING;

-- name: LockAccountVault :one
SELECT slots,gold,items FROM account_vaults WHERE account_id=sqlc.arg(account_id) FOR UPDATE;

-- name: SaveAccountVaultItems :exec
UPDATE account_vaults SET items=sqlc.arg(items),updated_at=now() WHERE account_id=sqlc.arg(account_id);

-- name: SaveAccountVault :exec
UPDATE account_vaults SET slots=sqlc.arg(slots),gold=sqlc.arg(gold),items=sqlc.arg(items),updated_at=now()
WHERE account_id=sqlc.arg(account_id);

-- name: SaveAccountVaultSlots :exec
UPDATE account_vaults SET slots=sqlc.arg(slots),updated_at=now() WHERE account_id=sqlc.arg(account_id);

-- name: AccountVaultEvent :one
SELECT character_id,operation FROM account_vault_events WHERE account_id=sqlc.arg(account_id) AND event_key=sqlc.arg(event_key);

-- name: RecordAccountVaultEvent :exec
INSERT INTO account_vault_events(account_id,event_key,character_id,operation)
VALUES(sqlc.arg(account_id),sqlc.arg(event_key),sqlc.arg(character_id),sqlc.arg(operation));

-- name: EnsurePrimaryVault :exec
INSERT INTO character_vaults(character_id,slots,config_version)
SELECT id,sqlc.arg(slots)::integer,sqlc.arg(config_version)::text FROM characters
WHERE id=sqlc.arg(character_id)::bigint AND account_id=sqlc.arg(account_id)::bigint AND deleted_at IS NULL
ON CONFLICT DO NOTHING;

-- name: OwnedPrimaryVault :one
SELECT v.slots,v.items,v.config_version FROM character_vaults v JOIN characters c ON c.id=v.character_id
WHERE c.id=sqlc.arg(character_id) AND c.account_id=sqlc.arg(account_id) AND c.deleted_at IS NULL;

-- name: LockPrimaryVault :one
SELECT slots,items,config_version FROM character_vaults WHERE character_id=sqlc.arg(character_id) FOR UPDATE;

-- name: SavePrimaryVaultItems :exec
UPDATE character_vaults SET items=sqlc.arg(items),updated_at=now() WHERE character_id=sqlc.arg(character_id);

-- name: SavePrimaryVaultSlots :exec
UPDATE character_vaults SET slots=sqlc.arg(slots),updated_at=now() WHERE character_id=sqlc.arg(character_id);

-- name: EnsureSecondaryVault :exec
INSERT INTO character_secondary_vaults(character_id,slots,config_version)
SELECT id,sqlc.arg(slots)::integer,sqlc.arg(config_version)::text FROM characters
WHERE id=sqlc.arg(character_id)::bigint AND account_id=sqlc.arg(account_id)::bigint AND deleted_at IS NULL
ON CONFLICT DO NOTHING;

-- name: OwnedSecondaryVault :one
SELECT v.slots,v.items,v.config_version FROM character_secondary_vaults v JOIN characters c ON c.id=v.character_id
WHERE c.id=sqlc.arg(character_id) AND c.account_id=sqlc.arg(account_id) AND c.deleted_at IS NULL;

-- name: LockSecondaryVault :one
SELECT slots,items,config_version FROM character_secondary_vaults WHERE character_id=sqlc.arg(character_id) FOR UPDATE;

-- name: SaveSecondaryVaultItems :exec
UPDATE character_secondary_vaults SET items=sqlc.arg(items),updated_at=now() WHERE character_id=sqlc.arg(character_id);

-- name: SaveSecondaryVaultSlots :exec
UPDATE character_secondary_vaults SET slots=sqlc.arg(slots),updated_at=now() WHERE character_id=sqlc.arg(character_id);
