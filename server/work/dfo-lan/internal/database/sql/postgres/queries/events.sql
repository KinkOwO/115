-- name: StoredCharacterEvent :one
SELECT model,outcome FROM character_events
WHERE character_id=sqlc.arg(character_id) AND event_key=sqlc.arg(event_key);

-- name: CharacterEventStage :one
SELECT model,(outcome->>'stage')::text AS stage FROM character_events
WHERE character_id=sqlc.arg(character_id) AND event_key=sqlc.arg(event_key);

-- name: BlackPurgatoryEntryStage :one
SELECT (outcome->>'stage')::text AS stage FROM character_events
WHERE character_id=sqlc.arg(character_id) AND event_key=sqlc.arg(event_key) AND model='black-purgatory-entry-v1';

-- name: SetCharacterEventStage :exec
UPDATE character_events SET outcome=jsonb_set(outcome,'{stage}',to_jsonb(sqlc.arg(stage)::text))
WHERE character_id=sqlc.arg(character_id) AND event_key=sqlc.arg(event_key);

-- name: RefundPendingBlackPurgatoryEntries :exec
UPDATE character_events SET outcome=jsonb_set(outcome,'{stage}','"refunded"')
WHERE character_id=sqlc.arg(character_id) AND model='black-purgatory-entry-v1' AND outcome->>'stage'='entered';

-- name: BlackPurgatoryEntryCounts :one
SELECT count(*) FILTER(WHERE created_at >= sqlc.arg(day_start)::timestamptz) AS daily,
count(*) AS weekly FROM character_events
WHERE character_id=sqlc.arg(character_id) AND model='black-purgatory-entry-v1'
AND created_at >= sqlc.arg(week_start)::timestamptz AND outcome->>'stage' IN ('entered','cleared');

-- name: PendingBlackPurgatoryRewards :many
SELECT e.outcome FROM character_events e JOIN characters c ON c.id=e.character_id
WHERE c.account_id=sqlc.arg(account_id) AND c.id=sqlc.arg(character_id) AND e.model=sqlc.arg(model)
AND e.event_key LIKE 'cardplan:%'
AND NOT EXISTS(SELECT 1 FROM character_events g WHERE g.character_id=e.character_id
AND g.event_key='black-purgatory-recovered:'||substr(e.event_key,10)) ORDER BY e.created_at LIMIT 16;

-- name: NormalizeCharacterIdentity :execrows
UPDATE characters SET config_version=sqlc.arg(identity)::text WHERE config_version IS DISTINCT FROM sqlc.arg(identity)::text;

-- name: NormalizeWorldIdentity :execrows
UPDATE character_world SET config_version=sqlc.arg(identity)::text WHERE config_version IS DISTINCT FROM sqlc.arg(identity)::text;

-- name: NormalizeQuestIdentity :execrows
UPDATE character_quests SET config_version=sqlc.arg(identity)::text WHERE config_version IS DISTINCT FROM sqlc.arg(identity)::text;

-- name: NormalizeEventIdentity :execrows
UPDATE character_events SET config_version=sqlc.arg(identity)::text WHERE config_version IS DISTINCT FROM sqlc.arg(identity)::text;

-- name: NormalizeMapClearIdentity :execrows
UPDATE character_map_clears SET source_version=sqlc.arg(identity)::text WHERE source_version IS DISTINCT FROM sqlc.arg(identity)::text;

-- name: NormalizeQuestRewardIdentity :execrows
UPDATE character_quest_rewards SET source_version=sqlc.arg(identity)::text WHERE source_version IS DISTINCT FROM sqlc.arg(identity)::text;
