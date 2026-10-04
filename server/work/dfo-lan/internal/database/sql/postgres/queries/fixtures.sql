-- These queries are used only by the isolated charactercheck fixture.
-- name: FixtureSchema :one
SELECT current_schema()::text;

-- name: FixtureAccountCurrency :exec
INSERT INTO account_currency(account_id,cera) VALUES(sqlc.arg(account_id),sqlc.arg(cera))
ON CONFLICT(account_id) DO UPDATE SET cera=EXCLUDED.cera;

-- name: FixtureCharacterSnapshot :exec
UPDATE characters SET state=sqlc.arg(state),config_version=sqlc.arg(config_version)
WHERE id=sqlc.arg(character_id);

-- name: FixtureFatigueRoomStats :one
SELECT count(*) AS count,coalesce(sum(cost),0)::bigint AS cost
FROM character_fatigue_rooms WHERE character_id=sqlc.arg(character_id);

-- name: FixtureDeleteBirth :exec
DELETE FROM character_birth WHERE character_id=sqlc.arg(character_id);

-- name: FixtureArchivedCharacter :one
SELECT (deleted_at IS NOT NULL)::boolean AS archived,state FROM characters WHERE id=sqlc.arg(character_id);

-- name: FixtureEventCount :one
SELECT count(*) FROM character_events WHERE (sqlc.arg(character_id)::bigint=0 OR character_id=sqlc.arg(character_id)::bigint);

-- name: FixtureVaultItems :exec
UPDATE character_vaults SET items=sqlc.arg(items) WHERE character_id=sqlc.arg(character_id);

-- name: FixtureFatigueUsed :exec
UPDATE character_fatigue SET used=sqlc.arg(used)::integer WHERE character_id=sqlc.arg(character_id);

-- name: FixtureQuestProgress :exec
UPDATE character_quests SET progress=sqlc.arg(progress)::bigint
WHERE character_id=sqlc.arg(character_id) AND quest_id=sqlc.arg(quest_id)::integer;

-- name: FixtureLegacyQuest :exec
UPDATE character_quests SET progress=0,progress_model='legacy-zero'
WHERE character_id=sqlc.arg(character_id) AND quest_id=sqlc.arg(quest_id)::integer;

-- name: FixtureQuestRepairCount :one
SELECT count(*) FROM character_quest_repairs WHERE character_id=sqlc.arg(character_id);

-- name: FixtureCompletedQuest :exec
INSERT INTO character_quests(character_id,quest_id,status,progress,config_version,progress_model)
VALUES(sqlc.arg(character_id),sqlc.arg(quest_id)::integer,'completed',0,sqlc.arg(config_version),sqlc.arg(progress_model));

-- name: FixtureQuestRewardCount :one
SELECT count(*) FROM character_quest_rewards WHERE character_id=sqlc.arg(character_id)
AND (sqlc.arg(quest_id)::integer=0 OR quest_id=sqlc.arg(quest_id)::integer);

-- name: FixtureMapClearCount :one
SELECT count(*) FROM character_map_clears WHERE character_id=sqlc.arg(character_id) AND run_id=sqlc.arg(run_id);

-- name: FixtureCharacterState :one
SELECT state FROM characters WHERE id=sqlc.arg(character_id);

-- name: FixtureMonsterEventCount :one
SELECT count(*) FROM character_events
WHERE character_id=sqlc.arg(character_id) AND event_key LIKE 'monster:%';

-- name: FixtureInventoryItems :exec
UPDATE characters SET state=jsonb_set(state,'{inventory,items}',sqlc.arg(items)::jsonb)
WHERE id=sqlc.arg(character_id);

-- name: FixtureWalletGold :exec
UPDATE characters SET state=jsonb_set(state,'{inventory,gold}',sqlc.arg(gold)::jsonb)
WHERE id=sqlc.arg(character_id);

-- name: FixtureMergeCharacterFields :exec
UPDATE characters SET state=state||sqlc.arg(fields)::jsonb WHERE id=sqlc.arg(character_id);

-- name: FixtureReadCharacterSnapshot :one
SELECT state,config_version FROM characters WHERE id=sqlc.arg(character_id);

-- name: FixtureCashOrderCount :one
SELECT count(*) FROM cash_orders WHERE account_id=sqlc.arg(account_id);

-- name: FixtureCashInventoryCount :one
SELECT count(*) FROM cash_inventory WHERE account_id=sqlc.arg(account_id)
AND (sqlc.arg(claim_state)::integer=0
 OR (sqlc.arg(claim_state)::integer=1 AND claimed_at IS NULL)
 OR (sqlc.arg(claim_state)::integer=2 AND claimed_at IS NOT NULL))
AND (sqlc.arg(template)::bigint=0 OR template=sqlc.arg(template)::bigint)
AND (sqlc.arg(amount)::integer=0 OR amount=sqlc.arg(amount)::integer);

-- name: FixtureCashOrderSource :one
SELECT coalesce(request->>'source','')::text FROM cash_orders
WHERE account_id=sqlc.arg(account_id) AND order_key=sqlc.arg(order_key);

-- name: FixtureQuestRecordSeed :exec
INSERT INTO character_quests(character_id,quest_id,status,config_version,progress_model)
VALUES(sqlc.arg(character_id),sqlc.arg(quest_id)::integer,sqlc.arg(status),sqlc.arg(config_version),sqlc.arg(progress_model));

-- name: FixtureQuestCount :one
SELECT count(*) FROM character_quests
WHERE (sqlc.arg(character_id)::bigint=0 OR character_id=sqlc.arg(character_id)::bigint);

-- name: FixtureQuestRecord :one
SELECT status,progress_model FROM character_quests
WHERE character_id=sqlc.arg(character_id) AND quest_id=sqlc.arg(quest_id)::integer;

-- name: FixtureFatigueUsage :exec
UPDATE character_fatigue SET used=sqlc.arg(used)::integer,used_max=sqlc.arg(used_max)::integer
WHERE character_id=sqlc.arg(character_id);
