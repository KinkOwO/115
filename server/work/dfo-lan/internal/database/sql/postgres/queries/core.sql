-- name: NameExists :one
SELECT EXISTS(SELECT 1 FROM characters WHERE lower(name)=lower(sqlc.arg(name)::text));

-- name: DevelopmentAccount :one
INSERT INTO accounts(username,development_only) VALUES(sqlc.arg(username),true)
ON CONFLICT(username) DO UPDATE SET username=EXCLUDED.username
WHERE accounts.development_only RETURNING id;

-- name: CharacterEventReceipt :one
SELECT e.outcome FROM character_events e JOIN characters c ON c.id=e.character_id
WHERE c.account_id=sqlc.arg(account_id) AND c.id=sqlc.arg(character_id) AND e.event_key=sqlc.arg(event_key);

-- name: CharacterOwned :one
SELECT EXISTS(SELECT 1 FROM characters WHERE account_id=sqlc.arg(account_id) AND id=sqlc.arg(character_id));

-- name: LockAccount :one
SELECT id FROM accounts WHERE id=sqlc.arg(account_id) FOR UPDATE;

-- name: LockAccountState :one
-- Serialize shared callback state without blocking foreign-key key-share locks.
SELECT id FROM accounts WHERE id=sqlc.arg(account_id) FOR NO KEY UPDATE;

-- name: CharacterAllocation :one
SELECT count(*) FILTER (WHERE deleted_at IS NULL) AS active_count,
coalesce(max(wire_id),0)+1 AS next_wire_id,
(coalesce(max(coalesce(roster_order,wire_id)),0)+1)::bigint AS next_roster_order
FROM characters WHERE account_id=sqlc.arg(account_id);

-- name: CreateCharacter :one
INSERT INTO characters(account_id,wire_id,name,profession,create_request,config_version,state,roster_order,fixed_slot)
VALUES(sqlc.arg(account_id),sqlc.arg(wire_id),sqlc.arg(name),sqlc.arg(profession),sqlc.arg(create_request),
sqlc.arg(config_version),sqlc.arg(state),sqlc.arg(roster_order)::bigint,0) RETURNING id,created_at;

-- name: Characters :many
SELECT id,account_id,wire_id,name,profession,create_request,config_version,state,created_at,fixed_slot
FROM characters WHERE account_id=sqlc.arg(account_id) AND deleted_at IS NULL
ORDER BY coalesce(roster_order,wire_id),wire_id;

-- name: CharactersWithAdventure :many
SELECT c.id,c.account_id,c.wire_id,c.name,c.profession,c.create_request,c.config_version,
jsonb_set(c.state,'{season_level}',COALESCE(a.data->'season_level','{}'::jsonb),true)::jsonb AS state,
c.created_at,c.fixed_slot FROM characters c LEFT JOIN account_adventures a ON a.account_id=c.account_id
WHERE c.account_id=sqlc.arg(account_id) AND c.deleted_at IS NULL
ORDER BY coalesce(c.roster_order,c.wire_id),c.wire_id;

-- name: LockCharacterRoster :many
SELECT id,fixed_slot FROM characters WHERE account_id=sqlc.arg(account_id) AND deleted_at IS NULL
ORDER BY coalesce(roster_order,wire_id),wire_id FOR UPDATE;

-- name: SaveCharacterSlot :exec
UPDATE characters SET roster_order=sqlc.arg(roster_order)::bigint,fixed_slot=sqlc.arg(fixed_slot)
WHERE account_id=sqlc.arg(account_id) AND id=sqlc.arg(character_id);

-- name: LockCharacterVersion :one
SELECT config_version FROM characters WHERE account_id=sqlc.arg(account_id) AND id=sqlc.arg(character_id)
AND deleted_at IS NULL FOR UPDATE;

-- name: LockCharacterOwner :one
SELECT id FROM characters WHERE account_id=sqlc.arg(account_id) AND id=sqlc.arg(character_id)
AND deleted_at IS NULL FOR UPDATE;

-- name: LockCharacterOwnerIncludingDeleted :one
SELECT id FROM characters WHERE account_id=sqlc.arg(account_id) AND id=sqlc.arg(character_id) FOR UPDATE;

-- name: CharacterEventModel :one
SELECT model FROM character_events WHERE character_id=sqlc.arg(character_id) AND event_key=sqlc.arg(event_key);

-- name: RecordCharacterEvent :exec
INSERT INTO character_events(character_id,event_key,config_version,model,outcome)
VALUES(sqlc.arg(character_id),sqlc.arg(event_key),sqlc.arg(config_version),sqlc.arg(model),sqlc.arg(outcome));

-- name: HasCharacterEvent :one
SELECT EXISTS(SELECT 1 FROM character_events WHERE character_id=sqlc.arg(character_id) AND event_key=sqlc.arg(event_key));

-- name: LockCharacter :one
-- Event callbacks historically leave FixedSlot at zero; preserve that projection.
SELECT id,account_id,wire_id,name,profession,create_request,config_version,state,created_at,
0::smallint AS fixed_slot FROM characters
WHERE account_id=sqlc.arg(account_id) AND id=sqlc.arg(character_id) AND deleted_at IS NULL FOR UPDATE;

-- name: UpdateCharacterState :exec
UPDATE characters SET state=sqlc.arg(state) WHERE id=sqlc.arg(character_id);

-- name: CharacterAtRosterSlot :one
SELECT id,name FROM characters WHERE account_id=sqlc.arg(account_id) AND deleted_at IS NULL
ORDER BY coalesce(roster_order,wire_id),wire_id OFFSET sqlc.arg(roster_slot)::bigint LIMIT 1 FOR UPDATE;

-- name: ArchiveCharacter :exec
UPDATE characters SET deleted_at=now() WHERE id=sqlc.arg(character_id) AND account_id=sqlc.arg(account_id) AND deleted_at IS NULL;

-- name: RecordCharacterFame :one
UPDATE characters SET max_fame=GREATEST(max_fame,sqlc.arg(current_fame)::integer)
WHERE account_id=sqlc.arg(account_id) AND id=sqlc.arg(character_id) AND deleted_at IS NULL RETURNING max_fame;

-- name: DatabaseName :one
SELECT current_database()::text;

-- name: Accounts :many
SELECT id,username FROM accounts ORDER BY id;

-- name: AdminCharacters :many
SELECT id,account_id,wire_id,name,profession,create_request,config_version,state,created_at,fixed_slot
FROM characters WHERE account_id=sqlc.arg(account_id) AND deleted_at IS NULL ORDER BY wire_id;

-- name: AdminCharacter :one
SELECT id,account_id,wire_id,name,profession,create_request,config_version,state,created_at,fixed_slot
FROM characters WHERE id=sqlc.arg(character_id) AND deleted_at IS NULL;

-- name: DevelopmentCharacterAccount :one
SELECT c.account_id FROM characters c JOIN accounts a ON a.id=c.account_id
WHERE c.id=sqlc.arg(character_id) AND a.development_only;

-- name: CharacterSeason :one
SELECT jsonb_build_object('season_level',COALESCE(a.data->'season_level','{}'::jsonb))::jsonb AS state
FROM characters c LEFT JOIN account_adventures a ON a.account_id=c.account_id
WHERE c.account_id=sqlc.arg(account_id) AND c.id=sqlc.arg(character_id) AND c.deleted_at IS NULL;

-- name: TrySharedAdminGuard :one
SELECT pg_try_advisory_lock_shared(11520260922)::boolean;

-- name: ShareActiveCharacterState :one
SELECT state FROM characters WHERE account_id=sqlc.arg(account_id) AND id=sqlc.arg(character_id) AND deleted_at IS NULL FOR SHARE;

-- name: LockActiveCharacterState :one
SELECT state FROM characters WHERE account_id=sqlc.arg(account_id) AND id=sqlc.arg(character_id) AND deleted_at IS NULL FOR UPDATE;

-- name: LockActiveCharacterID :one
SELECT id FROM characters WHERE id=sqlc.arg(character_id) AND deleted_at IS NULL FOR UPDATE;

-- name: ShareActiveCharacterIDs :many
SELECT id FROM characters WHERE account_id=sqlc.arg(account_id) AND deleted_at IS NULL ORDER BY id FOR SHARE;

-- name: AccountCharacterStates :many
SELECT state FROM characters WHERE account_id=sqlc.arg(account_id);

-- name: PendingMoonRewardRuns :many
SELECT substr(event_key,12)::text AS run_id FROM character_events e
WHERE e.character_id=sqlc.arg(character_id) AND e.model=sqlc.arg(model) AND e.event_key LIKE 'moon-clear:%'
AND NOT EXISTS(SELECT 1 FROM character_events g WHERE g.character_id=e.character_id
AND g.event_key='moon-grant:'||substr(e.event_key,12)) ORDER BY created_at LIMIT 16;
