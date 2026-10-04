-- SQLite query fork of sql/postgres/queries/commerce.sql (dual-engine, S3).
--
-- The PostgreSQL file stays untouched and remains authoritative for the PG path.
-- Dialect rules applied here are the ones in docs/sqlite-query-port-guide.md and
-- restated in context in the header of sqlite/queries/core.sql:
--   * FOR UPDATE / FOR NO KEY UPDATE / FOR SHARE -> REMOVED. SQLite has no row
--     locks and rejects the clause; writers are serialized by BEGIN IMMEDIATE.
--     Lock queries keep their name and their ownership predicate.
--   * sqlc.arg(x)::type -> CAST(sqlc.arg(x) AS type). Spike rule R-1: a bare
--     expression infers interface{} and cannot enter the shared query interface,
--     so every expression result gets an explicit CAST.
--   * JSON parameters are deliberately NOT wrapped in CAST(... AS TEXT): that
--     turns the parameter into a string, which database/sql cannot scan into
--     json.RawMessage. JSON columns are BLOB here, and a BLOB parameter/result is
--     generated as []byte, which json.RawMessage is assignable to.
--   * now() -> integer microseconds, matching the DDL defaults and the driver's
--     unix_micro representation.
--   * account_premiums.end_time is a plain INTEGER epoch in both engines; it is
--     NOT one of the TIMESTAMP columns.
--   * jsonb containment (@>) has no SQLite operator; the two migration-candidate
--     queries use json_each over the same JSON path, which matches exactly the
--     same single-key object ({"Template": N}) that PostgreSQL @> accepts.
--   * sqlc.narg(character_id)::bigint -> CAST(sqlc.narg(character_id) AS INTEGER),
--     which generates *int64 under emit_pointers_for_null_types.

-- name: CountAccountShopPurchases :one
SELECT count(*) FROM character_shop_purchases WHERE account_id=sqlc.arg(account_id)
AND npc_id=sqlc.arg(npc_id) AND template=sqlc.arg(template) AND bought_at>=sqlc.arg(window_start);

-- name: CountCharacterShopPurchases :one
SELECT count(*) FROM character_shop_purchases WHERE character_id=sqlc.arg(character_id)
AND npc_id=sqlc.arg(npc_id) AND template=sqlc.arg(template) AND bought_at>=sqlc.arg(window_start);

-- name: RecordShopPurchase :exec
INSERT INTO character_shop_purchases(account_id,character_id,npc_id,template)
VALUES(sqlc.arg(account_id),sqlc.arg(character_id),sqlc.arg(npc_id),sqlc.arg(template));

-- name: GrantHistory :many
SELECT grant_id,CAST(coalesce(character_id,0) AS INTEGER) AS character_id,operator,reason,request,created_at
FROM admin_grants WHERE account_id=sqlc.arg(account_id) ORDER BY created_at DESC LIMIT CAST(sqlc.arg(max_entries) AS INTEGER);

-- name: AccountCera :one
SELECT CAST(coalesce((SELECT cera FROM account_currency WHERE account_id=sqlc.arg(account_id)),0) AS INTEGER);

-- name: ActivePremiums :many
SELECT premium_type,end_time FROM account_premiums
WHERE account_id=sqlc.arg(account_id) AND end_time>CAST(sqlc.arg(now_epoch) AS INTEGER) ORDER BY premium_type;

-- name: HasActivePremium :one
SELECT EXISTS(SELECT 1 FROM account_premiums WHERE account_id=sqlc.arg(account_id)
AND premium_type=sqlc.arg(premium_type) AND end_time>CAST(sqlc.arg(now_epoch) AS INTEGER));

-- name: LockPremiumExpiry :one
-- Lock query; the FOR UPDATE is replaced by the engine-wide BEGIN IMMEDIATE write
-- serialization. Name and projection are unchanged.
SELECT end_time FROM account_premiums WHERE account_id=sqlc.arg(account_id)
AND premium_type=sqlc.arg(premium_type);

-- name: SavePremiumExpiry :exec
INSERT INTO account_premiums(account_id,premium_type,end_time,updated_at)
VALUES(sqlc.arg(account_id),sqlc.arg(premium_type),sqlc.arg(end_time),(CAST((julianday('now') - 2440587.5) * 86400000000 AS INTEGER)))
ON CONFLICT(account_id,premium_type) DO UPDATE SET end_time=EXCLUDED.end_time,updated_at=(CAST((julianday('now') - 2440587.5) * 86400000000 AS INTEGER));

-- name: EnsureAccountCurrency :exec
INSERT INTO account_currency(account_id,cera) VALUES(sqlc.arg(account_id),0) ON CONFLICT DO NOTHING;

-- name: LockAccountCurrency :one
-- Lock query; lock clause removed (see LockPremiumExpiry).
SELECT cera FROM account_currency WHERE account_id=sqlc.arg(account_id);

-- name: SaveAccountCurrency :exec
UPDATE account_currency SET cera=sqlc.arg(cera),updated_at=(CAST((julianday('now') - 2440587.5) * 86400000000 AS INTEGER)) WHERE account_id=sqlc.arg(account_id);

-- name: ClaimAdminGrant :execrows
INSERT INTO admin_grants(grant_id,account_id,character_id,request,receipt,operator,reason)
VALUES(sqlc.arg(grant_id),sqlc.arg(account_id),CAST(sqlc.narg(character_id) AS INTEGER),
sqlc.arg(request),CAST('{}' AS BLOB),sqlc.arg(operator),sqlc.arg(reason)) ON CONFLICT(grant_id) DO NOTHING;

-- name: AdminGrantReceipt :one
SELECT receipt FROM admin_grants WHERE grant_id=sqlc.arg(grant_id);

-- name: AdjustAccountCurrency :one
UPDATE account_currency SET cera=cera+CAST(sqlc.arg(adjustment) AS INTEGER),updated_at=(CAST((julianday('now') - 2440587.5) * 86400000000 AS INTEGER))
WHERE account_id=sqlc.arg(account_id) RETURNING cera;

-- name: SaveAdminGrantReceipt :exec
UPDATE admin_grants SET receipt=sqlc.arg(receipt) WHERE grant_id=sqlc.arg(grant_id);

-- name: CoinItemMigrationCandidates :many
-- PostgreSQL: state->'inventory'->'items' @> '[{"Template": 1}]'. The JSON1
-- spelling walks the same path and keeps the same single-key element match.
SELECT id,state FROM characters
WHERE EXISTS(SELECT 1 FROM json_each(json_extract(state,'$.inventory.items')) j WHERE json_extract(j.value,'$.Template')=1);

-- name: PackagePlaceholderMigrationCandidates :many
SELECT id,state FROM characters
WHERE EXISTS(SELECT 1 FROM json_each(json_extract(state,'$.inventory.items')) j WHERE json_extract(j.value,'$.Template')=590722921)
OR EXISTS(SELECT 1 FROM json_each(json_extract(state,'$.inventory.items')) j WHERE json_extract(j.value,'$.Template')=590722922);

-- name: CashOrderVaultSpace :one
SELECT CAST(coalesce(json_extract(request,'$.vault_space'),0) AS INTEGER) AS vault_space FROM cash_orders
WHERE account_id=sqlc.arg(account_id) AND character_id=sqlc.arg(character_id) AND order_key=sqlc.arg(order_key);

-- name: CashOrderReceipt :one
SELECT digest,receipt FROM cash_orders WHERE account_id=sqlc.arg(account_id) AND order_key=sqlc.arg(order_key);

-- name: RecordCashOrder :exec
INSERT INTO cash_orders(account_id,order_key,character_id,digest,request,receipt)
VALUES(sqlc.arg(account_id),sqlc.arg(order_key),sqlc.arg(character_id),sqlc.arg(digest),sqlc.arg(request),CAST('{}' AS BLOB));

-- name: RecordCashInventory :one
INSERT INTO cash_inventory(account_id,character_id,order_key,line_index,product,template,amount)
VALUES(sqlc.arg(account_id),sqlc.arg(character_id),sqlc.arg(order_key),sqlc.arg(line_index),
sqlc.arg(product),sqlc.arg(template),sqlc.arg(amount)) RETURNING id;

-- name: MarkCashOrderDelivered :exec
UPDATE cash_inventory SET claimed_at=(CAST((julianday('now') - 2440587.5) * 86400000000 AS INTEGER)) WHERE account_id=sqlc.arg(account_id) AND order_key=sqlc.arg(order_key);

-- name: SaveCashOrderReceipt :exec
UPDATE cash_orders SET receipt=sqlc.arg(receipt) WHERE account_id=sqlc.arg(account_id) AND order_key=sqlc.arg(order_key);

-- name: CashInventory :many
SELECT i.id,i.product,i.template,i.amount FROM cash_inventory i JOIN characters c ON c.id=i.character_id
WHERE i.account_id=sqlc.arg(account_id) AND i.character_id=sqlc.arg(character_id)
AND c.account_id=sqlc.arg(account_id) AND c.deleted_at IS NULL AND i.claimed_at IS NULL ORDER BY i.id;
