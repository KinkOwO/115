-- SQLite query fork of sql/postgres/queries/fixtures.sql (dual-engine, S3).
--
-- The PostgreSQL file stays untouched and remains authoritative for the PG path.
--
-- What this file is: it is the query half of the isolated charactercheck fixture.
-- The isolation itself is NOT expressed in this file - the PostgreSQL fixture
-- creates a throwaway schema and sets search_path from Go
-- (internal/database/fixture.go), and the SQLite fork isolates the same way with
-- one temporary database file per test (design doc D18). Every statement below is
-- ordinary DML against tables that exist in the SQLite schema, so porting it does
-- not weaken the fixture's guarantee; the guarantee lives in the Go fixture.
--
-- NOT PORTED: FixtureSchema (:one) ran SELECT current_schema()::text. That is a
-- PostgreSQL catalog diagnostic with no SQLite equivalent (design doc D19 and
-- section 3.4 list FixtureSchema as an engine-specific diagnostic); the SQLite
-- fixture validates its container from Go instead (PRAGMA database_list / the
-- temp file path). Omitted rather than guessed.
--
-- Dialect rules applied:
--   * JSON parameters are never cast (D24): the columns are BLOB, so a plain
--     sqlc.arg keeps json.RawMessage in both directions. With the per-column json
--     overrides in sqlc.yaml this now holds for character_vaults.items as well.
--   * WHERE a JSON value is written with json_set/json_patch, the parameter is
--     wrapped in json(). Measured on the pinned driver: json_set/json_patch treat a
--     plain value as a literal, and a BLOB value is rejected outright ("JSON cannot
--     hold BLOB values"), so json() is what makes the parameter a JSON value (guide
--     section 1: "wrap in json(...) when a JSON value is needed"). The wrapper is
--     CAST(... AS BLOB) inside json() to give sqlc a concrete []byte parameter:
--     CAST(... AS JSON) would look right to sqlc (json.RawMessage) but SQLite gives
--     JSON NUMERIC affinity and destroys the value (measured: [1,2] -> 0,
--     {"a":1} -> 0), so it must not be used.
--   * The json_set/json_patch RESULT is CAST back to BLOB because JSON1 returns
--     TEXT, and a TEXT value in the BLOB column reads back as a Go string that
--     database/sql cannot scan into json.RawMessage (D24).
--   * state||fields (jsonb concatenation) -> json_patch(state,json(...)). For the
--     objects used here both are a right-biased merge of members; note that
--     json_patch follows RFC 7386 and therefore merges nested objects recursively
--     and deletes members whose value is null, while PG || replaces a top-level
--     member wholesale. The fixture only merges top-level scalars.
--   * (x IS NOT NULL)::boolean -> CAST((x IS NOT NULL) AS BOOLEAN).
--   * request->>'source' -> json_extract(request,'$.source') with CAST(... AS TEXT).
--   * Parameters bound to a column and plain column projections are bare, so they
--     keep the schema widths and json types (used/used_max int32, quest_id int32,
--     character_id int64, quest progress bigint via CAST because the PostgreSQL
--     file casts it to bigint). A parameter compared ONLY with a literal infers
--     interface{} (measured), so the 0-sentinels in FixtureEventCount,
--     FixtureQuestCount and FixtureCashInventoryCount keep an explicit CAST(... AS
--     INTEGER), matching the PostgreSQL bigint width.

-- name: FixtureAccountCurrency :exec
INSERT INTO account_currency(account_id,cera) VALUES(sqlc.arg(account_id),sqlc.arg(cera))
ON CONFLICT(account_id) DO UPDATE SET cera=EXCLUDED.cera;

-- name: FixtureCharacterSnapshot :exec
UPDATE characters SET state=sqlc.arg(state),config_version=sqlc.arg(config_version)
WHERE id=sqlc.arg(character_id);

-- name: FixtureFatigueRoomStats :one
SELECT count(*) AS count,CAST(coalesce(sum(cost),0) AS INTEGER) AS cost
FROM character_fatigue_rooms WHERE character_id=sqlc.arg(character_id);

-- name: FixtureDeleteBirth :exec
DELETE FROM character_birth WHERE character_id=sqlc.arg(character_id);

-- name: FixtureArchivedCharacter :one
SELECT CAST((deleted_at IS NOT NULL) AS BOOLEAN) AS archived,state FROM characters WHERE id=sqlc.arg(character_id);

-- name: FixtureEventCount :one
-- CAST is required: a parameter compared only with the literal 0 infers
-- interface{} (measured), while the PostgreSQL file casts it to bigint.
SELECT count(*) FROM character_events WHERE (CAST(sqlc.arg(character_id) AS INTEGER)=0 OR character_id=CAST(sqlc.arg(character_id) AS INTEGER));

-- name: FixtureVaultItems :exec
UPDATE character_vaults SET items=sqlc.arg(items) WHERE character_id=sqlc.arg(character_id);

-- name: FixtureFatigueUsed :exec
UPDATE character_fatigue SET used=sqlc.arg(used) WHERE character_id=sqlc.arg(character_id);

-- name: FixtureQuestProgress :exec
UPDATE character_quests SET progress=CAST(sqlc.arg(progress) AS INTEGER)
WHERE character_id=sqlc.arg(character_id) AND quest_id=sqlc.arg(quest_id);

-- name: FixtureLegacyQuest :exec
UPDATE character_quests SET progress=0,progress_model='legacy-zero'
WHERE character_id=sqlc.arg(character_id) AND quest_id=sqlc.arg(quest_id);

-- name: FixtureQuestRepairCount :one
SELECT count(*) FROM character_quest_repairs WHERE character_id=sqlc.arg(character_id);

-- name: FixtureCompletedQuest :exec
INSERT INTO character_quests(character_id,quest_id,status,progress,config_version,progress_model)
VALUES(sqlc.arg(character_id),sqlc.arg(quest_id),'completed',0,sqlc.arg(config_version),sqlc.arg(progress_model));

-- name: FixtureQuestRewardCount :one
SELECT count(*) FROM character_quest_rewards WHERE character_id=sqlc.arg(character_id)
AND (quest_id=sqlc.arg(quest_id) OR sqlc.arg(quest_id)=0);

-- name: FixtureMapClearCount :one
SELECT count(*) FROM character_map_clears WHERE character_id=sqlc.arg(character_id) AND run_id=sqlc.arg(run_id);

-- name: FixtureCharacterState :one
SELECT state FROM characters WHERE id=sqlc.arg(character_id);

-- name: FixtureMonsterEventCount :one
SELECT count(*) FROM character_events
WHERE character_id=sqlc.arg(character_id) AND event_key LIKE 'monster:%';

-- name: FixtureInventoryItems :exec
UPDATE characters SET state=CAST(json_set(state,'$.inventory.items',json(CAST(sqlc.arg(items) AS BLOB))) AS BLOB)
WHERE id=sqlc.arg(character_id);

-- name: FixtureWalletGold :exec
UPDATE characters SET state=CAST(json_set(state,'$.inventory.gold',json(CAST(sqlc.arg(gold) AS BLOB))) AS BLOB)
WHERE id=sqlc.arg(character_id);

-- name: FixtureMergeCharacterFields :exec
UPDATE characters SET state=CAST(json_patch(state,json(CAST(sqlc.arg(fields) AS BLOB))) AS BLOB) WHERE id=sqlc.arg(character_id);

-- name: FixtureReadCharacterSnapshot :one
SELECT state,config_version FROM characters WHERE id=sqlc.arg(character_id);

-- name: FixtureCashOrderCount :one
SELECT count(*) FROM cash_orders WHERE account_id=sqlc.arg(account_id);

-- name: FixtureCashInventoryCount :one
SELECT count(*) FROM cash_inventory WHERE account_id=sqlc.arg(account_id)
AND (CAST(sqlc.arg(claim_state) AS INTEGER)=0
 OR (CAST(sqlc.arg(claim_state) AS INTEGER)=1 AND claimed_at IS NULL)
 OR (CAST(sqlc.arg(claim_state) AS INTEGER)=2 AND claimed_at IS NOT NULL))
AND (template=CAST(sqlc.arg(template) AS INTEGER) OR CAST(sqlc.arg(template) AS INTEGER)=0)
AND (amount=sqlc.arg(amount) OR sqlc.arg(amount)=0);

-- name: FixtureCashOrderSource :one
SELECT CAST(coalesce(json_extract(request,'$.source'),'') AS TEXT) FROM cash_orders
WHERE account_id=sqlc.arg(account_id) AND order_key=sqlc.arg(order_key);

-- name: FixtureQuestRecordSeed :exec
INSERT INTO character_quests(character_id,quest_id,status,config_version,progress_model)
VALUES(sqlc.arg(character_id),sqlc.arg(quest_id),sqlc.arg(status),sqlc.arg(config_version),sqlc.arg(progress_model));

-- name: FixtureQuestCount :one
SELECT count(*) FROM character_quests
WHERE (CAST(sqlc.arg(character_id) AS INTEGER)=0 OR character_id=CAST(sqlc.arg(character_id) AS INTEGER));

-- name: FixtureQuestRecord :one
SELECT status,progress_model FROM character_quests
WHERE character_id=sqlc.arg(character_id) AND quest_id=sqlc.arg(quest_id);

-- name: FixtureFatigueUsage :exec
UPDATE character_fatigue SET used=sqlc.arg(used),used_max=sqlc.arg(used_max)
WHERE character_id=sqlc.arg(character_id);
