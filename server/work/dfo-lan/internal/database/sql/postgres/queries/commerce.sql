-- name: CountAccountShopPurchases :one
SELECT count(*) FROM character_shop_purchases WHERE account_id=sqlc.arg(account_id)
AND npc_id=sqlc.arg(npc_id) AND template=sqlc.arg(template) AND bought_at>=sqlc.arg(window_start)::timestamptz;

-- name: CountCharacterShopPurchases :one
SELECT count(*) FROM character_shop_purchases WHERE character_id=sqlc.arg(character_id)
AND npc_id=sqlc.arg(npc_id) AND template=sqlc.arg(template) AND bought_at>=sqlc.arg(window_start)::timestamptz;

-- name: RecordShopPurchase :exec
INSERT INTO character_shop_purchases(account_id,character_id,npc_id,template)
VALUES(sqlc.arg(account_id),sqlc.arg(character_id),sqlc.arg(npc_id),sqlc.arg(template));

-- name: GrantHistory :many
SELECT grant_id,coalesce(character_id,0)::bigint AS character_id,operator,reason,request,created_at
FROM admin_grants WHERE account_id=sqlc.arg(account_id) ORDER BY created_at DESC LIMIT sqlc.arg(max_entries)::integer;

-- name: AccountCera :one
SELECT coalesce((SELECT cera FROM account_currency WHERE account_id=sqlc.arg(account_id)),0)::bigint;

-- name: ActivePremiums :many
SELECT premium_type,end_time FROM account_premiums
WHERE account_id=sqlc.arg(account_id) AND end_time>sqlc.arg(now_epoch)::bigint ORDER BY premium_type;

-- name: HasActivePremium :one
SELECT EXISTS(SELECT 1 FROM account_premiums WHERE account_id=sqlc.arg(account_id)
AND premium_type=sqlc.arg(premium_type) AND end_time>sqlc.arg(now_epoch)::bigint);

-- name: LockPremiumExpiry :one
SELECT end_time FROM account_premiums WHERE account_id=sqlc.arg(account_id)
AND premium_type=sqlc.arg(premium_type) FOR UPDATE;

-- name: SavePremiumExpiry :exec
INSERT INTO account_premiums(account_id,premium_type,end_time,updated_at)
VALUES(sqlc.arg(account_id),sqlc.arg(premium_type),sqlc.arg(end_time),now())
ON CONFLICT(account_id,premium_type) DO UPDATE SET end_time=EXCLUDED.end_time,updated_at=now();

-- name: EnsureAccountCurrency :exec
INSERT INTO account_currency(account_id,cera) VALUES(sqlc.arg(account_id),0) ON CONFLICT DO NOTHING;

-- name: LockAccountCurrency :one
SELECT cera FROM account_currency WHERE account_id=sqlc.arg(account_id) FOR UPDATE;

-- name: SaveAccountCurrency :exec
UPDATE account_currency SET cera=sqlc.arg(cera),updated_at=now() WHERE account_id=sqlc.arg(account_id);

-- name: ClaimAdminGrant :execrows
INSERT INTO admin_grants(grant_id,account_id,character_id,request,receipt,operator,reason)
VALUES(sqlc.arg(grant_id),sqlc.arg(account_id),sqlc.narg(character_id)::bigint,
sqlc.arg(request),'{}'::jsonb,sqlc.arg(operator),sqlc.arg(reason)) ON CONFLICT(grant_id) DO NOTHING;

-- name: AdminGrantReceipt :one
SELECT receipt FROM admin_grants WHERE grant_id=sqlc.arg(grant_id);

-- name: AdjustAccountCurrency :one
UPDATE account_currency SET cera=cera+sqlc.arg(adjustment)::bigint,updated_at=now()
WHERE account_id=sqlc.arg(account_id) RETURNING cera;

-- name: SaveAdminGrantReceipt :exec
UPDATE admin_grants SET receipt=sqlc.arg(receipt) WHERE grant_id=sqlc.arg(grant_id);

-- name: CoinItemMigrationCandidates :many
SELECT id,state FROM characters WHERE state->'inventory'->'items' @> '[{"Template": 1}]';

-- name: PackagePlaceholderMigrationCandidates :many
SELECT id,state FROM characters WHERE state->'inventory'->'items' @> '[{"Template": 590722921}]'
OR state->'inventory'->'items' @> '[{"Template": 590722922}]';

-- name: CashOrderVaultSpace :one
SELECT coalesce((request->>'vault_space')::integer,0)::integer AS vault_space FROM cash_orders
WHERE account_id=sqlc.arg(account_id) AND character_id=sqlc.arg(character_id) AND order_key=sqlc.arg(order_key);

-- name: CashOrderReceipt :one
SELECT digest,receipt FROM cash_orders WHERE account_id=sqlc.arg(account_id) AND order_key=sqlc.arg(order_key);

-- name: RecordCashOrder :exec
INSERT INTO cash_orders(account_id,order_key,character_id,digest,request,receipt)
VALUES(sqlc.arg(account_id),sqlc.arg(order_key),sqlc.arg(character_id),sqlc.arg(digest),sqlc.arg(request),'{}');

-- name: RecordCashInventory :one
INSERT INTO cash_inventory(account_id,character_id,order_key,line_index,product,template,amount)
VALUES(sqlc.arg(account_id),sqlc.arg(character_id),sqlc.arg(order_key),sqlc.arg(line_index),
sqlc.arg(product),sqlc.arg(template),sqlc.arg(amount)) RETURNING id;

-- name: MarkCashOrderDelivered :exec
UPDATE cash_inventory SET claimed_at=now() WHERE account_id=sqlc.arg(account_id) AND order_key=sqlc.arg(order_key);

-- name: SaveCashOrderReceipt :exec
UPDATE cash_orders SET receipt=sqlc.arg(receipt) WHERE account_id=sqlc.arg(account_id) AND order_key=sqlc.arg(order_key);

-- name: CashInventory :many
SELECT i.id,i.product,i.template,i.amount FROM cash_inventory i JOIN characters c ON c.id=i.character_id
WHERE i.account_id=sqlc.arg(account_id) AND i.character_id=sqlc.arg(character_id)
AND c.account_id=sqlc.arg(account_id) AND c.deleted_at IS NULL AND i.claimed_at IS NULL ORDER BY i.id;
