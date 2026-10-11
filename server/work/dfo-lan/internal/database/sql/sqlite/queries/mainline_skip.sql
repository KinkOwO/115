-- Local mainline skip keeps the same event and reward contracts on both engines.
-- name: UpsertMainlineSkippedQuest :exec
INSERT INTO character_quests(character_id,quest_id,status,config_version,progress,progress_model,completed_at)
VALUES(sqlc.arg(character_id),sqlc.arg(quest_id),'completed',sqlc.arg(config_version),0,sqlc.arg(progress_model),(CAST((julianday('now') - 2440587.5) * 86400000000 AS INTEGER)))
ON CONFLICT(character_id,quest_id) DO UPDATE SET status='completed',config_version=EXCLUDED.config_version,progress=0,progress_model=EXCLUDED.progress_model,completed_at=EXCLUDED.completed_at
WHERE character_quests.status <> 'completed';

-- name: HasMainlineSweep :one
SELECT EXISTS(SELECT 1 FROM character_events WHERE character_id=sqlc.arg(character_id) AND event_key LIKE 'gm-quest-sweep:%');

-- name: SaveMainlineTicketOutcome :exec
INSERT INTO character_events(character_id,event_key,config_version,model,outcome)
VALUES(sqlc.arg(character_id),'quest-skip-ticket-items',sqlc.arg(config_version),'quest-skip-ticket-items-v1',sqlc.arg(outcome))
ON CONFLICT(character_id,event_key) DO UPDATE SET outcome=EXCLUDED.outcome;
