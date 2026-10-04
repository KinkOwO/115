-- name: LockOwnedCharacterState :one
SELECT state FROM characters WHERE id=sqlc.arg(character_id) AND account_id=sqlc.arg(account_id) FOR UPDATE;

-- name: LockOwnedCharacterIncludingDeleted :one
SELECT id,account_id,wire_id,name,profession,create_request,config_version,state,created_at,
0::smallint AS fixed_slot FROM characters
WHERE account_id=sqlc.arg(account_id) AND id=sqlc.arg(character_id) FOR UPDATE;

-- name: Quest :one
SELECT status,progress,config_version,progress_model FROM character_quests
WHERE character_id=sqlc.arg(character_id) AND quest_id=sqlc.arg(quest_id);

-- name: CompleteGraduationQuests :exec
INSERT INTO character_quests(character_id,quest_id,status,config_version,progress,progress_model,completed_at)
SELECT sqlc.arg(character_id)::bigint,q.id,'completed',sqlc.arg(config_version)::text,0,sqlc.arg(progress_model)::text,now()
FROM unnest(sqlc.arg(quest_ids)::integer[]) AS q(id)
ON CONFLICT(character_id,quest_id) DO UPDATE SET status='completed',progress=0,
progress_model=EXCLUDED.progress_model,config_version=EXCLUDED.config_version,completed_at=now()
WHERE character_quests.status='accepted';

-- name: LockQuest :one
SELECT status,progress,config_version,progress_model FROM character_quests
WHERE character_id=sqlc.arg(character_id) AND quest_id=sqlc.arg(quest_id) FOR UPDATE;

-- name: HasCompletedQuest :one
SELECT EXISTS(SELECT 1 FROM character_quests WHERE character_id=sqlc.arg(character_id)
AND quest_id=sqlc.arg(quest_id)::bigint AND status='completed');

-- name: AcceptQuest :exec
INSERT INTO character_quests(character_id,quest_id,status,config_version,progress,progress_model)
VALUES(sqlc.arg(character_id),sqlc.arg(quest_id),'accepted',sqlc.arg(config_version),sqlc.arg(progress),sqlc.arg(progress_model));

-- name: AbandonQuest :execrows
DELETE FROM character_quests q USING characters c WHERE q.character_id=c.id
AND c.id=sqlc.arg(character_id) AND c.account_id=sqlc.arg(account_id)
AND q.quest_id=sqlc.arg(quest_id) AND q.status='accepted';

-- name: MarkMeetNPCQuest :execrows
UPDATE character_quests q SET progress=0 FROM characters c WHERE c.id=q.character_id
AND c.account_id=sqlc.arg(account_id) AND c.id=sqlc.arg(character_id) AND c.deleted_at IS NULL
AND q.quest_id=sqlc.arg(quest_id) AND q.status='accepted' AND q.config_version=sqlc.arg(config_version)
AND q.progress_model=sqlc.arg(progress_model) AND q.progress IN(0,1);

-- name: ClearQuests :execrows
INSERT INTO character_quests(character_id,quest_id,status,config_version,progress,progress_model)
SELECT c.id,q.id,'completed',sqlc.arg(config_version)::text,0,'odyssey-skip-v1'
FROM unnest(sqlc.arg(quest_ids)::integer[]) AS q(id)
JOIN characters c ON c.id=sqlc.arg(character_id)::bigint AND c.account_id=sqlc.arg(account_id)::bigint AND c.deleted_at IS NULL
ON CONFLICT(character_id,quest_id) DO NOTHING;

-- name: ClearActQuests :execrows
UPDATE character_quests SET status='completed',progress=0,progress_model='act-clear-v2',completed_at=now()
WHERE character_id=sqlc.arg(character_id) AND quest_id=ANY(sqlc.arg(quest_ids)::integer[])
AND status='accepted' AND config_version=sqlc.arg(config_version);

-- name: Quests :many
SELECT quest_id,status,progress,config_version,progress_model FROM character_quests
WHERE character_id=sqlc.arg(character_id) ORDER BY quest_id;

-- name: CompletedQuestIDs :many
SELECT q.character_id,q.quest_id FROM character_quests q JOIN characters c ON c.id=q.character_id
WHERE c.account_id=sqlc.arg(account_id) AND c.deleted_at IS NULL
AND q.status='completed' AND q.config_version=sqlc.arg(config_version) AND q.quest_id=ANY(sqlc.arg(quest_ids)::integer[]);

-- name: RepairLegacyQuest :execrows
UPDATE character_quests q SET progress=sqlc.arg(progress),progress_model=sqlc.arg(progress_model) FROM characters c
WHERE q.character_id=c.id AND c.account_id=sqlc.arg(account_id) AND c.id=sqlc.arg(character_id)
AND q.quest_id=sqlc.arg(quest_id) AND q.status='accepted' AND q.progress=0
AND q.progress_model='legacy-zero' AND q.config_version=sqlc.arg(config_version);

-- name: RecordLegacyQuestRepair :exec
INSERT INTO character_quest_repairs(character_id,quest_id,reason,before_state,after_state)
VALUES(sqlc.arg(character_id),sqlc.arg(quest_id),'initial-zero-false-completion',sqlc.arg(before_state),sqlc.arg(after_state));

-- name: CompleteQuestObjective :execrows
UPDATE character_quests q SET progress=0 FROM characters c
WHERE c.id=q.character_id AND c.account_id=sqlc.arg(account_id) AND c.id=sqlc.arg(character_id) AND c.deleted_at IS NULL
AND q.quest_id=sqlc.arg(quest_id) AND q.status='accepted' AND q.config_version=sqlc.arg(config_version)
AND q.progress_model=sqlc.arg(progress_model) AND q.progress<>0;

-- name: CompleteQuestUseObjective :execrows
UPDATE character_quests q SET progress=0 FROM characters c,character_events e
WHERE c.id=q.character_id AND c.account_id=sqlc.arg(account_id) AND c.id=sqlc.arg(character_id) AND c.deleted_at IS NULL
AND e.character_id=c.id AND e.event_key=sqlc.arg(event_key) AND e.config_version=sqlc.arg(config_version)
AND e.outcome->>'template'=sqlc.arg(template)::text AND e.created_at>=q.accepted_at
AND q.quest_id=sqlc.arg(quest_id) AND q.status='accepted' AND q.config_version=sqlc.arg(config_version)
AND q.progress_model=sqlc.arg(progress_model) AND q.progress=1;

-- name: RecordQuestMapClear :execrows
INSERT INTO character_map_clears(character_id,run_id,map_id,source_version)
VALUES(sqlc.arg(character_id),sqlc.arg(run_id),sqlc.arg(map_id),sqlc.arg(source_version)) ON CONFLICT DO NOTHING;

-- name: CompleteQuestMapObjective :exec
UPDATE character_quests SET progress=0 WHERE character_id=sqlc.arg(character_id)
AND quest_id=sqlc.arg(quest_id) AND status='accepted' AND progress=1
AND config_version=sqlc.arg(config_version) AND progress_model=sqlc.arg(progress_model);

-- name: QuestRewardReceipt :one
SELECT receipt FROM character_quest_rewards WHERE character_id=sqlc.arg(character_id)
AND quest_id=sqlc.arg(quest_id) AND source_version=sqlc.arg(source_version);

-- name: RecordQuestReward :exec
INSERT INTO character_quest_rewards(character_id,quest_id,source_version,model,receipt)
VALUES(sqlc.arg(character_id),sqlc.arg(quest_id),sqlc.arg(source_version),sqlc.arg(model),sqlc.arg(receipt));

-- name: CompleteRewardedQuest :exec
UPDATE character_quests SET status='completed',completed_at=now()
WHERE character_id=sqlc.arg(character_id) AND quest_id=sqlc.arg(quest_id);
