-- name: EnsureWorld :exec
INSERT INTO character_world(character_id,position,config_version)
SELECT id,sqlc.arg(position),sqlc.arg(config_version) FROM characters
WHERE id=sqlc.arg(character_id) AND account_id=sqlc.arg(account_id) ON CONFLICT DO NOTHING;

-- name: LoadWorld :one
SELECT w.position,w.revision,w.config_version FROM character_world w JOIN characters c ON c.id=w.character_id
WHERE c.account_id=sqlc.arg(account_id) AND c.id=sqlc.arg(character_id);

-- name: EnterChannelWorld :exec
INSERT INTO character_channel_world(character_id,channel_type,position,config_version)
SELECT id,sqlc.arg(channel_type)::integer,sqlc.arg(position),sqlc.arg(config_version) FROM characters
WHERE id=sqlc.arg(character_id) AND account_id=sqlc.arg(account_id)
ON CONFLICT (character_id,channel_type) DO UPDATE SET position=EXCLUDED.position,updated_at=now();

-- name: SaveWorld :execrows
UPDATE character_world w SET position=sqlc.arg(position),revision=revision+1,updated_at=now()
FROM characters c WHERE c.id=w.character_id AND c.account_id=sqlc.arg(account_id)
AND c.id=sqlc.arg(character_id) AND w.revision=sqlc.arg(revision);

-- name: ScrubPollutedWorldPositions :execrows
DELETE FROM character_world WHERE (position->>'town')::bigint=ANY(sqlc.arg(towns)::bigint[]);

-- name: BackfillBirth :exec
INSERT INTO character_birth(character_id,stage)
SELECT id,sqlc.arg(stage)::smallint FROM characters ON CONFLICT(character_id) DO NOTHING;

-- name: BirthStage :one
SELECT b.stage,b.dungeon FROM character_birth b JOIN characters c ON c.id=b.character_id
WHERE c.account_id=sqlc.arg(account_id) AND c.id=sqlc.arg(character_id) AND c.deleted_at IS NULL;

-- name: ActiveCharacterOwned :one
SELECT EXISTS(SELECT 1 FROM characters WHERE account_id=sqlc.arg(account_id)
AND id=sqlc.arg(character_id) AND deleted_at IS NULL);

-- name: StartBirth :execrows
INSERT INTO character_birth(character_id,stage)
SELECT id,sqlc.arg(stage)::smallint FROM characters
WHERE account_id=sqlc.arg(account_id) AND id=sqlc.arg(character_id) ON CONFLICT(character_id) DO NOTHING;

-- name: AdvanceBirth :execrows
UPDATE character_birth b SET stage=sqlc.arg(stage)::smallint,dungeon=sqlc.arg(dungeon)::bigint,updated_at=now()
FROM characters c WHERE c.id=b.character_id AND c.account_id=sqlc.arg(account_id)
AND c.id=sqlc.arg(character_id) AND c.deleted_at IS NULL AND b.stage<sqlc.arg(stage)::smallint;

-- name: LoadFatigue :one
INSERT INTO character_fatigue(character_id,day,daily_limit)
SELECT id,sqlc.arg(day)::text::date,sqlc.arg(daily_limit)::integer FROM characters
WHERE account_id=sqlc.arg(account_id) AND id=sqlc.arg(character_id)
ON CONFLICT(character_id) DO UPDATE SET
used=CASE WHEN EXCLUDED.day>character_fatigue.day THEN 0 ELSE character_fatigue.used END,
used_max=CASE WHEN EXCLUDED.day>character_fatigue.day THEN 0 ELSE character_fatigue.used_max END,
daily_limit=CASE WHEN EXCLUDED.day>=character_fatigue.day THEN EXCLUDED.daily_limit ELSE character_fatigue.daily_limit END,
day=GREATEST(EXCLUDED.day,character_fatigue.day),updated_at=now()
RETURNING day::text,used,daily_limit,used_max;

-- name: LoadLockedCharacterFatigue :one
INSERT INTO character_fatigue(character_id,day,daily_limit)
VALUES(sqlc.arg(character_id),sqlc.arg(day)::text::date,sqlc.arg(daily_limit)::integer)
ON CONFLICT(character_id) DO UPDATE SET
used=CASE WHEN EXCLUDED.day>character_fatigue.day THEN 0 ELSE character_fatigue.used END,
used_max=CASE WHEN EXCLUDED.day>character_fatigue.day THEN 0 ELSE character_fatigue.used_max END,
daily_limit=CASE WHEN EXCLUDED.day>=character_fatigue.day THEN EXCLUDED.daily_limit ELSE character_fatigue.daily_limit END,
day=GREATEST(EXCLUDED.day,character_fatigue.day),updated_at=now()
RETURNING day::text,used,daily_limit,used_max;

-- name: FatigueRoomRecorded :one
SELECT EXISTS(SELECT 1 FROM character_fatigue_rooms WHERE character_id=sqlc.arg(character_id)
AND run_id=sqlc.arg(run_id) AND map_id=sqlc.arg(map_id));

-- name: RunPaidFatigue :one
SELECT EXISTS(SELECT 1 FROM character_fatigue_rooms WHERE character_id=sqlc.arg(character_id)
AND run_id=sqlc.arg(run_id) AND cost>0);

-- name: RecordFatigueRoom :exec
INSERT INTO character_fatigue_rooms(character_id,run_id,map_id,day,cost)
VALUES(sqlc.arg(character_id),sqlc.arg(run_id),sqlc.arg(map_id),sqlc.arg(day)::text::date,sqlc.arg(cost));

-- name: SaveFatigueCharge :exec
UPDATE character_fatigue SET used=sqlc.arg(used),used_max=sqlc.arg(used_max),updated_at=now()
WHERE character_id=sqlc.arg(character_id);

-- name: FatigueRecoveryUsage :one
SELECT count(*) FILTER(WHERE day=sqlc.arg(day)::text::date)::bigint AS daily_uses,
COALESCE(max(used_at),'epoch'::timestamptz)::timestamptz AS last_used,
count(*)>0 AS has_last_used FROM character_fatigue_recovery
WHERE character_id=sqlc.arg(character_id) AND template=sqlc.arg(template);

-- name: SaveFatigueRecovery :exec
UPDATE character_fatigue SET used=sqlc.arg(used),updated_at=now() WHERE character_id=sqlc.arg(character_id);

-- name: RecordFatigueRecovery :exec
INSERT INTO character_fatigue_recovery(character_id,template,day,ordinal,restored,used_at)
VALUES(sqlc.arg(character_id),sqlc.arg(template),sqlc.arg(day)::text::date,
sqlc.arg(ordinal)::bigint,sqlc.arg(restored),sqlc.arg(used_at));

-- name: RunMonsterExperience :one
SELECT COALESCE(SUM((e.outcome->>'gain')::bigint),0)::bigint AS total
FROM character_events e JOIN characters c ON c.id=e.character_id
WHERE c.account_id=sqlc.arg(account_id) AND c.id=sqlc.arg(character_id) AND e.event_key LIKE sqlc.arg(event_pattern);

-- name: RunFatigueLedger :one
SELECT COALESCE(SUM(f.cost),0)::bigint AS charged,COUNT(*)::bigint AS rooms
FROM character_fatigue_rooms f JOIN characters c ON c.id=f.character_id
WHERE c.account_id=sqlc.arg(account_id) AND c.id=sqlc.arg(character_id) AND f.run_id=sqlc.arg(run_id);

-- name: IspinsWeeklyUsed :one
SELECT EXISTS(SELECT 1 FROM character_events e JOIN characters c ON c.id=e.character_id
WHERE c.id=sqlc.arg(character_id) AND c.account_id=sqlc.arg(account_id) AND c.deleted_at IS NULL
AND e.model=sqlc.arg(model) AND e.outcome->>'week'=sqlc.arg(week)::text);

-- name: LockedIspinsWeeklyUsed :one
SELECT EXISTS(SELECT 1 FROM character_events WHERE character_id=sqlc.arg(character_id)
AND model=sqlc.arg(model) AND outcome->>'week'=sqlc.arg(week)::text);

-- name: RecordIspinsWeeklyClear :exec
INSERT INTO character_events(character_id,event_key,config_version,model,outcome)
VALUES(sqlc.arg(character_id),sqlc.arg(event_key),sqlc.arg(config_version),sqlc.arg(model),
jsonb_build_object('week',sqlc.arg(week)::text,'run',sqlc.arg(run)::text));

-- name: OdysseyGraduationAlreadyPaid :one
SELECT EXISTS(SELECT 1 FROM character_events WHERE character_id=sqlc.arg(character_id)
AND event_key IN('odyssey-graduate-reward-v1','odyssey-honor-mail-v1'));

-- name: OmenState :one
SELECT held,orthaire_pending FROM character_omen_state WHERE character_id=sqlc.arg(character_id) AND dungeon_id=sqlc.arg(dungeon_id);

-- name: SaveOmenHeld :exec
INSERT INTO character_omen_state(character_id,dungeon_id,held,updated_at)
VALUES(sqlc.arg(character_id),sqlc.arg(dungeon_id),sqlc.arg(held)::bigint,now())
ON CONFLICT(character_id,dungeon_id) DO UPDATE SET held=EXCLUDED.held,updated_at=now();

-- name: SetOmenPending :exec
INSERT INTO character_omen_state(character_id,dungeon_id,orthaire_pending,updated_at)
VALUES(sqlc.arg(character_id),sqlc.arg(dungeon_id),sqlc.arg(pending),now())
ON CONFLICT(character_id,dungeon_id) DO UPDATE SET orthaire_pending=EXCLUDED.orthaire_pending,updated_at=now();

-- name: OathProgressClears :one
SELECT clears FROM character_oath_progress WHERE character_id=sqlc.arg(character_id) AND dungeon_id=sqlc.arg(dungeon_id);

-- name: LockOathProgress :one
SELECT clears FROM character_oath_progress WHERE character_id=sqlc.arg(character_id) AND dungeon_id=sqlc.arg(dungeon_id) FOR UPDATE;

-- name: SaveOathProgress :exec
INSERT INTO character_oath_progress(character_id,dungeon_id,clears,updated_at)
VALUES(sqlc.arg(character_id),sqlc.arg(dungeon_id),sqlc.arg(clears)::bigint,now())
ON CONFLICT(character_id,dungeon_id) DO UPDATE SET clears=EXCLUDED.clears,updated_at=now();

-- name: EnsureTowerProgress :exec
INSERT INTO account_tower_progress(account_id,tower_key,highest_cleared)
VALUES(sqlc.arg(account_id),sqlc.arg(tower_key),sqlc.arg(highest_cleared)::integer) ON CONFLICT(account_id,tower_key) DO NOTHING;

-- name: ReadTowerProgress :one
SELECT highest_cleared,COALESCE(entry_day::text,'')::text AS entry_day,entries_today,last_run_id
FROM account_tower_progress WHERE account_id=sqlc.arg(account_id) AND tower_key=sqlc.arg(tower_key);

-- name: ReserveTowerEntry :one
UPDATE account_tower_progress SET entry_day=sqlc.arg(day)::text::date,
entries_today=CASE WHEN entry_day=sqlc.arg(day)::text::date THEN entries_today+1 ELSE 1 END
WHERE account_id=sqlc.arg(account_id) AND tower_key=sqlc.arg(tower_key) AND highest_cleared+1=sqlc.arg(floor)::integer
RETURNING highest_cleared,entry_day::text,entries_today,last_run_id;

-- name: AdvanceTowerFloor :one
UPDATE account_tower_progress SET highest_cleared=sqlc.arg(floor)::integer,last_run_id=sqlc.arg(run_id)
WHERE account_id=sqlc.arg(account_id) AND tower_key=sqlc.arg(tower_key)
AND highest_cleared=sqlc.arg(floor)::integer-1 AND entries_today>0
RETURNING highest_cleared,COALESCE(entry_day::text,'')::text AS entry_day,entries_today,last_run_id;

-- name: ReadTowerGriefProgress :one
SELECT highest_cleared,COALESCE(cleared_day::text,'')::text AS cleared_day,last_run_id
FROM account_tower_grief_progress WHERE account_id=sqlc.arg(account_id);

-- name: EnsureTowerGriefProgress :exec
INSERT INTO account_tower_grief_progress(account_id,highest_cleared)
VALUES(sqlc.arg(account_id),sqlc.arg(highest_cleared)::integer) ON CONFLICT(account_id) DO NOTHING;

-- name: AdvanceTowerGrief :one
UPDATE account_tower_grief_progress SET highest_cleared=sqlc.arg(floor)::integer,
cleared_day=sqlc.arg(day)::text::date,last_run_id=sqlc.arg(run_id)
WHERE account_id=sqlc.arg(account_id) AND highest_cleared=sqlc.arg(floor)::integer-1
AND (cleared_day IS NULL OR cleared_day<>sqlc.arg(day)::text::date)
RETURNING highest_cleared,cleared_day::text,last_run_id;
