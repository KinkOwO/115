-- SQLite query fork of sql/postgres/queries/adventure.sql (dual-engine, S3).
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
--   * jsonb operators -> JSON1: data->'k' -> json_extract(x,'$.k'),
--     GREATEST(a,b) -> scalar max(a,b). now() -> integer microseconds, matching
--     the DDL defaults and the driver's unix_micro representation.
--   * DATE columns hold integer microseconds (DDL header, D29). npc_favor's
--     last_gift_day is therefore written from the caller's yyyy-mm-dd string
--     through julianday() and read back as yyyy-mm-dd text through strftime(),
--     which keeps the generated Go type string in both directions.
--   * account_bleeding_mine_teams.members is a PostgreSQL bigint[] and a JSON
--     array in a BLOB here (D27); sqlc types it []byte, so the caller owns the
--     JSON encoding.
--   * The RETURNING alias of ReserveFavorGift is not honoured by sqlc's SQLite
--     engine: the text day column is named positionally (Column3) where
--     PostgreSQL names it LastGiftDay. Value and Go type (string) are unchanged.

-- name: AdventureLevel :one
SELECT CAST(COALESCE(a.level,1) AS INTEGER) AS level FROM characters c
LEFT JOIN account_adventures a ON a.account_id=c.account_id
WHERE c.account_id=sqlc.arg(account_id) AND c.id=sqlc.arg(character_id) AND c.deleted_at IS NULL;

-- name: AdventureEventReceipt :one
SELECT outcome FROM character_events WHERE character_id=sqlc.arg(character_id)
AND event_key=sqlc.arg(event_key) AND model='account-adventure-v1';

-- name: EnsureOwnedAdventure :exec
INSERT INTO account_adventures(account_id,name,created_at)
SELECT a.id,CAST(sqlc.arg(name) AS TEXT),a.created_at FROM accounts a JOIN characters c ON c.account_id=a.id
WHERE a.id=CAST(sqlc.arg(account_id) AS INTEGER) AND c.id=CAST(sqlc.arg(character_id) AS INTEGER) AND c.deleted_at IS NULL
ON CONFLICT(account_id) DO NOTHING;

-- name: LoadAdventure :one
SELECT a.name,a.level,a.experience,a.created_at,a.data FROM account_adventures a JOIN characters c ON c.account_id=a.account_id
WHERE a.account_id=sqlc.arg(account_id) AND c.id=sqlc.arg(character_id) AND c.deleted_at IS NULL;

-- name: FirstActiveCharacterName :one
SELECT name FROM characters WHERE account_id=sqlc.arg(account_id) AND deleted_at IS NULL ORDER BY id LIMIT 1;

-- name: EnsureAdventure :exec
INSERT INTO account_adventures(account_id,name,created_at)
SELECT id,CAST(sqlc.arg(name) AS TEXT),created_at FROM accounts WHERE id=CAST(sqlc.arg(account_id) AS INTEGER) ON CONFLICT DO NOTHING;

-- name: LockAdventure :one
-- Lock query; the FOR UPDATE is replaced by the engine-wide BEGIN IMMEDIATE write
-- serialization. Name and projection are unchanged.
SELECT name,level,experience,created_at,data FROM account_adventures WHERE account_id=sqlc.arg(account_id);

-- name: SaveAdventure :exec
UPDATE account_adventures SET level=CAST(sqlc.arg(level) AS INTEGER),experience=sqlc.arg(experience),data=sqlc.arg(data)
WHERE account_id=sqlc.arg(account_id);

-- name: AdventureCollectionEquipment :one
SELECT CAST(COALESCE(json_extract(a.data,'$.collection_equipment'),'{}') AS BLOB) AS equipment FROM characters c
LEFT JOIN account_adventures a ON a.account_id=c.account_id
WHERE c.account_id=sqlc.arg(account_id) AND c.id=sqlc.arg(character_id) AND c.deleted_at IS NULL;

-- name: AdventureEquipmentRegistered :one
SELECT CAST(COALESCE(json_extract(a.data,'$.collection_equipment.'||CAST(CAST(sqlc.arg(template) AS INTEGER) AS TEXT)),0) AS BOOLEAN) AS registered
FROM characters c LEFT JOIN account_adventures a ON a.account_id=c.account_id
WHERE c.account_id=sqlc.arg(account_id) AND c.id=sqlc.arg(character_id) AND c.deleted_at IS NULL;

-- name: ListFavor :many
SELECT npc_id,point FROM npc_favor WHERE character_id=sqlc.arg(character_id) AND point>0 ORDER BY npc_id;

-- name: ReserveFavorGift :one
-- GREATEST(a,b) -> max(a,b), with the NULL handling spelled out: the scalar
-- max() returns NULL when any argument is NULL while GREATEST ignores NULLs, and
-- npc_favor.last_gift_day is nullable. The day is stored as DATE microseconds
-- (julianday) and returned as yyyy-mm-dd text (strftime).
INSERT INTO npc_favor(character_id,npc_id,point,daily_count,last_gift_day)
VALUES(sqlc.arg(character_id),sqlc.arg(npc_id),0,1,
CAST((julianday(CAST(sqlc.arg(day) AS TEXT)) - 2440587.5) * 86400000000 AS DATE))
ON CONFLICT(character_id,npc_id) DO UPDATE SET
daily_count=CASE WHEN EXCLUDED.last_gift_day>npc_favor.last_gift_day OR npc_favor.last_gift_day IS NULL THEN 1 ELSE npc_favor.daily_count+1 END,
last_gift_day=max(EXCLUDED.last_gift_day,coalesce(npc_favor.last_gift_day,EXCLUDED.last_gift_day)),point=npc_favor.point,
updated_at=(CAST((julianday('now') - 2440587.5) * 86400000000 AS INTEGER))
RETURNING point,daily_count,CAST(strftime('%Y-%m-%d',last_gift_day/1000000,'unixepoch') AS TEXT) AS "last_gift_day";

-- name: SaveFavorPoint :exec
UPDATE npc_favor SET point=sqlc.arg(point),updated_at=(CAST((julianday('now') - 2440587.5) * 86400000000 AS INTEGER))
WHERE character_id=sqlc.arg(character_id) AND npc_id=sqlc.arg(npc_id);

-- name: EnsureBleedingMineRewards :exec
INSERT INTO account_bleeding_mine_rewards(account_id) VALUES(sqlc.arg(account_id)) ON CONFLICT DO NOTHING;

-- name: LockBleedingMineRewards :one
-- Lock query; lock clause removed (see LockAdventure).
SELECT state FROM account_bleeding_mine_rewards WHERE account_id=sqlc.arg(account_id);

-- name: SaveBleedingMineRewards :exec
UPDATE account_bleeding_mine_rewards SET state=sqlc.arg(state),updated_at=(CAST((julianday('now') - 2440587.5) * 86400000000 AS INTEGER)) WHERE account_id=sqlc.arg(account_id);

-- name: BleedingMineTeams :many
SELECT team,members FROM account_bleeding_mine_teams WHERE account_id=sqlc.arg(account_id);

-- name: SaveBleedingMineTeam :exec
INSERT INTO account_bleeding_mine_teams(account_id,team,members)
VALUES(sqlc.arg(account_id),sqlc.arg(team),sqlc.arg(members))
ON CONFLICT(account_id,team) DO UPDATE SET members=EXCLUDED.members,updated_at=(CAST((julianday('now') - 2440587.5) * 86400000000 AS INTEGER));
