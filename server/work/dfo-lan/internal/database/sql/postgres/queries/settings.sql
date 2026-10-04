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
INSERT INTO character_tutorial_flags(character_id,flag,completed)
SELECT id,sqlc.arg(flag)::integer,sqlc.arg(completed)::boolean FROM characters
WHERE account_id=sqlc.arg(account_id) AND id=sqlc.arg(character_id)
ON CONFLICT(character_id,flag) DO UPDATE SET completed=EXCLUDED.completed,updated_at=now();

-- name: TutorialFlags :many
SELECT flag FROM character_tutorial_flags WHERE character_id=sqlc.arg(character_id) AND completed ORDER BY flag;

-- name: SaveAccountWarpFavorite :exec
INSERT INTO account_warp_favorites(account_id,slot_index,value)
VALUES(sqlc.arg(account_id),sqlc.arg(slot_index),sqlc.arg(value))
ON CONFLICT(account_id,slot_index) DO UPDATE SET value=EXCLUDED.value,updated_at=now()
WHERE account_warp_favorites.value IS DISTINCT FROM EXCLUDED.value;

-- name: AccountWarpFavorites :many
SELECT slot_index,value FROM account_warp_favorites WHERE account_id=sqlc.arg(account_id) ORDER BY slot_index;

-- name: SaveEquipmentSkills :exec
INSERT INTO character_equipment_skill(character_id,skills,updated_at)
VALUES(sqlc.arg(character_id),sqlc.arg(skills),now()) ON CONFLICT(character_id)
DO UPDATE SET skills=EXCLUDED.skills,updated_at=now();

-- name: SaveEquipmentCommands :exec
INSERT INTO character_equipment_skill(character_id,commands,updated_at)
VALUES(sqlc.arg(character_id),sqlc.arg(commands),now()) ON CONFLICT(character_id)
DO UPDATE SET commands=EXCLUDED.commands,updated_at=now();

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
WHERE account_id=sqlc.arg(account_id) AND (expires_at=0 OR expires_at>sqlc.arg(now_unix)::bigint)
ORDER BY category,background_id;

-- name: RosterBackgrounds :many
SELECT page,category,background_id FROM account_roster_backgrounds WHERE account_id=sqlc.arg(account_id) ORDER BY page;

-- name: SelectRosterBackground :exec
INSERT INTO account_roster_backgrounds(account_id,page,category,background_id)
VALUES(sqlc.arg(account_id),sqlc.arg(page),sqlc.arg(category),sqlc.arg(background_id))
ON CONFLICT(account_id,page) DO UPDATE SET category=EXCLUDED.category,background_id=EXCLUDED.background_id,updated_at=now();

-- name: UnlockRosterBackground :execrows
INSERT INTO account_roster_background_unlocks(account_id,category,background_id,expires_at)
VALUES(sqlc.arg(account_id),sqlc.arg(category),sqlc.arg(background_id),sqlc.arg(expires_at))
ON CONFLICT(account_id,category,background_id) DO UPDATE SET expires_at=EXCLUDED.expires_at
WHERE account_roster_background_unlocks.expires_at>0 AND account_roster_background_unlocks.expires_at<=sqlc.arg(now_unix)::bigint;

-- name: SaveAccountGamepadKeys :exec
INSERT INTO account_gamepad_settings(account_id, mapping_tsv, updated_at)
VALUES(sqlc.arg(account_id), sqlc.arg(mapping_tsv), now())
ON CONFLICT (account_id)
DO UPDATE SET mapping_tsv = EXCLUDED.mapping_tsv, updated_at = now();

-- name: SaveAccountGamepadOptions :exec
INSERT INTO account_gamepad_settings(account_id, options, updated_at)
VALUES(sqlc.arg(account_id), sqlc.arg(options), now())
ON CONFLICT (account_id)
DO UPDATE SET options = EXCLUDED.options, updated_at = now();

-- name: SaveCharacterGamepadKeys :exec
INSERT INTO character_gamepad_settings(character_id, account_id, mapping_tsv, updated_at)
VALUES(sqlc.arg(character_id), sqlc.arg(account_id), sqlc.arg(mapping_tsv), now())
ON CONFLICT (character_id)
DO UPDATE SET account_id = EXCLUDED.account_id, mapping_tsv = EXCLUDED.mapping_tsv, updated_at = now();

-- name: SaveCharacterGamepadOptions :exec
INSERT INTO character_gamepad_settings(character_id, account_id, options, updated_at)
VALUES(sqlc.arg(character_id), sqlc.arg(account_id), sqlc.arg(options), now())
ON CONFLICT (character_id)
DO UPDATE SET account_id = EXCLUDED.account_id, options = EXCLUDED.options, updated_at = now();

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
ON CONFLICT(character_id,page) DO UPDATE SET skin_key=EXCLUDED.skin_key,updated_at=now();

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
SELECT page,skin_key FROM character_skin_favorite WHERE character_id=sqlc.arg(character_id) AND page<sqlc.arg(groups)::bigint ORDER BY page,skin_key;

-- name: UnlockSkin :exec
INSERT INTO account_skin_cargo(account_id,source_template,skin_key) VALUES(sqlc.arg(account_id),sqlc.arg(source_template),sqlc.arg(skin_key)) ON CONFLICT(account_id,source_template) DO NOTHING;

-- name: ListSkins :many
SELECT source_template,skin_key,unlocked_at FROM account_skin_cargo WHERE account_id=sqlc.arg(account_id) AND skin_key<>0 ORDER BY source_template;

-- name: SaveAccountUnifiedOption :exec
INSERT INTO account_unified_options(account_id,opt_index,value) VALUES(sqlc.arg(account_id),sqlc.arg(opt_index),sqlc.arg(value))
 ON CONFLICT(account_id,opt_index) DO UPDATE SET value=EXCLUDED.value,updated_at=now();

-- name: SaveCharacterUnifiedOption :exec
INSERT INTO character_unified_options(character_id,opt_index,value) VALUES(sqlc.arg(character_id),sqlc.arg(opt_index),sqlc.arg(value))
 ON CONFLICT(character_id,opt_index) DO UPDATE SET value=EXCLUDED.value,updated_at=now();

-- name: SaveCharacterUnifiedOptionGroup :exec
INSERT INTO character_unified_option_groups(character_id,subtype,opt_index,value) VALUES(sqlc.arg(character_id),sqlc.arg(subtype),sqlc.arg(opt_index),sqlc.arg(value))
ON CONFLICT(character_id,subtype,opt_index) DO UPDATE SET value=EXCLUDED.value,updated_at=now();

-- name: SaveAccountHotkey :exec
INSERT INTO account_hotkeys(account_id, subtype, slot_index, keycode)
VALUES(sqlc.arg(account_id), sqlc.arg(subtype), sqlc.arg(slot_index), sqlc.arg(keycode))
ON CONFLICT(account_id, subtype, slot_index) DO UPDATE SET keycode=EXCLUDED.keycode, updated_at=now();

-- name: SaveCharacterHotkey :exec
INSERT INTO character_hotkeys(character_id, subtype, slot_index, keycode)
VALUES(sqlc.arg(character_id), sqlc.arg(subtype), sqlc.arg(slot_index), sqlc.arg(keycode))
ON CONFLICT(character_id, subtype, slot_index) DO UPDATE SET keycode=EXCLUDED.keycode, updated_at=now();

-- name: PromoteCharacterHotkeysToAccount :exec
INSERT INTO account_hotkeys (account_id, subtype, slot_index, keycode, updated_at)
SELECT sqlc.arg(account_id)::bigint, h.subtype, h.slot_index, h.keycode, now()
FROM character_hotkeys h
WHERE h.character_id = sqlc.arg(character_id) AND h.subtype = sqlc.arg(subtype)
ON CONFLICT (account_id, subtype, slot_index)
DO UPDATE SET keycode = EXCLUDED.keycode, updated_at = now();

-- name: ClearAccountCharacterHotkeys :exec
DELETE FROM character_hotkeys
WHERE character_id IN (SELECT id FROM characters WHERE account_id = sqlc.arg(account_id))
  AND subtype = sqlc.arg(subtype);

-- name: CopyAccountHotkeysToCharacter :exec
INSERT INTO character_hotkeys (character_id, subtype, slot_index, keycode, updated_at)
SELECT sqlc.arg(character_id)::bigint, h.subtype, h.slot_index, h.keycode, now()
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
