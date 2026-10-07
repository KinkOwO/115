-- SQLite query fork of sql/postgres/queries/progression.sql (dual-engine, S3).
--
-- Ported for the progression slice: world/channel position, birth, fatigue,
-- fatigue recovery, omen state, oath progress, tower progress and the run
-- ledgers. The PostgreSQL file stays untouched and remains authoritative for the
-- PostgreSQL path; every `-- name:` marker below is byte-identical to it.
--
-- Dialect rules applied (port guide sec.1, design doc sec.3.2):
--   * FOR UPDATE is removed (LockOathProgress). SQLite has no row locks; the
--     engine serializes writers with BEGIN IMMEDIATE. The query keeps its name
--     and its full ownership predicate, so callers are unchanged.
--   * `UPDATE ... FROM` is rewritten as a correlated EXISTS ownership guard (see
--     SaveWorld and AdvanceBirth); the sqlc sqlite grammar rejects the clause
--     outright (`no viable alternative at input 'UPDATE character_world w'`).
--     characters.id is the primary key, so the PostgreSQL join matched at most
--     one row and the rewrite updates exactly the same rows. Outer columns are
--     qualified with the table name; argument order is kept identical.
--   * Integer parameters are left bare so they inherit the column width from the
--     sqlite DDL, which mirrors the PostgreSQL widths (SMALLINT/INTEGER/BIGINT).
--     A CAST type name ignores those overrides and always yields int64, so CAST
--     is only used where an expression genuinely needs a type (R-1).
--   * now() -> (CAST((julianday('now') - 2440587.5) * 86400000000 AS INTEGER)),
--     the integer-microsecond expression already used by core.sql.
--   * `(x ->> 'k')::bigint` -> CAST(json_extract(x,'$.k') AS INTEGER).
--   * jsonb_build_object -> json_object, GREATEST -> scalar max.
--   * `= ANY(sqlc.arg(towns)::bigint[])` -> `IN (SELECT ... FROM json_each(...))`
--     with the list passed as JSON text (D9).
--
-- DATE columns: they are written and read as ISO-8601 'YYYY-MM-DD' TEXT here.
--   * `$n::text::date` -> CAST(sqlc.arg(d) AS TEXT). Callers already build the
--     string as time.Now().Format("2006-01-02") (internal/character/fatigue.go),
--     so the stored bytes are exactly what PostgreSQL stored for the same call.
--   * `col::text` (a date) -> CAST(col AS TEXT), and `GREATEST(a,b)` -> max(a,b).
--     Fixed-width ISO text orders lexicographically exactly like a date, so the
--     rollover comparisons in LoadFatigue and the `<>` test in AdvanceTowerGrief
--     keep PostgreSQL's chronological semantics. Verified against the pinned
--     driver (modernc.org/sqlite, SQLite 3.53.4): typeof() is 'text' and the
--     round-trip through LoadFatigue/ReserveTowerEntry/AdvanceTower* is exact.
--   * This is deliberately NOT the integer-microsecond DATE representation the
--     sqlite DDL header describes for the general case; design doc D14 allows
--     either ("TEXT/integer storage"). SQLite only reports a decltype for a bare
--     column reference, and the driver's _inttotime conversion needs that
--     decltype, so a bare DATE column scans as time.Time. The PostgreSQL queries
--     here return a date *string* (`day::text`), and the only way to produce a
--     string with the PostgreSQL field name is CAST(<column> AS TEXT) over TEXT
--     storage. Every DATE column in this file (character_fatigue.day,
--     character_fatigue_rooms.day, character_fatigue_recovery.day,
--     account_tower_progress.entry_day, account_tower_grief_progress.cleared_day)
--     is written and read only by the queries below, so it stays self-consistent.
--     npc_favor.last_gift_day belongs to adventure.sql and is not in this slice.
--
-- sqlc v1.31.1 sqlite-engine traps verified against this schema and worked around
-- below (each would otherwise generate code that fails only at runtime):
--   * A parameter inside `FILTER (WHERE ...)` is dropped: the clause is copied
--     through verbatim, the generated struct loses the parameter entirely, and
--     the literal text `sqlc.arg(day)` ships inside the SQL. `daily_uses` is
--     therefore spelled COALESCE(SUM(CASE WHEN day=<d> THEN 1 ELSE 0 END),0) --
--     the exact equivalent of count(*) FILTER (WHERE day=<d>), including 0 rather
--     than NULL when no row matches. (A FILTER with no parameter inside is fine;
--     that is why core.sql keeps one.)
--   * A parameter inside `ON CONFLICT ... DO UPDATE SET`/`WHERE` is likewise
--     copied through unbound. No query here has one: the DO UPDATE clauses only
--     reference EXCLUDED/character_* columns and now().
--   * An alias in `RETURNING <expr> AS <name>` is ignored for naming: only a
--     bare column or CAST(<column> ...) comes back under its PostgreSQL name.
--     AdvanceTowerFloor keeps PostgreSQL's defensive COALESCE(entry_day::text,'')
--     (value semantics preserved) and therefore exposes that one result column as
--     `Column2` instead of `EntryDay`; every other returned date expression is
--     either a plain CAST of a column or assigned in the same statement, so it
--     keeps its PostgreSQL name.
--
-- Known divergences from the PostgreSQL side that no SQL rewrite can fix (each
-- needs an out-of-file decision; reported to the caller):
--   * Integer widths where the two DDLs disagree. PostgreSQL declares
--     character_omen_state.dungeon_id, character_oath_progress.dungeon_id and
--     character_fatigue_recovery.template as bigint, while the sqlite DDL
--     currently declares them INTEGER, so bare parameters bound to those columns
--     infer int32 instead of int64 (measured with the current sqlc.yaml). The
--     five tower `floor` parameters are int16 here against PostgreSQL int32,
--     because PostgreSQL casts them to integer explicitly
--     (`$1::integer`) while the column they are compared with is smallint and a
--     CAST type name always yields int64 on the sqlite side. Both are integer
--     width differences only, which generated_parity_test.go's widthEquivalent
--     tolerates; the three columns align automatically once the sqlite DDL uses
--     bigint.
--   * FatigueRecoveryUsage.last_used is `int64` here (integer microseconds, 0 for
--     the empty case, which is exactly COALESCE(max(used_at),'epoch')) where
--     PostgreSQL returns time.Time. The driver converts an integer to time.Time
--     only when SQLite reports a DATE/DATETIME/TIMESTAMP decltype, and that is
--     only reported for a bare column reference -- an aggregate such as
--     max(used_at) has no decltype, so a time.Time scan target fails with
--     "unsupported Scan, storing driver.Value type int64 into type *time.Time"
--     (measured on modernc.org/sqlite v1.59.0 + SQLite 3.53.4). The adapter must
--     convert these microseconds to time.Time.
--   * ScrubPollutedWorldPositions takes its bigint[] as JSON text, so the
--     parameter is `string` here and `[]int64` in PostgreSQL (D9: the adapter
--     owns the encoding).
--   * AdvanceTowerFloorRow.EntryDay is generated as Column2 (see the RETURNING
--     trap above).
--   * json_object emits keys in the written order; PostgreSQL jsonb canonicalizes
--     key order, so the stored bytes of a fresh RecordIspinsWeeklyClear outcome
--     can differ in key order. The JSON value itself is identical.

-- name: EnsureWorld :exec
-- The INSERT ... SELECT carries a WHERE clause: SQLite requires one whenever an
-- upsert clause follows a SELECT, so BackfillBirth adds `WHERE true` below. This
-- query already has a real predicate.
INSERT INTO character_world(character_id,position,config_version)
SELECT id,sqlc.arg(position),sqlc.arg(config_version) FROM characters
WHERE id=sqlc.arg(character_id) AND account_id=sqlc.arg(account_id) ON CONFLICT DO NOTHING;

-- name: LoadWorld :one
SELECT w.position,w.revision,w.config_version FROM character_world w JOIN characters c ON c.id=w.character_id
WHERE c.account_id=sqlc.arg(account_id) AND c.id=sqlc.arg(character_id);

-- name: EnterChannelWorld :exec
INSERT INTO character_channel_world(character_id,channel_type,position,config_version)
SELECT id,sqlc.arg(channel_type),sqlc.arg(position),sqlc.arg(config_version) FROM characters
WHERE id=sqlc.arg(character_id) AND account_id=sqlc.arg(account_id)
ON CONFLICT (character_id,channel_type) DO UPDATE SET position=EXCLUDED.position,
updated_at=(CAST((julianday('now') - 2440587.5) * 86400000000 AS INTEGER));

-- name: SaveWorld :execrows
UPDATE character_world SET position=sqlc.arg(position),revision=revision+1,
updated_at=(CAST((julianday('now') - 2440587.5) * 86400000000 AS INTEGER))
WHERE EXISTS(SELECT 1 FROM characters c WHERE c.id=character_world.character_id AND c.account_id=sqlc.arg(account_id))
AND character_id=sqlc.arg(character_id) AND revision=sqlc.arg(revision);

-- name: ScrubPollutedWorldPositions :execrows
DELETE FROM character_world
WHERE CAST(json_extract(position,'$.town') AS INTEGER)
IN (SELECT CAST(value AS INTEGER) FROM json_each(CAST(sqlc.arg(towns) AS TEXT)));

-- name: BackfillBirth :exec
-- `WHERE true` is required by SQLite: an upsert clause attached to an
-- INSERT ... SELECT needs the SELECT to have a WHERE clause, otherwise the
-- statement fails at runtime with `near "DO": syntax error` (measured). The
-- predicate is a tautology, so the inserted rows are unchanged.
INSERT INTO character_birth(character_id,stage)
SELECT id,sqlc.arg(stage) FROM characters WHERE true ON CONFLICT(character_id) DO NOTHING;

-- name: BirthStage :one
SELECT b.stage,b.dungeon FROM character_birth b JOIN characters c ON c.id=b.character_id
WHERE c.account_id=sqlc.arg(account_id) AND c.id=sqlc.arg(character_id) AND c.deleted_at IS NULL;

-- name: ActiveCharacterOwned :one
SELECT EXISTS(SELECT 1 FROM characters WHERE account_id=sqlc.arg(account_id)
AND id=sqlc.arg(character_id) AND deleted_at IS NULL);

-- name: StartBirth :execrows
INSERT INTO character_birth(character_id,stage)
SELECT id,sqlc.arg(stage) FROM characters
WHERE account_id=sqlc.arg(account_id) AND id=sqlc.arg(character_id) ON CONFLICT(character_id) DO NOTHING;

-- name: AdvanceBirth :execrows
UPDATE character_birth SET stage=sqlc.arg(stage),dungeon=sqlc.arg(dungeon),
updated_at=(CAST((julianday('now') - 2440587.5) * 86400000000 AS INTEGER))
WHERE EXISTS(SELECT 1 FROM characters c WHERE c.id=character_birth.character_id AND c.account_id=sqlc.arg(account_id)
AND c.deleted_at IS NULL)
AND character_id=sqlc.arg(character_id) AND stage<sqlc.arg(stage);

-- name: LoadFatigue :one
INSERT INTO character_fatigue(character_id,day,daily_limit)
SELECT id,CAST(sqlc.arg(day) AS TEXT),sqlc.arg(daily_limit) FROM characters
WHERE account_id=sqlc.arg(account_id) AND id=sqlc.arg(character_id)
ON CONFLICT(character_id) DO UPDATE SET
used=CASE WHEN EXCLUDED.day>character_fatigue.day THEN 0 ELSE character_fatigue.used END,
used_max=CASE WHEN EXCLUDED.day>character_fatigue.day THEN 0 ELSE character_fatigue.used_max END,
daily_limit=CASE WHEN EXCLUDED.day>=character_fatigue.day THEN EXCLUDED.daily_limit ELSE character_fatigue.daily_limit END,
day=CAST(max(EXCLUDED.day,character_fatigue.day) AS TEXT),
updated_at=(CAST((julianday('now') - 2440587.5) * 86400000000 AS INTEGER))
RETURNING CAST(day AS TEXT) AS day,used,daily_limit,used_max;

-- name: LoadLockedCharacterFatigue :one
INSERT INTO character_fatigue(character_id,day,daily_limit)
VALUES(sqlc.arg(character_id),CAST(sqlc.arg(day) AS TEXT),sqlc.arg(daily_limit))
ON CONFLICT(character_id) DO UPDATE SET
used=CASE WHEN EXCLUDED.day>character_fatigue.day THEN 0 ELSE character_fatigue.used END,
used_max=CASE WHEN EXCLUDED.day>character_fatigue.day THEN 0 ELSE character_fatigue.used_max END,
daily_limit=CASE WHEN EXCLUDED.day>=character_fatigue.day THEN EXCLUDED.daily_limit ELSE character_fatigue.daily_limit END,
day=CAST(max(EXCLUDED.day,character_fatigue.day) AS TEXT),
updated_at=(CAST((julianday('now') - 2440587.5) * 86400000000 AS INTEGER))
RETURNING CAST(day AS TEXT) AS day,used,daily_limit,used_max;

-- name: FatigueRoomRecorded :one
SELECT EXISTS(SELECT 1 FROM character_fatigue_rooms WHERE character_id=sqlc.arg(character_id)
AND run_id=sqlc.arg(run_id) AND map_id=sqlc.arg(map_id));

-- name: RunPaidFatigue :one
SELECT EXISTS(SELECT 1 FROM character_fatigue_rooms WHERE character_id=sqlc.arg(character_id)
AND run_id=sqlc.arg(run_id) AND cost>0);

-- name: RecordFatigueRoom :exec
INSERT INTO character_fatigue_rooms(character_id,run_id,map_id,day,cost)
VALUES(sqlc.arg(character_id),sqlc.arg(run_id),sqlc.arg(map_id),CAST(sqlc.arg(day) AS TEXT),sqlc.arg(cost));

-- name: SaveFatigueCharge :exec
UPDATE character_fatigue SET used=sqlc.arg(used),used_max=sqlc.arg(used_max),
updated_at=(CAST((julianday('now') - 2440587.5) * 86400000000 AS INTEGER))
WHERE character_id=sqlc.arg(character_id);

-- name: FatigueRecoveryUsage :one
SELECT CAST(COALESCE(SUM(CASE WHEN day=CAST(sqlc.arg(day) AS TEXT) THEN 1 ELSE 0 END),0) AS INTEGER) AS daily_uses,
CAST(COALESCE(max(used_at),0) AS INTEGER) AS last_used,
CAST(count(*)>0 AS BOOLEAN) AS has_last_used FROM character_fatigue_recovery
WHERE character_id=sqlc.arg(character_id) AND template=sqlc.arg(template);

-- name: SaveFatigueRecovery :exec
UPDATE character_fatigue SET used=sqlc.arg(used),
updated_at=(CAST((julianday('now') - 2440587.5) * 86400000000 AS INTEGER)) WHERE character_id=sqlc.arg(character_id);

-- name: RecordFatigueRecovery :exec
INSERT INTO character_fatigue_recovery(character_id,template,day,ordinal,restored,used_at)
-- `ordinal` is PostgreSQL `::bigint`, so it keeps the int64 width via the CAST.
VALUES(sqlc.arg(character_id),sqlc.arg(template),CAST(sqlc.arg(day) AS TEXT),
CAST(sqlc.arg(ordinal) AS INTEGER),sqlc.arg(restored),sqlc.arg(used_at));

-- name: RunMonsterExperience :one
SELECT CAST(COALESCE(SUM(CAST(json_extract(e.outcome,'$.gain') AS INTEGER)),0) AS INTEGER) AS total
FROM character_events e JOIN characters c ON c.id=e.character_id
WHERE c.account_id=sqlc.arg(account_id) AND c.id=sqlc.arg(character_id) AND e.event_key LIKE sqlc.arg(event_pattern);

-- name: RunFatigueLedger :one
SELECT CAST(COALESCE(SUM(f.cost),0) AS INTEGER) AS charged,CAST(COUNT(*) AS INTEGER) AS rooms
FROM character_fatigue_rooms f JOIN characters c ON c.id=f.character_id
WHERE c.account_id=sqlc.arg(account_id) AND c.id=sqlc.arg(character_id) AND f.run_id=sqlc.arg(run_id);

-- name: IspinsWeeklyUsed :one
SELECT EXISTS(SELECT 1 FROM character_events e JOIN characters c ON c.id=e.character_id
WHERE c.id=sqlc.arg(character_id) AND c.account_id=sqlc.arg(account_id) AND c.deleted_at IS NULL
AND e.model=sqlc.arg(model) AND json_extract(e.outcome,'$.week')=CAST(sqlc.arg(week) AS TEXT));

-- name: LockedIspinsWeeklyUsed :one
SELECT EXISTS(SELECT 1 FROM character_events WHERE character_id=sqlc.arg(character_id)
AND model=sqlc.arg(model) AND json_extract(outcome,'$.week')=CAST(sqlc.arg(week) AS TEXT));

-- name: RecordIspinsWeeklyClear :exec
-- json_object returns TEXT; character_events.outcome is a BLOB column with BLOB
-- affinity, which stores TEXT unchanged, so the JSON is cast to BLOB to keep the
-- byte round-trip database/sql needs for a json.RawMessage scan target (D24).
INSERT INTO character_events(character_id,event_key,config_version,model,outcome)
VALUES(sqlc.arg(character_id),sqlc.arg(event_key),sqlc.arg(config_version),sqlc.arg(model),
CAST(json_object('week',CAST(sqlc.arg(week) AS TEXT),'run',CAST(sqlc.arg(run) AS TEXT)) AS BLOB));

-- name: OdysseyGraduationAlreadyPaid :one
SELECT EXISTS(SELECT 1 FROM character_events WHERE character_id=sqlc.arg(character_id)
AND event_key IN('odyssey-graduate-reward-v1','odyssey-honor-mail-v1'));

-- name: OmenState :one
SELECT held,orthaire_pending FROM character_omen_state WHERE character_id=sqlc.arg(character_id) AND dungeon_id=sqlc.arg(dungeon_id);

-- name: SaveOmenHeld :exec
-- PostgreSQL casts `held` to bigint even though the column is integer, so the
-- CAST keeps the int64 width on this side too (measured: CAST(... AS INTEGER)
-- ignores the yaml width overrides and always yields int64).
INSERT INTO character_omen_state(character_id,dungeon_id,held,updated_at)
VALUES(sqlc.arg(character_id),sqlc.arg(dungeon_id),CAST(sqlc.arg(held) AS INTEGER),
(CAST((julianday('now') - 2440587.5) * 86400000000 AS INTEGER)))
ON CONFLICT(character_id,dungeon_id) DO UPDATE SET held=EXCLUDED.held,
updated_at=(CAST((julianday('now') - 2440587.5) * 86400000000 AS INTEGER));

-- name: SetOmenPending :exec
INSERT INTO character_omen_state(character_id,dungeon_id,orthaire_pending,updated_at)
VALUES(sqlc.arg(character_id),sqlc.arg(dungeon_id),sqlc.arg(pending),
(CAST((julianday('now') - 2440587.5) * 86400000000 AS INTEGER)))
ON CONFLICT(character_id,dungeon_id) DO UPDATE SET orthaire_pending=EXCLUDED.orthaire_pending,
updated_at=(CAST((julianday('now') - 2440587.5) * 86400000000 AS INTEGER));

-- name: OathProgressClears :one
SELECT clears FROM character_oath_progress WHERE character_id=sqlc.arg(character_id) AND dungeon_id=sqlc.arg(dungeon_id);

-- name: LockOathProgress :one
-- Lock query; the FOR UPDATE clause is replaced by the engine-wide BEGIN
-- IMMEDIATE write serialization (see the file header). Name and predicate are
-- unchanged, so the caller sees the same API.
SELECT clears FROM character_oath_progress WHERE character_id=sqlc.arg(character_id) AND dungeon_id=sqlc.arg(dungeon_id);

-- name: SaveOathProgress :exec
-- `clears` is PostgreSQL `::bigint`, so it keeps the int64 width via the CAST.
INSERT INTO character_oath_progress(character_id,dungeon_id,clears,updated_at)
VALUES(sqlc.arg(character_id),sqlc.arg(dungeon_id),CAST(sqlc.arg(clears) AS INTEGER),
(CAST((julianday('now') - 2440587.5) * 86400000000 AS INTEGER)))
ON CONFLICT(character_id,dungeon_id) DO UPDATE SET clears=EXCLUDED.clears,
updated_at=(CAST((julianday('now') - 2440587.5) * 86400000000 AS INTEGER));

-- name: EnsureTowerProgress :exec
INSERT INTO account_tower_progress(account_id,tower_key,highest_cleared)
VALUES(sqlc.arg(account_id),sqlc.arg(tower_key),sqlc.arg(highest_cleared)) ON CONFLICT(account_id,tower_key) DO NOTHING;

-- name: ReadTowerProgress :one
SELECT highest_cleared,CAST(COALESCE(entry_day,'') AS TEXT) AS entry_day,entries_today,last_run_id
FROM account_tower_progress WHERE account_id=sqlc.arg(account_id) AND tower_key=sqlc.arg(tower_key);

-- name: ReserveTowerEntry :one
-- entry_day is assigned in this same statement, so the returned
-- CAST(entry_day AS TEXT) is non-NULL and keeps the plain `string` shape
-- PostgreSQL returns from entry_day::text.
UPDATE account_tower_progress SET entry_day=CAST(sqlc.arg(day) AS TEXT),
entries_today=CASE WHEN entry_day=CAST(sqlc.arg(day) AS TEXT) THEN entries_today+1 ELSE 1 END
WHERE account_id=sqlc.arg(account_id) AND tower_key=sqlc.arg(tower_key) AND highest_cleared+1=sqlc.arg(floor)
RETURNING highest_cleared,CAST(entry_day AS TEXT) AS entry_day,entries_today,last_run_id;

-- name: AdvanceTowerFloor :one
-- PostgreSQL wraps the returned date in COALESCE(entry_day::text,''), and that
-- expression is returned unchanged here: the sqlc sqlite engine names an
-- aliased RETURNING expression ColumnN (see the header), so this one result
-- column is generated as Column2 instead of EntryDay.
UPDATE account_tower_progress SET highest_cleared=sqlc.arg(floor),last_run_id=sqlc.arg(run_id)
WHERE account_id=sqlc.arg(account_id) AND tower_key=sqlc.arg(tower_key)
AND highest_cleared=sqlc.arg(floor)-1 AND entries_today>0
RETURNING highest_cleared,CAST(COALESCE(entry_day,'') AS TEXT) AS entry_day,entries_today,last_run_id;

-- name: ReadTowerGriefProgress :one
SELECT highest_cleared,CAST(COALESCE(cleared_day,'') AS TEXT) AS cleared_day,last_run_id
FROM account_tower_grief_progress WHERE account_id=sqlc.arg(account_id);

-- name: EnsureTowerGriefProgress :exec
INSERT INTO account_tower_grief_progress(account_id,highest_cleared)
VALUES(sqlc.arg(account_id),sqlc.arg(highest_cleared)) ON CONFLICT(account_id) DO NOTHING;

-- name: AdvanceTowerGrief :one
-- cleared_day is assigned in this same statement, so the returned
-- CAST(cleared_day AS TEXT) is non-NULL and matches cleared_day::text.
UPDATE account_tower_grief_progress SET highest_cleared=sqlc.arg(floor),
cleared_day=CAST(sqlc.arg(day) AS TEXT),last_run_id=sqlc.arg(run_id)
WHERE account_id=sqlc.arg(account_id) AND highest_cleared=sqlc.arg(floor)-1
AND (cleared_day IS NULL OR cleared_day<>CAST(sqlc.arg(day) AS TEXT))
RETURNING highest_cleared,CAST(cleared_day AS TEXT) AS cleared_day,last_run_id;
