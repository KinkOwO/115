-- SQLite query fork of sql/postgres/queries/quests.sql (dual-engine, S3).
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
--   * JSON parameters (before_state, after_state, receipt) are deliberately NOT
--     wrapped in CAST(... AS TEXT): that turns the parameter into a string, which
--     database/sql cannot scan into json.RawMessage. They are BLOB here, and a
--     BLOB parameter/result is generated as []byte, which json.RawMessage is
--     assignable to.
--   * unnest(sqlc.arg(ids)::integer[]) -> json_each(CAST(sqlc.arg(ids) AS TEXT));
--     the id list travels as a JSON array text and each element is cast back with
--     CAST(j.value AS INTEGER).
--   * quest_id=ANY(sqlc.arg(ids)::integer[]) -> quest_id IN (SELECT value FROM
--     json_each(CAST(sqlc.arg(ids) AS TEXT))).
--   * now() -> integer microseconds, matching the DDL defaults and the driver's
--     unix_micro representation.
--   * UPDATE ... FROM and DELETE ... USING are rejected by the SQLite grammar
--     (and by sqlc's SQLite parser). The five statements that used them keep the
--     identical ownership predicate, spelled as a correlated EXISTS over the same
--     characters/character_events join. Every outer column is table-qualified
--     because characters also has character_id/config_version.
--   * outcome->>'template' -> json_extract(outcome,'$.template'), compared against
--     a text parameter (CAST(sqlc.arg(template) AS TEXT)).

-- name: LockOwnedCharacterState :one
-- Lock query; the FOR UPDATE is replaced by the engine-wide BEGIN IMMEDIATE write
-- serialization. Name and ownership predicate are unchanged.
SELECT state FROM characters WHERE id=sqlc.arg(character_id) AND account_id=sqlc.arg(account_id);

-- name: LockOwnedCharacterIncludingDeleted :one
-- Lock query; lock clause removed. The PostgreSQL 0::smallint projection is kept
-- as an explicit CAST so sqlc does not infer interface{}.
SELECT id,account_id,wire_id,name,profession,create_request,config_version,state,created_at,
CAST(0 AS INTEGER) AS fixed_slot FROM characters
WHERE account_id=sqlc.arg(account_id) AND id=sqlc.arg(character_id);

-- name: Quest :one
SELECT status,progress,config_version,progress_model FROM character_quests
WHERE character_id=sqlc.arg(character_id) AND quest_id=sqlc.arg(quest_id);

-- name: CompleteGraduationQuests :exec
-- unnest(...) becomes json_each(...); the ids arrive as a JSON array text.
-- WHERE 1=1 is required, not decorative: when an INSERT ... SELECT ends on a bare
-- table reference the SQLite parser reads the following ON as a join constraint and
-- rejects ON CONFLICT ("near DO: syntax error"). A WHERE clause removes the
-- ambiguity and is a no-op for the row set.
INSERT INTO character_quests(character_id,quest_id,status,config_version,progress,progress_model,completed_at)
SELECT CAST(sqlc.arg(character_id) AS INTEGER),CAST(j.value AS INTEGER),'completed',CAST(sqlc.arg(config_version) AS TEXT),0,CAST(sqlc.arg(progress_model) AS TEXT),
(CAST((julianday('now') - 2440587.5) * 86400000000 AS INTEGER))
FROM json_each(CAST(sqlc.arg(quest_ids) AS TEXT)) j
WHERE 1=1
ON CONFLICT(character_id,quest_id) DO UPDATE SET status='completed',progress=0,
progress_model=EXCLUDED.progress_model,config_version=EXCLUDED.config_version,
completed_at=(CAST((julianday('now') - 2440587.5) * 86400000000 AS INTEGER))
WHERE character_quests.status='accepted';

-- name: LockQuest :one
-- Lock query; lock clause removed (see LockOwnedCharacterState).
SELECT status,progress,config_version,progress_model FROM character_quests
WHERE character_id=sqlc.arg(character_id) AND quest_id=sqlc.arg(quest_id);

-- name: HasCompletedQuest :one
SELECT EXISTS(SELECT 1 FROM character_quests WHERE character_id=sqlc.arg(character_id)
AND quest_id=sqlc.arg(quest_id) AND status='completed');

-- name: AcceptQuest :exec
INSERT INTO character_quests(character_id,quest_id,status,config_version,progress,progress_model)
VALUES(sqlc.arg(character_id),sqlc.arg(quest_id),'accepted',sqlc.arg(config_version),sqlc.arg(progress),sqlc.arg(progress_model));

-- name: AbandonQuest :execrows
-- PostgreSQL: DELETE FROM character_quests q USING characters c WHERE ... The
-- USING join is not SQLite grammar, so it becomes the equivalent correlated
-- EXISTS with the same ownership predicate.
DELETE FROM character_quests WHERE character_quests.character_id=sqlc.arg(character_id)
AND character_quests.quest_id=sqlc.arg(quest_id) AND character_quests.status='accepted'
AND EXISTS(SELECT 1 FROM characters c WHERE c.id=character_quests.character_id AND c.account_id=sqlc.arg(account_id));

-- name: MarkMeetNPCQuest :execrows
-- PostgreSQL: UPDATE ... FROM characters c. Rewritten as a correlated EXISTS.
UPDATE character_quests SET progress=0
WHERE character_quests.quest_id=sqlc.arg(quest_id) AND character_quests.status='accepted'
AND character_quests.config_version=sqlc.arg(config_version) AND character_quests.progress_model=sqlc.arg(progress_model)
AND character_quests.progress IN(0,1)
AND EXISTS(SELECT 1 FROM characters c WHERE c.id=character_quests.character_id
AND c.account_id=sqlc.arg(account_id) AND c.id=sqlc.arg(character_id) AND c.deleted_at IS NULL);

-- name: ClearQuests :execrows
INSERT INTO character_quests(character_id,quest_id,status,config_version,progress,progress_model)
SELECT c.id,CAST(j.value AS INTEGER),'completed',CAST(sqlc.arg(config_version) AS TEXT),0,'odyssey-skip-v1'
FROM json_each(CAST(sqlc.arg(quest_ids) AS TEXT)) j
JOIN characters c ON c.id=CAST(sqlc.arg(character_id) AS INTEGER) AND c.account_id=CAST(sqlc.arg(account_id) AS INTEGER) AND c.deleted_at IS NULL
ON CONFLICT(character_id,quest_id) DO NOTHING;

-- name: ClearActQuests :execrows
UPDATE character_quests SET status='completed',progress=0,progress_model='act-clear-v2',completed_at=(CAST((julianday('now') - 2440587.5) * 86400000000 AS INTEGER))
WHERE character_id=sqlc.arg(character_id) AND quest_id IN(SELECT value FROM json_each(CAST(sqlc.arg(quest_ids) AS TEXT)))
AND status='accepted' AND config_version=sqlc.arg(config_version);

-- name: Quests :many
SELECT quest_id,status,progress,config_version,progress_model FROM character_quests
WHERE character_id=sqlc.arg(character_id) ORDER BY quest_id;

-- name: CompletedQuestIDs :many
SELECT q.character_id,q.quest_id FROM character_quests q JOIN characters c ON c.id=q.character_id
WHERE c.account_id=sqlc.arg(account_id) AND c.deleted_at IS NULL
AND q.status='completed' AND q.config_version=sqlc.arg(config_version)
AND q.quest_id IN(SELECT value FROM json_each(CAST(sqlc.arg(quest_ids) AS TEXT)));

-- name: RepairLegacyQuest :execrows
-- PostgreSQL: UPDATE ... FROM characters c. Rewritten as a correlated EXISTS.
UPDATE character_quests SET progress=sqlc.arg(progress),progress_model=sqlc.arg(progress_model)
WHERE character_quests.quest_id=sqlc.arg(quest_id) AND character_quests.status='accepted'
AND character_quests.progress=0 AND character_quests.progress_model='legacy-zero'
AND character_quests.config_version=sqlc.arg(config_version)
AND EXISTS(SELECT 1 FROM characters c WHERE c.id=character_quests.character_id
AND c.account_id=sqlc.arg(account_id) AND c.id=sqlc.arg(character_id));

-- name: RecordLegacyQuestRepair :exec
INSERT INTO character_quest_repairs(character_id,quest_id,reason,before_state,after_state)
VALUES(sqlc.arg(character_id),sqlc.arg(quest_id),'initial-zero-false-completion',sqlc.arg(before_state),sqlc.arg(after_state));

-- name: CompleteQuestObjective :execrows
-- PostgreSQL: UPDATE ... FROM characters c. Rewritten as a correlated EXISTS.
UPDATE character_quests SET progress=0
WHERE character_quests.quest_id=sqlc.arg(quest_id) AND character_quests.status='accepted'
AND character_quests.config_version=sqlc.arg(config_version) AND character_quests.progress_model=sqlc.arg(progress_model)
AND character_quests.progress<>0
AND EXISTS(SELECT 1 FROM characters c WHERE c.id=character_quests.character_id
AND c.account_id=sqlc.arg(account_id) AND c.id=sqlc.arg(character_id) AND c.deleted_at IS NULL);

-- name: CompleteQuestUseObjective :execrows
-- PostgreSQL: UPDATE ... FROM characters c,character_events e. Rewritten as a
-- correlated EXISTS over the same join, including the accepted_at bound.
UPDATE character_quests SET progress=0
WHERE character_quests.quest_id=sqlc.arg(quest_id) AND character_quests.status='accepted'
AND character_quests.config_version=sqlc.arg(config_version) AND character_quests.progress_model=sqlc.arg(progress_model)
AND character_quests.progress=1
AND EXISTS(SELECT 1 FROM characters c JOIN character_events e ON e.character_id=c.id
WHERE c.id=character_quests.character_id AND c.account_id=sqlc.arg(account_id) AND c.id=sqlc.arg(character_id)
AND c.deleted_at IS NULL AND e.event_key=sqlc.arg(event_key) AND e.config_version=sqlc.arg(config_version)
AND json_extract(e.outcome,'$.template')=CAST(sqlc.arg(template) AS TEXT) AND e.created_at>=character_quests.accepted_at);

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
UPDATE character_quests SET status='completed',completed_at=(CAST((julianday('now') - 2440587.5) * 86400000000 AS INTEGER))
WHERE character_id=sqlc.arg(character_id) AND quest_id=sqlc.arg(quest_id);
