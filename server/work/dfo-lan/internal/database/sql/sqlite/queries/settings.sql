-- SQLite query fork of sql/postgres/queries/settings.sql (dual-engine, S3).
--
-- The PostgreSQL file stays untouched and remains authoritative for the PG path;
-- this fork only has to expose the same query names to the SQLite package.
--
-- Dialect rules applied (docs/sqlite-query-port-guide.md; core.sql's header has
-- the same list in context):
--   * The PostgreSQL source has no FOR UPDATE / FOR NO KEY UPDATE / FOR SHARE
--     clause in this file, so nothing had to be deleted here.
--   * The sqlite DDL mirrors the PostgreSQL column widths (BIGINT / SMALLINT /
--     INTEGER) and sqlc types a bare sqlc.arg() bound to such a column from that
--     column, so arguments stay uncast wherever the column supplies the width.
--     An explicit CAST(... AS INTEGER) is used only where the PostgreSQL side is
--     a bigint expression, because a cast type name is NOT covered by the db_type
--     overrides and always resolves to int64 (see RosterBackgroundUnlocks and
--     SkinFavorites). SPIKE RULE R-1 asks for the cast only when sqlc would
--     otherwise infer interface{}; every generated type was inspected and no
--     parameter in this file regressed to interface{}.
--   * The JSON parameter of InitializeProfileSkins (`state`) is deliberately NOT
--     cast: CAST(sqlc.arg(state) AS TEXT) would turn the parameter into a string,
--     which database/sql cannot scan into json.RawMessage (see core.sql, D24).
--     character_profile_skins.state has a per-column override, so the parameter
--     and the ProfileSkins result both stay json.RawMessage.
--   * now() -> integer microseconds, matching the sqlite DDL defaults and the
--     driver's _inttotime=1 + _time_integer_format=unix_micro representation.
--   * IS DISTINCT FROM is NOT kept as written. The bundled driver (SQLite
--     3.53.4) does implement it, but sqlc v1.31.1's SQLite grammar does not
--     parse it ("extraneous input 'FROM'", also seen in a plain UPDATE ... WHERE).
--     SQLite's null-safe `IS NOT` is the exact equivalent and is used instead
--     (both columns are NOT NULL, so `<>` would also be equivalent).
--   * Parameters inside an ON CONFLICT ... DO UPDATE clause (SET or WHERE) are
--     emitted VERBATIM by the SQLite engine -- sqlc does not rewrite
--     sqlc.arg(...) to a placeholder there, which would ship a broken statement
--     and still exit 0. UnlockRosterBackground therefore hoists its guard into
--     the parsed SELECT of the INSERT (see the comment on that query). Every
--     other upsert in this file has a parameter-free DO UPDATE clause.
--   * Everything else -- the plain ON CONFLICT DO NOTHING upserts, the
--     parameter-free DO UPDATE upserts and count(*) in CountSkinFavorites -- is
--     unchanged apart from the now() and JSON rules above.
--   * LIMIT/OFFSET: no LIMIT or OFFSET appears in this file, so the ordering rule
--     (LIMIT before OFFSET) has nothing to apply to here.
--
-- Nothing in the source needed the PostgreSQL-only escape hatch: there is no
-- current_database(), pg_try_advisory_lock*, pg_advisory_*, nextval, unnest,
-- plainto_tsquery or `= ANY(...)` in settings.sql. All 55 queries are ported.

-- name: MarkCharacterNotice :exec
INSERT INTO character_notice_seen(account_id,character_id,tree,notice_id)
VALUES(sqlc.arg(account_id),sqlc.arg(character_id),sqlc.arg(tree),sqlc.arg(notice_id)) ON CONFLICT DO NOTHING;

-- name: UnmarkCharacterNotice :exec
DELETE FROM character_notice_seen WHERE account_id=sqlc.arg(account_id) AND character_id=sqlc.arg(character_id)
AND tree=sqlc.arg(tree) AND notice_id=sqlc.arg(notice_id);

-- name: CharacterNoticeSeen :many
SELECT notice_id FROM character_notice_seen WHERE account_id=sqlc.arg(account_id) AND character_id=sqlc.arg(character_id)
AND tree=sqlc.arg(tree) ORDER BY notice_id;

-- name: SaveTutorialFlag :execrows
-- flag and completed are typed by the INSERT target columns (INTEGER, BOOLEAN),
-- which is exactly the PostgreSQL `::integer` / `::boolean` width.
INSERT INTO character_tutorial_flags(character_id,flag,completed)
SELECT id,sqlc.arg(flag),sqlc.arg(completed) FROM characters
WHERE account_id=sqlc.arg(account_id) AND id=sqlc.arg(character_id)
ON CONFLICT(character_id,flag) DO UPDATE SET completed=EXCLUDED.completed,updated_at=(CAST((julianday('now') - 2440587.5) * 86400000000 AS INTEGER));

-- name: TutorialFlags :many
SELECT flag FROM character_tutorial_flags WHERE character_id=sqlc.arg(character_id) AND completed ORDER BY flag;

-- name: SaveAccountWarpFavorite :exec
INSERT INTO account_warp_favorites(account_id,slot_index,value)
VALUES(sqlc.arg(account_id),sqlc.arg(slot_index),sqlc.arg(value))
ON CONFLICT(account_id,slot_index) DO UPDATE SET value=EXCLUDED.value,updated_at=(CAST((julianday('now') - 2440587.5) * 86400000000 AS INTEGER))
WHERE account_warp_favorites.value IS NOT EXCLUDED.value;

-- name: AccountWarpFavorites :many
SELECT slot_index,value FROM account_warp_favorites WHERE account_id=sqlc.arg(account_id) ORDER BY slot_index;

-- name: SaveEquipmentSkills :exec
INSERT INTO character_equipment_skill(character_id,skills,updated_at)
VALUES(sqlc.arg(character_id),sqlc.arg(skills),(CAST((julianday('now') - 2440587.5) * 86400000000 AS INTEGER))) ON CONFLICT(character_id)
DO UPDATE SET skills=EXCLUDED.skills,updated_at=(CAST((julianday('now') - 2440587.5) * 86400000000 AS INTEGER));

-- name: SaveEquipmentCommands :exec
INSERT INTO character_equipment_skill(character_id,commands,updated_at)
VALUES(sqlc.arg(character_id),sqlc.arg(commands),(CAST((julianday('now') - 2440587.5) * 86400000000 AS INTEGER))) ON CONFLICT(character_id)
DO UPDATE SET commands=EXCLUDED.commands,updated_at=(CAST((julianday('now') - 2440587.5) * 86400000000 AS INTEGER));

-- name: ClearEquipmentSkill :exec
DELETE FROM character_equipment_skill WHERE character_id=sqlc.arg(character_id);

-- name: EquipmentSkillSnapshots :one
SELECT s.skills,s.commands FROM characters c LEFT JOIN character_equipment_skill s ON s.character_id=c.id
WHERE c.id=sqlc.arg(character_id) AND c.account_id=sqlc.arg(account_id);

-- name: SkillLocks :many
SELECT skill_id FROM character_skill_locks WHERE character_id=sqlc.arg(character_id) ORDER BY skill_id;

-- name: ClearSkillLocks :exec
DELETE FROM character_skill_locks WHERE character_id=sqlc.arg(character_id);

-- name: AddSkillLock :exec
INSERT INTO character_skill_locks(character_id,skill_id) VALUES(sqlc.arg(character_id),sqlc.arg(skill_id));

-- name: InitializeProfileSkins :exec
INSERT INTO character_profile_skins(character_id,state) VALUES(sqlc.arg(character_id),sqlc.arg(state))
ON CONFLICT(character_id) DO NOTHING;

-- name: ProfileSkins :one
SELECT state FROM character_profile_skins WHERE character_id=sqlc.arg(character_id);

-- name: RosterBackgroundUnlocks :many
SELECT category,background_id,expires_at FROM account_roster_background_unlocks
WHERE account_id=sqlc.arg(account_id) AND (expires_at=0 OR expires_at>CAST(sqlc.arg(now_unix) AS INTEGER))
ORDER BY category,background_id;

-- name: RosterBackgrounds :many
SELECT page,category,background_id FROM account_roster_backgrounds WHERE account_id=sqlc.arg(account_id) ORDER BY page;

-- name: SelectRosterBackground :exec
INSERT INTO account_roster_backgrounds(account_id,page,category,background_id)
VALUES(sqlc.arg(account_id),sqlc.arg(page),sqlc.arg(category),sqlc.arg(background_id))
ON CONFLICT(account_id,page) DO UPDATE SET category=EXCLUDED.category,background_id=EXCLUDED.background_id,updated_at=(CAST((julianday('now') - 2440587.5) * 86400000000 AS INTEGER));

-- name: UnlockRosterBackground :execrows
-- The PostgreSQL guard (`... expires_at>0 AND expires_at<=sqlc.arg(now_unix)::bigint`)
-- lives in the DO UPDATE WHERE clause, which the SQLite engine copies verbatim
-- without binding its parameter. The guard is therefore moved into the parsed
-- SELECT of the INSERT: a row is offered for insertion only when the stored
-- unlock is absent, non-expiring (0) or already expired -- the exact complement
-- of the PostgreSQL guard -- and the conflict clause keeps the same
-- parameter-free DO UPDATE. Resulting row-counts are identical: no stored row
-- or an expired one updates (1), a still-valid one is left alone (0).
INSERT INTO account_roster_background_unlocks(account_id,category,background_id,expires_at)
SELECT sqlc.arg(account_id),sqlc.arg(category),sqlc.arg(background_id),sqlc.arg(expires_at)
WHERE NOT EXISTS(SELECT 1 FROM account_roster_background_unlocks u
 WHERE u.account_id=sqlc.arg(account_id)
 AND u.category=sqlc.arg(category)
 AND u.background_id=sqlc.arg(background_id)
 AND (u.expires_at=0 OR u.expires_at>CAST(sqlc.arg(now_unix) AS INTEGER)))
ON CONFLICT(account_id,category,background_id) DO UPDATE SET expires_at=EXCLUDED.expires_at;

-- name: SaveAccountGamepadKeys :exec
INSERT INTO account_gamepad_settings(account_id, mapping_tsv, updated_at)
VALUES(sqlc.arg(account_id), sqlc.arg(mapping_tsv), (CAST((julianday('now') - 2440587.5) * 86400000000 AS INTEGER)))
ON CONFLICT (account_id)
DO UPDATE SET mapping_tsv = EXCLUDED.mapping_tsv, updated_at = (CAST((julianday('now') - 2440587.5) * 86400000000 AS INTEGER));

-- name: SaveAccountGamepadOptions :exec
INSERT INTO account_gamepad_settings(account_id, options, updated_at)
VALUES(sqlc.arg(account_id), sqlc.arg(options), (CAST((julianday('now') - 2440587.5) * 86400000000 AS INTEGER)))
ON CONFLICT (account_id)
DO UPDATE SET options = EXCLUDED.options, updated_at = (CAST((julianday('now') - 2440587.5) * 86400000000 AS INTEGER));

-- name: SaveCharacterGamepadKeys :exec
INSERT INTO character_gamepad_settings(character_id, account_id, mapping_tsv, updated_at)
VALUES(sqlc.arg(character_id), sqlc.arg(account_id), sqlc.arg(mapping_tsv), (CAST((julianday('now') - 2440587.5) * 86400000000 AS INTEGER)))
ON CONFLICT (character_id)
DO UPDATE SET account_id = EXCLUDED.account_id, mapping_tsv = EXCLUDED.mapping_tsv, updated_at = (CAST((julianday('now') - 2440587.5) * 86400000000 AS INTEGER));

-- name: SaveCharacterGamepadOptions :exec
INSERT INTO character_gamepad_settings(character_id, account_id, options, updated_at)
VALUES(sqlc.arg(character_id), sqlc.arg(account_id), sqlc.arg(options), (CAST((julianday('now') - 2440587.5) * 86400000000 AS INTEGER)))
ON CONFLICT (character_id)
DO UPDATE SET account_id = EXCLUDED.account_id, options = EXCLUDED.options, updated_at = (CAST((julianday('now') - 2440587.5) * 86400000000 AS INTEGER));

-- name: ClearCharacterGamepadSettings :exec
DELETE FROM character_gamepad_settings WHERE character_id = sqlc.arg(character_id);

-- name: ClearAccountCharacterGamepadSettings :exec
DELETE FROM character_gamepad_settings WHERE account_id = sqlc.arg(account_id);

-- name: AccountGamepadSettings :one
SELECT mapping_tsv,options FROM account_gamepad_settings WHERE account_id=sqlc.arg(account_id);

-- name: CharacterGamepadSettings :one
SELECT mapping_tsv,options FROM character_gamepad_settings WHERE character_id=sqlc.arg(character_id);

-- name: SelectSkin :exec
INSERT INTO character_skin_selection(character_id,page,skin_key)
VALUES(sqlc.arg(character_id),sqlc.arg(page),sqlc.arg(skin_key))
ON CONFLICT(character_id,page) DO UPDATE SET skin_key=EXCLUDED.skin_key,updated_at=(CAST((julianday('now') - 2440587.5) * 86400000000 AS INTEGER));

-- name: SelectedSkin :one
SELECT skin_key FROM character_skin_selection
WHERE character_id=sqlc.arg(character_id) AND page=sqlc.arg(page);

-- name: ClearSkinSelectionList :exec
DELETE FROM character_skin_selection_list WHERE character_id=sqlc.arg(character_id) AND category=sqlc.arg(category);

-- name: AddSkinSelectionList :exec
INSERT INTO character_skin_selection_list(character_id,category,skin_key) VALUES(sqlc.arg(character_id),sqlc.arg(category),sqlc.arg(skin_key)) ON CONFLICT DO NOTHING;

-- name: SkinSelectionList :many
SELECT skin_key FROM character_skin_selection_list WHERE character_id=sqlc.arg(character_id) AND category=sqlc.arg(category) ORDER BY skin_key;

-- name: ClearSkinSelectionSlots :exec
DELETE FROM character_skin_selection_slot WHERE character_id=sqlc.arg(character_id) AND category=sqlc.arg(category);

-- name: AddSkinSelectionSlot :exec
INSERT INTO character_skin_selection_slot(character_id,category,slot,skin_key) VALUES(sqlc.arg(character_id),sqlc.arg(category),sqlc.arg(slot),sqlc.arg(skin_key));

-- name: SkinSelectionSlots :many
SELECT slot,skin_key FROM character_skin_selection_slot WHERE character_id=sqlc.arg(character_id) AND category=sqlc.arg(category);

-- name: RemoveSkinFavorite :exec
DELETE FROM character_skin_favorite WHERE character_id=sqlc.arg(character_id) AND page=sqlc.arg(page) AND skin_key=sqlc.arg(skin_key);

-- name: CountSkinFavorites :one
SELECT count(*) FROM character_skin_favorite WHERE character_id=sqlc.arg(character_id) AND page=sqlc.arg(page);

-- name: AddSkinFavorite :exec
INSERT INTO character_skin_favorite(character_id,page,skin_key) VALUES(sqlc.arg(character_id),sqlc.arg(page),sqlc.arg(skin_key)) ON CONFLICT DO NOTHING;

-- name: SkinFavorites :many
SELECT page,skin_key FROM character_skin_favorite WHERE character_id=sqlc.arg(character_id) AND page<CAST(sqlc.arg(groups) AS INTEGER) ORDER BY page,skin_key;

-- name: UnlockSkin :exec
INSERT INTO account_skin_cargo(account_id,source_template,skin_key) VALUES(sqlc.arg(account_id),sqlc.arg(source_template),sqlc.arg(skin_key)) ON CONFLICT(account_id,source_template) DO NOTHING;

-- name: ListSkins :many
SELECT source_template,skin_key,unlocked_at FROM account_skin_cargo WHERE account_id=sqlc.arg(account_id) AND skin_key<>0 ORDER BY source_template;

-- name: SaveAccountUnifiedOption :exec
INSERT INTO account_unified_options(account_id,opt_index,value) VALUES(sqlc.arg(account_id),sqlc.arg(opt_index),sqlc.arg(value))
 ON CONFLICT(account_id,opt_index) DO UPDATE SET value=EXCLUDED.value,updated_at=(CAST((julianday('now') - 2440587.5) * 86400000000 AS INTEGER));

-- name: SaveCharacterUnifiedOption :exec
INSERT INTO character_unified_options(character_id,opt_index,value) VALUES(sqlc.arg(character_id),sqlc.arg(opt_index),sqlc.arg(value))
 ON CONFLICT(character_id,opt_index) DO UPDATE SET value=EXCLUDED.value,updated_at=(CAST((julianday('now') - 2440587.5) * 86400000000 AS INTEGER));

-- name: SaveCharacterUnifiedOptionGroup :exec
INSERT INTO character_unified_option_groups(character_id,subtype,opt_index,value) VALUES(sqlc.arg(character_id),sqlc.arg(subtype),sqlc.arg(opt_index),sqlc.arg(value))
ON CONFLICT(character_id,subtype,opt_index) DO UPDATE SET value=EXCLUDED.value,updated_at=(CAST((julianday('now') - 2440587.5) * 86400000000 AS INTEGER));

-- name: SaveAccountHotkey :exec
INSERT INTO account_hotkeys(account_id, subtype, slot_index, keycode)
VALUES(sqlc.arg(account_id), sqlc.arg(subtype), sqlc.arg(slot_index), sqlc.arg(keycode))
ON CONFLICT(account_id, subtype, slot_index) DO UPDATE SET keycode=EXCLUDED.keycode, updated_at=(CAST((julianday('now') - 2440587.5) * 86400000000 AS INTEGER));

-- name: SaveCharacterHotkey :exec
INSERT INTO character_hotkeys(character_id, subtype, slot_index, keycode)
VALUES(sqlc.arg(character_id), sqlc.arg(subtype), sqlc.arg(slot_index), sqlc.arg(keycode))
ON CONFLICT(character_id, subtype, slot_index) DO UPDATE SET keycode=EXCLUDED.keycode, updated_at=(CAST((julianday('now') - 2440587.5) * 86400000000 AS INTEGER));

-- name: PromoteCharacterHotkeysToAccount :exec
INSERT INTO account_hotkeys (account_id, subtype, slot_index, keycode, updated_at)
SELECT CAST(sqlc.arg(account_id) AS INTEGER), h.subtype, h.slot_index, h.keycode, (CAST((julianday('now') - 2440587.5) * 86400000000 AS INTEGER))
FROM character_hotkeys h
WHERE h.character_id = sqlc.arg(character_id) AND h.subtype = sqlc.arg(subtype)
ON CONFLICT (account_id, subtype, slot_index)
DO UPDATE SET keycode = EXCLUDED.keycode, updated_at = (CAST((julianday('now') - 2440587.5) * 86400000000 AS INTEGER));

-- name: ClearAccountCharacterHotkeys :exec
DELETE FROM character_hotkeys
WHERE character_id IN (SELECT id FROM characters WHERE account_id = sqlc.arg(account_id))
  AND subtype = sqlc.arg(subtype);

-- name: CopyAccountHotkeysToCharacter :exec
INSERT INTO character_hotkeys (character_id, subtype, slot_index, keycode, updated_at)
SELECT CAST(sqlc.arg(character_id) AS INTEGER), h.subtype, h.slot_index, h.keycode, (CAST((julianday('now') - 2440587.5) * 86400000000 AS INTEGER))
FROM account_hotkeys h
WHERE h.account_id = sqlc.arg(account_id) AND h.subtype = sqlc.arg(subtype)
ON CONFLICT (character_id, subtype, slot_index) DO NOTHING;

-- name: AccountUnifiedOptions :many
SELECT opt_index,value FROM account_unified_options WHERE account_id=sqlc.arg(account_id) AND value <> 65535;

-- name: CharacterUnifiedOptions :many
SELECT opt_index,value FROM character_unified_options WHERE character_id=sqlc.arg(character_id);

-- name: CharacterUnifiedOptionGroup :many
SELECT opt_index,value FROM character_unified_option_groups WHERE character_id=sqlc.arg(character_id) AND subtype=sqlc.arg(subtype);

-- name: AccountHotkeys :many
SELECT slot_index, keycode FROM account_hotkeys WHERE account_id=sqlc.arg(account_id) AND subtype=sqlc.arg(subtype);

-- name: CharacterHotkeys :many
SELECT slot_index, keycode FROM character_hotkeys WHERE character_id=sqlc.arg(character_id) AND subtype=sqlc.arg(subtype);
