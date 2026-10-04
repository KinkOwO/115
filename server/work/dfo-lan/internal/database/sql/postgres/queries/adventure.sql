-- name: AdventureLevel :one
SELECT COALESCE(a.level,1)::integer AS level FROM characters c
LEFT JOIN account_adventures a ON a.account_id=c.account_id
WHERE c.account_id=sqlc.arg(account_id) AND c.id=sqlc.arg(character_id) AND c.deleted_at IS NULL;

-- name: AdventureEventReceipt :one
SELECT outcome FROM character_events WHERE character_id=sqlc.arg(character_id)
AND event_key=sqlc.arg(event_key) AND model='account-adventure-v1';

-- name: EnsureOwnedAdventure :exec
INSERT INTO account_adventures(account_id,name,created_at)
SELECT a.id,sqlc.arg(name)::text,a.created_at FROM accounts a JOIN characters c ON c.account_id=a.id
WHERE a.id=sqlc.arg(account_id)::bigint AND c.id=sqlc.arg(character_id)::bigint AND c.deleted_at IS NULL
ON CONFLICT(account_id) DO NOTHING;

-- name: LoadAdventure :one
SELECT a.name,a.level,a.experience,a.created_at,a.data FROM account_adventures a JOIN characters c ON c.account_id=a.account_id
WHERE a.account_id=sqlc.arg(account_id) AND c.id=sqlc.arg(character_id) AND c.deleted_at IS NULL;

-- name: FirstActiveCharacterName :one
SELECT name FROM characters WHERE account_id=sqlc.arg(account_id) AND deleted_at IS NULL ORDER BY id LIMIT 1;

-- name: EnsureAdventure :exec
INSERT INTO account_adventures(account_id,name,created_at)
SELECT id,sqlc.arg(name)::text,created_at FROM accounts WHERE id=sqlc.arg(account_id)::bigint ON CONFLICT DO NOTHING;

-- name: LockAdventure :one
SELECT name,level,experience,created_at,data FROM account_adventures WHERE account_id=sqlc.arg(account_id) FOR UPDATE;

-- name: SaveAdventure :exec
UPDATE account_adventures SET level=sqlc.arg(level)::bigint,experience=sqlc.arg(experience),data=sqlc.arg(data)
WHERE account_id=sqlc.arg(account_id);

-- name: AdventureCollectionEquipment :one
SELECT COALESCE(a.data->'collection_equipment','{}'::jsonb)::jsonb AS equipment FROM characters c
LEFT JOIN account_adventures a ON a.account_id=c.account_id
WHERE c.account_id=sqlc.arg(account_id) AND c.id=sqlc.arg(character_id) AND c.deleted_at IS NULL;

-- name: AdventureEquipmentRegistered :one
SELECT COALESCE((a.data->'collection_equipment'->>(sqlc.arg(template)::bigint::text))::boolean,false)::boolean AS registered
FROM characters c LEFT JOIN account_adventures a ON a.account_id=c.account_id
WHERE c.account_id=sqlc.arg(account_id) AND c.id=sqlc.arg(character_id) AND c.deleted_at IS NULL;

-- name: ListFavor :many
SELECT npc_id,point FROM npc_favor WHERE character_id=sqlc.arg(character_id) AND point>0 ORDER BY npc_id;

-- name: ReserveFavorGift :one
INSERT INTO npc_favor(character_id,npc_id,point,daily_count,last_gift_day)
VALUES(sqlc.arg(character_id),sqlc.arg(npc_id),0,1,sqlc.arg(day)::text::date)
ON CONFLICT(character_id,npc_id) DO UPDATE SET
daily_count=CASE WHEN EXCLUDED.last_gift_day>npc_favor.last_gift_day OR npc_favor.last_gift_day IS NULL THEN 1 ELSE npc_favor.daily_count+1 END,
last_gift_day=GREATEST(EXCLUDED.last_gift_day,npc_favor.last_gift_day),point=npc_favor.point,updated_at=now()
RETURNING point,daily_count,last_gift_day::text;

-- name: SaveFavorPoint :exec
UPDATE npc_favor SET point=sqlc.arg(point),updated_at=now()
WHERE character_id=sqlc.arg(character_id) AND npc_id=sqlc.arg(npc_id);

-- name: EnsureBleedingMineRewards :exec
INSERT INTO account_bleeding_mine_rewards(account_id) VALUES(sqlc.arg(account_id)) ON CONFLICT DO NOTHING;

-- name: LockBleedingMineRewards :one
SELECT state FROM account_bleeding_mine_rewards WHERE account_id=sqlc.arg(account_id) FOR UPDATE;

-- name: SaveBleedingMineRewards :exec
UPDATE account_bleeding_mine_rewards SET state=sqlc.arg(state),updated_at=now() WHERE account_id=sqlc.arg(account_id);

-- name: BleedingMineTeams :many
SELECT team,members FROM account_bleeding_mine_teams WHERE account_id=sqlc.arg(account_id);

-- name: SaveBleedingMineTeam :exec
INSERT INTO account_bleeding_mine_teams(account_id,team,members)
VALUES(sqlc.arg(account_id),sqlc.arg(team),sqlc.arg(members))
ON CONFLICT(account_id,team) DO UPDATE SET members=EXCLUDED.members,updated_at=now();
