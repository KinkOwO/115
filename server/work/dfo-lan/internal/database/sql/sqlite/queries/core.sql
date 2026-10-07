-- SQLite query fork of sql/postgres/queries/core.sql (dual-engine, S3).
--
-- Ported for the core slice only: accounts, characters, character_events. The
-- PostgreSQL file stays untouched and remains authoritative for the PG path.
--
-- Dialect rules applied (design doc sec.3.2, plus the S0 spike's two hard rules):
--   * FOR UPDATE / FOR NO KEY UPDATE / FOR SHARE -> REMOVED. SQLite has no row
--     locks and its grammar rejects the clause outright; the engine serializes
--     writers with BEGIN IMMEDIATE (DSN _txlock=immediate). Lock queries keep
--     their name and their ownership predicate, so callers are unchanged.
--   * `sqlc.arg(x)::type` -> CAST(sqlc.arg(x) AS type). SPIKE RULE R-1: an
--     expression sqlc cannot type infers interface{} and cannot enter the shared
--     query interface, so every bare expression gets an explicit CAST.
--   * JSON parameters are deliberately NOT cast. CAST(sqlc.arg(state) AS TEXT)
--     would make the generated parameter a string, and SQLite would then return a
--     string that database/sql cannot scan into json.RawMessage. Keeping the
--     value a []byte keeps json.RawMessage in BOTH directions (D24).
--   * jsonb operators -> json_extract / json_set (+ json() to keep a JSON value
--     a value rather than a string). Anything whose JSON semantics are subtle is
--     flagged for a runtime golden test in S4 rather than assumed here.
--   * now() -> integer microseconds, matching the DDL defaults and the driver's
--     _inttotime=1 + _time_integer_format=unix_micro representation.
--   * GREATEST(a,b) -> max(a,b) (SQLite scalar form).
--
-- Still not ported, with reasons (design doc sec.3.5):
--   * TrySharedAdminGuard - PostgreSQL advisory lock; SQLite needs the lease-row
--     design (sec.3.3), so it cannot be translated into one statement.
-- DatabaseName is ported below through pragma_database_list, and
-- CharactersWithAdventure / CharacterSeason through json_set / json_object.

-- name: NameExists :one
SELECT EXISTS(SELECT 1 FROM characters WHERE lower(name)=lower(CAST(sqlc.arg(name) AS TEXT)));

-- name: DevelopmentAccount :one
INSERT INTO accounts(username,development_only) VALUES(sqlc.arg(username),1)
ON CONFLICT(username) DO UPDATE SET username=EXCLUDED.username
WHERE accounts.development_only RETURNING id;

-- name: CharacterEventReceipt :one
SELECT e.outcome FROM character_events e JOIN characters c ON c.id=e.character_id
WHERE c.account_id=sqlc.arg(account_id) AND c.id=sqlc.arg(character_id) AND e.event_key=sqlc.arg(event_key);

-- name: CharacterOwned :one
SELECT EXISTS(SELECT 1 FROM characters WHERE account_id=sqlc.arg(account_id) AND id=sqlc.arg(character_id));

-- name: LockAccount :one
-- Lock query; the FOR UPDATE is replaced by the engine-wide BEGIN IMMEDIATE write
-- serialization (see the file header). Name and predicate are unchanged.
SELECT id FROM accounts WHERE id=sqlc.arg(account_id);

-- name: LockAccountState :one
-- Serialize shared callback state; the account row is read inside the same
-- serialized write transaction, so no row lock is required.
SELECT id FROM accounts WHERE id=sqlc.arg(account_id);

-- name: CharacterAllocation :one
SELECT count(*) FILTER (WHERE deleted_at IS NULL) AS active_count,
coalesce(max(wire_id),0)+1 AS next_wire_id,
CAST((coalesce(max(coalesce(roster_order,wire_id)),0)+1) AS INTEGER) AS next_roster_order
FROM characters WHERE account_id=sqlc.arg(account_id);

-- name: CreateCharacter :one
INSERT INTO characters(account_id,wire_id,name,profession,create_request,config_version,state,roster_order,fixed_slot)
VALUES(sqlc.arg(account_id),sqlc.arg(wire_id),sqlc.arg(name),sqlc.arg(profession),sqlc.arg(create_request),
sqlc.arg(config_version),sqlc.arg(state),CAST(sqlc.arg(roster_order) AS INTEGER),0) RETURNING id,created_at;

-- name: Characters :many
SELECT id,account_id,wire_id,name,profession,create_request,config_version,state,created_at,fixed_slot
FROM characters WHERE account_id=sqlc.arg(account_id) AND deleted_at IS NULL
ORDER BY coalesce(roster_order,wire_id),wire_id;

-- name: LockCharacterRoster :many
SELECT id,fixed_slot FROM characters WHERE account_id=sqlc.arg(account_id) AND deleted_at IS NULL
ORDER BY coalesce(roster_order,wire_id),wire_id;

-- name: SaveCharacterSlot :exec
UPDATE characters SET roster_order=CAST(sqlc.arg(roster_order) AS INTEGER),fixed_slot=sqlc.arg(fixed_slot)
WHERE account_id=sqlc.arg(account_id) AND id=sqlc.arg(character_id);

-- name: LockCharacterVersion :one
SELECT config_version FROM characters WHERE account_id=sqlc.arg(account_id) AND id=sqlc.arg(character_id)
AND deleted_at IS NULL;

-- name: LockCharacterOwner :one
SELECT id FROM characters WHERE account_id=sqlc.arg(account_id) AND id=sqlc.arg(character_id)
AND deleted_at IS NULL;

-- name: LockCharacterOwnerIncludingDeleted :one
SELECT id FROM characters WHERE account_id=sqlc.arg(account_id) AND id=sqlc.arg(character_id);

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
0 AS fixed_slot FROM characters
WHERE account_id=sqlc.arg(account_id) AND id=sqlc.arg(character_id) AND deleted_at IS NULL;

-- name: UpdateCharacterState :exec
UPDATE characters SET state=sqlc.arg(state) WHERE id=sqlc.arg(character_id);

-- name: CharacterAtRosterSlot :one
SELECT id,name FROM characters WHERE account_id=sqlc.arg(account_id) AND deleted_at IS NULL
ORDER BY coalesce(roster_order,wire_id),wire_id LIMIT 1 OFFSET CAST(sqlc.arg(roster_slot) AS INTEGER);

-- name: ArchiveCharacter :exec
UPDATE characters SET deleted_at=(CAST((julianday('now') - 2440587.5) * 86400000000 AS INTEGER))
WHERE id=sqlc.arg(character_id) AND account_id=sqlc.arg(account_id) AND deleted_at IS NULL;

-- name: RecordCharacterFame :one
UPDATE characters SET max_fame=max(max_fame,CAST(sqlc.arg(current_fame) AS INTEGER))
WHERE account_id=sqlc.arg(account_id) AND id=sqlc.arg(character_id) AND deleted_at IS NULL RETURNING max_fame;

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

-- name: ShareActiveCharacterState :one
SELECT state FROM characters WHERE account_id=sqlc.arg(account_id) AND id=sqlc.arg(character_id) AND deleted_at IS NULL;

-- name: LockActiveCharacterState :one
SELECT state FROM characters WHERE account_id=sqlc.arg(account_id) AND id=sqlc.arg(character_id) AND deleted_at IS NULL;

-- name: LockActiveCharacterID :one
SELECT id FROM characters WHERE id=sqlc.arg(character_id) AND deleted_at IS NULL;

-- name: ShareActiveCharacterIDs :many
SELECT id FROM characters WHERE account_id=sqlc.arg(account_id) AND deleted_at IS NULL ORDER BY id;

-- name: AccountCharacterStates :many
SELECT state FROM characters WHERE account_id=sqlc.arg(account_id);

-- name: PendingMoonRewardRuns :many
SELECT substr(event_key,12) AS run_id FROM character_events e
WHERE e.character_id=sqlc.arg(character_id) AND e.model=sqlc.arg(model) AND e.event_key LIKE 'moon-clear:%'
AND NOT EXISTS(SELECT 1 FROM character_events g WHERE g.character_id=e.character_id
AND g.event_key='moon-grant:'||substr(e.event_key,12)) ORDER BY created_at LIMIT 16;

-- name: CharactersWithAdventure :many
-- json_set returns TEXT, and a TEXT value in a BLOB json column cannot be scanned
-- into json.RawMessage, so the result is cast back to BLOB. Wrapping the merged
-- value in json(...) keeps it a JSON value rather than a quoted string.
SELECT c.id,c.account_id,c.wire_id,c.name,c.profession,c.create_request,c.config_version,
CAST(json_set(c.state,'$.season_level',json(COALESCE(json_extract(a.data,'$.season_level'),'{}'))) AS BLOB) AS state,
c.created_at,c.fixed_slot FROM characters c LEFT JOIN account_adventures a ON a.account_id=c.account_id
WHERE c.account_id=sqlc.arg(account_id) AND c.deleted_at IS NULL
ORDER BY coalesce(c.roster_order,c.wire_id),c.wire_id;

-- name: CharacterSeason :one
-- Same BLOB round trip as CharactersWithAdventure: json_object returns TEXT.
SELECT CAST(json_object('season_level',json(COALESCE(json_extract(a.data,'$.season_level'),'{}'))) AS BLOB) AS state
FROM characters c LEFT JOIN account_adventures a ON a.account_id=c.account_id
WHERE c.account_id=sqlc.arg(account_id) AND c.id=sqlc.arg(character_id) AND c.deleted_at IS NULL;
