-- SQLite schema fork (dual-engine, S3/S4). See docs/sqlite-dual-engine-design.md.
--
-- This file is the SQLite mirror of the PostgreSQL schema. The PostgreSQL file
-- (sql/postgres/migrations/0001_initial.sql) stays untouched and remains the
-- source of truth for the PG path. Section names are kept identical so the two
-- migration ledgers stay comparable even where a section carries no statement.
--
-- Dialect decisions applied here (design doc D22-D29):
--   * identity      -> INTEGER PRIMARY KEY AUTOINCREMENT. Monotonic and never
--                      reused, which is the property the save format relies on.
--                      This also replaces the one sequence in the schema
--                      (mailbox_id_seq), whose only use was this default.
--   * jsonb         -> BLOB + CHECK(json_valid(...)); deliberately NOT named
--                      "jsonb", which SQLite >= 3.45 treats as a keyword with
--                      different semantics. BLOB rather than TEXT, because the Go
--                      type is json.RawMessage: database/sql cannot scan a TEXT
--                      string into json.RawMessage, while a []byte round-trips
--                      byte-exactly in both directions with no adapter
--                      conversion (D24).
--   * jsonb_typeof  -> json_type; jsonb_set -> json_set.
--   * bytea         -> BLOB.
--   * bigint[]      -> BLOB holding a JSON array, with the element-count check
--                      spelled as json_array_length(...) (D27). SQLite has no
--                      array type, and the Go side already owns the encoding.
--   * boolean       -> BOOLEAN storing 0/1 (SQLite has no boolean type); a
--                      DEFAULT true/false becomes 1/0.
--   * timestamptz   -> TIMESTAMP holding INTEGER microseconds since the Unix
--                      epoch, matching PG's microsecond precision exactly. The
--                      driver writes/reads time.Time against this via
--                      _inttotime=1 + _time_integer_format=unix_micro, and the
--                      DEFAULT expression below produces the same representation
--                      so SQL-side defaults and Go-side writes stay comparable.
--   * date          -> DATE with the same integer-microsecond representation.
--                      PG truncates a date to midnight; SQLite does not, so the
--                      adapter must pass UTC-midnight values for DATE columns
--                      (D29). This is the one place where a byte-compatible
--                      schema still needs a caller-side invariant.
--   * octet_length  -> length(CAST(x AS BLOB)). SQLite length() on TEXT counts
--                      characters, so the cast keeps PG's byte semantics.
--   * FOR UPDATE / FOR NO KEY UPDATE / FOR SHARE -> removed (queries only); the
--                      engine serializes writers with BEGIN IMMEDIATE.
--   * ALTER TABLE ... ADD COLUMN IF NOT EXISTS (unsupported in SQLite) -> folded
--                      into CREATE TABLE.
--   * CHECK(...) with %/* arithmetic, IN lists, BETWEEN and named table
--                      constraints -> kept as written.
--
--   * integer width -> BIGINT / INTEGER / SMALLINT mirror the PostgreSQL
--                      declaration. SQLite stores all three identically (INTEGER
--                      affinity), so this changes nothing at runtime; it exists so
--                      sqlc can map each column back to the Go width PostgreSQL
--                      produces (smallint->int16, integer->int32, bigint->int64)
--                      through the db_type overrides in sqlc.yaml. Without it the
--                      two generated packages could not share one query interface
--                      (design doc D30). Identity columns must stay INTEGER,
--                      because SQLite only treats a column declared exactly
--                      INTEGER PRIMARY KEY as a rowid alias, so they are pinned
--                      back to int64 by a column override.
-- Deliberately NOT carried over, with reasons:
--   * DO $$ ... $$ blocks (0015, 0028) and the information_schema/to_regclass
--     probes inside them: they inspect PostgreSQL catalogs for dropped legacy
--     tables. A fresh SQLite database has no legacy state to repair.
--   * Historical repair DML (0023 quest model translation, 0028 vault upgrade,
--     0035 grief seeding): this fork expresses the CURRENT schema. Data reaches
--     SQLite through the S4 converter, which copies post-migration state, so
--     replaying repairs would be redundant at best and destructive at worst.
--   * PostgreSQL-only functions used only by that DML (to_jsonb, jsonb_set).

-- migration: 0000_migration_ledger.sql
-- Infrastructure ledger; mirrors the PostgreSQL section.
CREATE TABLE IF NOT EXISTS storage_migrations (
 name TEXT PRIMARY KEY,
 checksum TEXT NOT NULL CHECK(length(checksum)=64),
 applied_at TIMESTAMP NOT NULL DEFAULT (CAST((julianday('now') - 2440587.5) * 86400000000 AS INTEGER)),
 last_applied_at TIMESTAMP NOT NULL DEFAULT (CAST((julianday('now') - 2440587.5) * 86400000000 AS INTEGER)),
 runs BIGINT NOT NULL DEFAULT 1 CHECK(runs>0)
);

-- end migration

-- migration: 0001_core.sql
-- The four additive ALTER TABLE statements on the PostgreSQL side (deleted_at,
-- max_fame, roster_order, fixed_slot) are folded into CREATE TABLE here.
CREATE TABLE IF NOT EXISTS accounts (
 id INTEGER PRIMARY KEY AUTOINCREMENT,
 username TEXT NOT NULL UNIQUE,
 password_hash TEXT,
 development_only BOOLEAN NOT NULL DEFAULT 1,
 created_at TIMESTAMP NOT NULL DEFAULT (CAST((julianday('now') - 2440587.5) * 86400000000 AS INTEGER)),
 CHECK(development_only <> 0 OR password_hash IS NOT NULL)
);

CREATE TABLE IF NOT EXISTS characters (
 id INTEGER PRIMARY KEY AUTOINCREMENT,
 account_id BIGINT NOT NULL REFERENCES accounts(id),
 wire_id INTEGER NOT NULL CHECK(wire_id BETWEEN 1 AND 65534),
 name TEXT NOT NULL,
 profession INTEGER NOT NULL CHECK(profession BETWEEN 0 AND 255),
 create_request BLOB NOT NULL,
 config_version TEXT NOT NULL,
 state BLOB NOT NULL CHECK(json_valid(state)),
 created_at TIMESTAMP NOT NULL DEFAULT (CAST((julianday('now') - 2440587.5) * 86400000000 AS INTEGER)),
 deleted_at TIMESTAMP,
 max_fame INTEGER NOT NULL DEFAULT 0 CHECK(max_fame >= 0),
 roster_order BIGINT CHECK(roster_order > 0),
 fixed_slot SMALLINT NOT NULL DEFAULT 0 CHECK(fixed_slot BETWEEN 0 AND 255),
 UNIQUE(account_id,wire_id)
);

CREATE UNIQUE INDEX IF NOT EXISTS characters_name_unique ON characters(lower(name));

CREATE TABLE IF NOT EXISTS npc_favor (
 character_id BIGINT NOT NULL REFERENCES characters(id),
 npc_id BIGINT NOT NULL,
 point BIGINT NOT NULL DEFAULT 0,
 daily_count INTEGER NOT NULL DEFAULT 0,
 last_gift_day DATE,
 updated_at TIMESTAMP NOT NULL DEFAULT (CAST((julianday('now') - 2440587.5) * 86400000000 AS INTEGER)),
 PRIMARY KEY(character_id,npc_id));

-- end migration

-- migration: 0002_character_events.sql
-- Keeps the composite primary key, which is also the idempotency key for every
-- CommitCharacterEvent replay.
CREATE TABLE IF NOT EXISTS character_events (
 character_id BIGINT NOT NULL REFERENCES characters(id),
 event_key TEXT NOT NULL,
 config_version TEXT NOT NULL,
 model TEXT NOT NULL,
 outcome BLOB NOT NULL CHECK(json_valid(outcome)),
 created_at TIMESTAMP NOT NULL DEFAULT (CAST((julianday('now') - 2440587.5) * 86400000000 AS INTEGER)),
 PRIMARY KEY(character_id,event_key)
);

-- end migration

-- migration: 0003_character_notices.sql
CREATE TABLE IF NOT EXISTS character_notice_seen (
 account_id BIGINT NOT NULL REFERENCES accounts(id),
 character_id BIGINT NOT NULL,
 tree SMALLINT NOT NULL,
 notice_id SMALLINT NOT NULL,
 updated_at TIMESTAMP NOT NULL DEFAULT (CAST((julianday('now') - 2440587.5) * 86400000000 AS INTEGER)),
 PRIMARY KEY(account_id, character_id, tree, notice_id));

-- end migration

-- migration: 0004_tutorial.sql
CREATE TABLE IF NOT EXISTS character_tutorial_flags(
 character_id BIGINT NOT NULL REFERENCES characters(id),
 flag INTEGER NOT NULL CHECK(flag BETWEEN 0 AND 100),
 completed BOOLEAN NOT NULL,
 updated_at TIMESTAMP NOT NULL DEFAULT (CAST((julianday('now') - 2440587.5) * 86400000000 AS INTEGER)),
 PRIMARY KEY(character_id,flag));

-- end migration

-- migration: 0005_warp_favorites.sql
CREATE TABLE IF NOT EXISTS account_warp_favorites (
 account_id BIGINT NOT NULL REFERENCES accounts(id),
 slot_index SMALLINT NOT NULL CHECK(slot_index BETWEEN 0 AND 9),
 value BLOB NOT NULL CHECK(length(value) = 12),
 updated_at TIMESTAMP NOT NULL DEFAULT (CAST((julianday('now') - 2440587.5) * 86400000000 AS INTEGER)),
 PRIMARY KEY(account_id,slot_index));

-- end migration

-- migration: 0006_adventure.sql
-- The additive `data` column from the PostgreSQL side is folded in.
CREATE TABLE IF NOT EXISTS account_adventures (
 account_id BIGINT PRIMARY KEY REFERENCES accounts(id),
 name TEXT NOT NULL CHECK(length(name)>0),
 level INTEGER NOT NULL DEFAULT 1 CHECK(level BETWEEN 1 AND 50),
 experience BIGINT NOT NULL DEFAULT 0 CHECK(experience>=0),
 data BLOB NOT NULL DEFAULT (CAST('{}' AS BLOB)) CHECK(json_valid(data)),
 created_at TIMESTAMP NOT NULL DEFAULT (CAST((julianday('now') - 2440587.5) * 86400000000 AS INTEGER)));

-- end migration

-- migration: 0007_equipment_skill.sql
CREATE TABLE IF NOT EXISTS character_equipment_skill (
 character_id BIGINT PRIMARY KEY REFERENCES characters(id),
 skills BLOB,
 commands BLOB,
 updated_at TIMESTAMP NOT NULL DEFAULT (CAST((julianday('now') - 2440587.5) * 86400000000 AS INTEGER)));

-- end migration

-- migration: 0008_skill_locks.sql
CREATE TABLE IF NOT EXISTS character_skill_locks (
 character_id BIGINT NOT NULL REFERENCES characters(id),
 skill_id INTEGER NOT NULL CHECK(skill_id BETWEEN 1 AND 1023),
 PRIMARY KEY(character_id, skill_id));

-- end migration

-- migration: 0009_profile_skins.sql
CREATE TABLE IF NOT EXISTS character_profile_skins (
 character_id BIGINT PRIMARY KEY REFERENCES characters(id),
 state BLOB NOT NULL CHECK(json_valid(state)),
 updated_at TIMESTAMP NOT NULL DEFAULT (CAST((julianday('now') - 2440587.5) * 86400000000 AS INTEGER)));

-- end migration

-- migration: 0010_gamepad.sql
CREATE TABLE IF NOT EXISTS account_gamepad_settings (
    account_id BIGINT NOT NULL REFERENCES accounts(id) PRIMARY KEY,
    mapping_tsv BLOB NOT NULL DEFAULT (x''),
    options BLOB NOT NULL DEFAULT (x''),
    updated_at TIMESTAMP NOT NULL DEFAULT (CAST((julianday('now') - 2440587.5) * 86400000000 AS INTEGER))
);
CREATE TABLE IF NOT EXISTS character_gamepad_settings (
    character_id BIGINT NOT NULL REFERENCES characters(id) PRIMARY KEY,
    account_id BIGINT NOT NULL REFERENCES accounts(id),
    mapping_tsv BLOB NOT NULL DEFAULT (x''),
    options BLOB NOT NULL DEFAULT (x''),
    updated_at TIMESTAMP NOT NULL DEFAULT (CAST((julianday('now') - 2440587.5) * 86400000000 AS INTEGER))
);

-- end migration

-- migration: 0011_shop_purchases.sql
CREATE TABLE IF NOT EXISTS character_shop_purchases (
 character_id BIGINT NOT NULL REFERENCES characters(id),
 account_id BIGINT NOT NULL,
 npc_id INTEGER NOT NULL,
 template INTEGER NOT NULL,
 bought_at TIMESTAMP NOT NULL DEFAULT (CAST((julianday('now') - 2440587.5) * 86400000000 AS INTEGER)));
CREATE INDEX IF NOT EXISTS character_shop_purchases_window
 ON character_shop_purchases(character_id, npc_id, template, bought_at);

-- end migration

-- migration: 0012_roster_backgrounds.sql
-- The additive expires_at column from the PostgreSQL side is folded in.
CREATE TABLE IF NOT EXISTS account_roster_backgrounds (
 account_id BIGINT NOT NULL REFERENCES accounts(id),
 page SMALLINT NOT NULL CHECK(page BETWEEN 0 AND 4),
 category SMALLINT NOT NULL CHECK(category BETWEEN 0 AND 1),
 background_id INTEGER NOT NULL CHECK(background_id BETWEEN 0 AND 65535),
 updated_at TIMESTAMP NOT NULL DEFAULT (CAST((julianday('now') - 2440587.5) * 86400000000 AS INTEGER)),
 PRIMARY KEY(account_id,page));
CREATE TABLE IF NOT EXISTS account_roster_background_unlocks (
 account_id BIGINT NOT NULL REFERENCES accounts(id),
 category SMALLINT NOT NULL CHECK(category=1),
 background_id INTEGER NOT NULL CHECK(background_id BETWEEN 0 AND 65535),
 expires_at BIGINT NOT NULL DEFAULT 0 CHECK(expires_at BETWEEN 0 AND 2147483647),
 created_at TIMESTAMP NOT NULL DEFAULT (CAST((julianday('now') - 2440587.5) * 86400000000 AS INTEGER)),
 PRIMARY KEY(account_id,category,background_id));

-- end migration

-- migration: 0013_skin_selection.sql
CREATE TABLE IF NOT EXISTS character_skin_selection (
 character_id BIGINT NOT NULL REFERENCES characters(id),
 page BIGINT NOT NULL,
 skin_key BIGINT NOT NULL,
 updated_at TIMESTAMP NOT NULL DEFAULT (CAST((julianday('now') - 2440587.5) * 86400000000 AS INTEGER)),
 PRIMARY KEY(character_id,page));

-- end migration

-- migration: 0014_skin_lists.sql
CREATE TABLE IF NOT EXISTS character_skin_selection_list(
 character_id BIGINT NOT NULL REFERENCES characters(id),
 category BIGINT NOT NULL,
 skin_key BIGINT NOT NULL,
 updated_at TIMESTAMP NOT NULL DEFAULT (CAST((julianday('now') - 2440587.5) * 86400000000 AS INTEGER)),
 PRIMARY KEY(character_id,category,skin_key));
CREATE TABLE IF NOT EXISTS character_skin_selection_slot(
 character_id BIGINT NOT NULL REFERENCES characters(id),
 category BIGINT NOT NULL,
 slot BIGINT NOT NULL,
 skin_key BIGINT NOT NULL,
 updated_at TIMESTAMP NOT NULL DEFAULT (CAST((julianday('now') - 2440587.5) * 86400000000 AS INTEGER)),
 PRIMARY KEY(character_id,category,slot));
CREATE TABLE IF NOT EXISTS character_skin_favorite(
 character_id BIGINT NOT NULL REFERENCES characters(id),
 page BIGINT NOT NULL,
 skin_key BIGINT NOT NULL,
 updated_at TIMESTAMP NOT NULL DEFAULT (CAST((julianday('now') - 2440587.5) * 86400000000 AS INTEGER)),
 PRIMARY KEY(character_id,page,skin_key));

-- end migration

-- migration: 0015_skin_cargo.sql
-- The additive skin_key column is folded in. The PostgreSQL section then runs a
-- DO block that inspects information_schema for a dropped legacy column
-- (action_param) and copies it across; a fresh SQLite database has no such
-- legacy state, so the block is not carried over.
CREATE TABLE IF NOT EXISTS account_skin_cargo (
 account_id BIGINT NOT NULL REFERENCES accounts(id),
 source_template BIGINT NOT NULL,
 skin_key BIGINT NOT NULL DEFAULT 0,
 unlocked_at TIMESTAMP NOT NULL DEFAULT (CAST((julianday('now') - 2440587.5) * 86400000000 AS INTEGER)),
 PRIMARY KEY(account_id, source_template));

-- end migration

-- migration: 0016_unified_options.sql
CREATE TABLE IF NOT EXISTS account_unified_options(
 account_id BIGINT NOT NULL REFERENCES accounts(id),
 opt_index INTEGER NOT NULL CHECK(opt_index BETWEEN 0 AND 285),
 value INTEGER NOT NULL,
 updated_at TIMESTAMP NOT NULL DEFAULT (CAST((julianday('now') - 2440587.5) * 86400000000 AS INTEGER)),
 PRIMARY KEY(account_id,opt_index));
CREATE TABLE IF NOT EXISTS character_unified_options(
 character_id BIGINT NOT NULL REFERENCES characters(id),
 opt_index INTEGER NOT NULL CHECK(opt_index BETWEEN 0 AND 285),
 value INTEGER NOT NULL,
 updated_at TIMESTAMP NOT NULL DEFAULT (CAST((julianday('now') - 2440587.5) * 86400000000 AS INTEGER)),
 PRIMARY KEY(character_id,opt_index));
CREATE TABLE IF NOT EXISTS account_hotkeys(
 account_id BIGINT NOT NULL REFERENCES accounts(id),
 subtype SMALLINT NOT NULL CHECK(subtype IN (3, 4)),
 slot_index SMALLINT NOT NULL CHECK(slot_index BETWEEN 0 AND 156),
 keycode INTEGER NOT NULL,
 updated_at TIMESTAMP NOT NULL DEFAULT (CAST((julianday('now') - 2440587.5) * 86400000000 AS INTEGER)),
 PRIMARY KEY(account_id,subtype,slot_index));
CREATE TABLE IF NOT EXISTS character_hotkeys(
 character_id BIGINT NOT NULL REFERENCES characters(id),
 subtype SMALLINT NOT NULL CHECK(subtype IN (3, 4)),
 slot_index SMALLINT NOT NULL CHECK(slot_index BETWEEN 0 AND 156),
 keycode INTEGER NOT NULL,
 updated_at TIMESTAMP NOT NULL DEFAULT (CAST((julianday('now') - 2440587.5) * 86400000000 AS INTEGER)),
 PRIMARY KEY(character_id,subtype,slot_index));
CREATE TABLE IF NOT EXISTS character_unified_option_groups(
 character_id BIGINT NOT NULL REFERENCES characters(id),
 subtype SMALLINT NOT NULL CHECK(subtype = 18),
 opt_index INTEGER NOT NULL CHECK(opt_index BETWEEN 0 AND 5),
 value INTEGER NOT NULL CHECK(value BETWEEN 0 AND 65535),
 updated_at TIMESTAMP NOT NULL DEFAULT (CAST((julianday('now') - 2440587.5) * 86400000000 AS INTEGER)),
 PRIMARY KEY(character_id,subtype,opt_index));

-- end migration

-- migration: 0017_account_materials.sql
CREATE TABLE IF NOT EXISTS account_material_storage (
 account_id BIGINT PRIMARY KEY REFERENCES accounts(id),
 counts BLOB NOT NULL DEFAULT (CAST('{}' AS BLOB)) CHECK(json_valid(counts)),
 updated_at TIMESTAMP NOT NULL DEFAULT (CAST((julianday('now') - 2440587.5) * 86400000000 AS INTEGER)));

-- end migration

-- migration: 0018_grants.sql
CREATE TABLE IF NOT EXISTS account_currency(
 account_id BIGINT PRIMARY KEY REFERENCES accounts(id),
 cera BIGINT NOT NULL DEFAULT 0 CHECK(cera>=0),
 updated_at TIMESTAMP NOT NULL DEFAULT (CAST((julianday('now') - 2440587.5) * 86400000000 AS INTEGER)));
CREATE TABLE IF NOT EXISTS admin_grants(
 grant_id TEXT PRIMARY KEY,
 account_id BIGINT NOT NULL REFERENCES accounts(id),
 character_id BIGINT REFERENCES characters(id),
 request BLOB NOT NULL CHECK(json_valid(request)),
 receipt BLOB NOT NULL CHECK(json_valid(receipt)),
 operator TEXT NOT NULL, reason TEXT NOT NULL,
 created_at TIMESTAMP NOT NULL DEFAULT (CAST((julianday('now') - 2440587.5) * 86400000000 AS INTEGER)));

-- end migration

-- migration: 0019_world.sql
CREATE TABLE IF NOT EXISTS character_world (
 character_id BIGINT PRIMARY KEY REFERENCES characters(id),
 position BLOB NOT NULL CHECK(json_valid(position)), config_version TEXT NOT NULL,
 revision BIGINT NOT NULL DEFAULT 1 CHECK(revision>0),
 updated_at TIMESTAMP NOT NULL DEFAULT (CAST((julianday('now') - 2440587.5) * 86400000000 AS INTEGER)));

CREATE TABLE IF NOT EXISTS character_channel_world (
 character_id BIGINT NOT NULL REFERENCES characters(id),
 channel_type INTEGER NOT NULL CHECK(channel_type>0 AND channel_type<=255),
 position BLOB NOT NULL CHECK(json_valid(position)), config_version TEXT NOT NULL,
 revision BIGINT NOT NULL DEFAULT 1 CHECK(revision>0),
 updated_at TIMESTAMP NOT NULL DEFAULT (CAST((julianday('now') - 2440587.5) * 86400000000 AS INTEGER)),
 PRIMARY KEY(character_id, channel_type));

-- end migration

-- migration: 0020_birth.sql
CREATE TABLE IF NOT EXISTS character_birth(
 character_id BIGINT PRIMARY KEY REFERENCES characters(id),
 stage SMALLINT NOT NULL CHECK(stage BETWEEN 0 AND 3),
 dungeon BIGINT NOT NULL DEFAULT 0 CHECK(dungeon>=0),
 updated_at TIMESTAMP NOT NULL DEFAULT (CAST((julianday('now') - 2440587.5) * 86400000000 AS INTEGER)));

-- end migration

-- migration: 0021_fatigue.sql
CREATE TABLE IF NOT EXISTS character_fatigue (
 character_id BIGINT PRIMARY KEY REFERENCES characters(id),
 day DATE NOT NULL, used INTEGER NOT NULL DEFAULT 0 CHECK(used BETWEEN 0 AND 65535),
 daily_limit INTEGER NOT NULL CHECK(daily_limit BETWEEN 1 AND 65535),
 used_max INTEGER NOT NULL DEFAULT 0 CHECK(used_max BETWEEN 0 AND 65535),
 updated_at TIMESTAMP NOT NULL DEFAULT (CAST((julianday('now') - 2440587.5) * 86400000000 AS INTEGER)));
 CREATE TABLE IF NOT EXISTS character_fatigue_rooms (
 character_id BIGINT NOT NULL REFERENCES characters(id), run_id TEXT NOT NULL,
 map_id BIGINT NOT NULL CHECK(map_id > 0), day DATE NOT NULL,
 cost INTEGER NOT NULL CHECK(cost BETWEEN 0 AND 65535),
 created_at TIMESTAMP NOT NULL DEFAULT (CAST((julianday('now') - 2440587.5) * 86400000000 AS INTEGER)),
 PRIMARY KEY(character_id,run_id,map_id));
 CREATE TABLE IF NOT EXISTS character_fatigue_recovery (
 character_id BIGINT NOT NULL REFERENCES characters(id), template BIGINT NOT NULL,
 day DATE NOT NULL, ordinal INTEGER NOT NULL, restored INTEGER NOT NULL,
 used_at TIMESTAMP NOT NULL,
 PRIMARY KEY(character_id,template,day,ordinal));

-- end migration

-- migration: 0022_premiums.sql
CREATE TABLE IF NOT EXISTS account_premiums(
 account_id BIGINT NOT NULL REFERENCES accounts(id),
 premium_type SMALLINT NOT NULL CHECK(premium_type BETWEEN 1 AND 255),
 end_time BIGINT NOT NULL CHECK(end_time>0),
 updated_at TIMESTAMP NOT NULL DEFAULT (CAST((julianday('now') - 2440587.5) * 86400000000 AS INTEGER)),
 PRIMARY KEY(account_id,premium_type));
CREATE INDEX IF NOT EXISTS account_premiums_expiry ON account_premiums(account_id,end_time);

-- end migration

-- migration: 0023_quests.sql
-- The additive progress_model column is folded in. The PostgreSQL section then
-- replays three historical quest-model repairs (INSERT into
-- character_quest_repairs using to_jsonb/jsonb_set, plus DELETE/UPDATE). Those
-- describe a past data state, not the schema, and use PostgreSQL-only functions,
-- so they are not carried over: converted data arrives already repaired.
CREATE TABLE IF NOT EXISTS character_quests (
 character_id BIGINT NOT NULL REFERENCES characters(id), quest_id INTEGER NOT NULL CHECK(quest_id BETWEEN 1 AND 65535),
 status TEXT NOT NULL CHECK(status IN ('accepted','completed')), progress BIGINT NOT NULL DEFAULT 0 CHECK(progress BETWEEN 0 AND 4294967295),
 config_version TEXT NOT NULL, progress_model TEXT NOT NULL DEFAULT 'legacy-zero',
 accepted_at TIMESTAMP NOT NULL DEFAULT (CAST((julianday('now') - 2440587.5) * 86400000000 AS INTEGER)), completed_at TIMESTAMP,
 PRIMARY KEY(character_id,quest_id));
 CREATE TABLE IF NOT EXISTS character_quest_repairs (
   character_id BIGINT NOT NULL, quest_id INTEGER NOT NULL, reason TEXT NOT NULL,
   before_state BLOB NOT NULL CHECK(json_valid(before_state)), after_state BLOB NOT NULL CHECK(json_valid(after_state)),
   repaired_at TIMESTAMP NOT NULL DEFAULT (CAST((julianday('now') - 2440587.5) * 86400000000 AS INTEGER)),
   PRIMARY KEY(character_id,quest_id,reason));

-- end migration

-- migration: 0024_quest_objectives.sql
CREATE TABLE IF NOT EXISTS character_map_clears (
 character_id BIGINT NOT NULL REFERENCES characters(id), run_id TEXT NOT NULL,
 map_id BIGINT NOT NULL CHECK(map_id>0), source_version TEXT NOT NULL,
 cleared_at TIMESTAMP NOT NULL DEFAULT (CAST((julianday('now') - 2440587.5) * 86400000000 AS INTEGER)),
 PRIMARY KEY(character_id,run_id,map_id));

-- end migration

-- migration: 0025_quest_rewards.sql
CREATE TABLE IF NOT EXISTS character_quest_rewards (
 character_id BIGINT NOT NULL, quest_id INTEGER NOT NULL, source_version TEXT NOT NULL,
 model TEXT NOT NULL, receipt BLOB NOT NULL CHECK(json_valid(receipt)),
 created_at TIMESTAMP NOT NULL DEFAULT (CAST((julianday('now') - 2440587.5) * 86400000000 AS INTEGER)),
 PRIMARY KEY(character_id,quest_id), FOREIGN KEY(character_id,quest_id) REFERENCES character_quests(character_id,quest_id));

-- end migration

-- migration: 0026_vaults.sql
CREATE TABLE IF NOT EXISTS character_vaults (
      character_id BIGINT PRIMARY KEY REFERENCES characters(id),
      slots INTEGER NOT NULL CHECK(slots BETWEEN 1 AND 65535),
      items BLOB NOT NULL DEFAULT (CAST('[]' AS BLOB)) CHECK(json_type(items)='array'),
      config_version TEXT NOT NULL, updated_at TIMESTAMP NOT NULL DEFAULT (CAST((julianday('now') - 2440587.5) * 86400000000 AS INTEGER)));
CREATE TABLE IF NOT EXISTS character_secondary_vaults (
      character_id BIGINT PRIMARY KEY REFERENCES characters(id),
      slots INTEGER NOT NULL CHECK(slots BETWEEN 1 AND 65535),
      items BLOB NOT NULL DEFAULT (CAST('[]' AS BLOB)) CHECK(json_type(items)='array'),
      config_version TEXT NOT NULL, updated_at TIMESTAMP NOT NULL DEFAULT (CAST((julianday('now') - 2440587.5) * 86400000000 AS INTEGER)));

-- end migration

-- migration: 0027_account_vault.sql
CREATE TABLE IF NOT EXISTS account_vaults (
 account_id BIGINT PRIMARY KEY REFERENCES accounts(id),
 slots INTEGER NOT NULL DEFAULT 0 CHECK(slots BETWEEN 0 AND 320 AND slots%8=0),
 gold BIGINT NOT NULL DEFAULT 0 CHECK(gold BETWEEN 0 AND 800000000),
 items BLOB NOT NULL DEFAULT (CAST('[]' AS BLOB)) CHECK(json_type(items)='array'),
 updated_at TIMESTAMP NOT NULL DEFAULT (CAST((julianday('now') - 2440587.5) * 86400000000 AS INTEGER)));
CREATE TABLE IF NOT EXISTS account_vault_events (
 account_id BIGINT NOT NULL REFERENCES accounts(id), event_key TEXT NOT NULL,
 character_id BIGINT NOT NULL REFERENCES characters(id), operation INTEGER NOT NULL,
 created_at TIMESTAMP NOT NULL DEFAULT (CAST((julianday('now') - 2440587.5) * 86400000000 AS INTEGER)),
 PRIMARY KEY(account_id,event_key));

-- end migration

-- migration: 0028_secondary_vault_upgrade.sql
-- PostgreSQL-only data repair (clamp secondary vault slots to 24 and merge the
-- dropped character_cargos table through a DO block). Converted data arrives
-- with the repair already applied, so this section carries no statement; it is
-- kept so both ledgers list the same section names.

-- end migration

-- migration: 0029_cash_shop.sql
-- account_premiums and its index are re-declared with IF NOT EXISTS on the
-- PostgreSQL side too; the duplicate is harmless in both dialects.
CREATE TABLE IF NOT EXISTS cash_orders(
 account_id BIGINT NOT NULL REFERENCES accounts(id), order_key TEXT NOT NULL,
 character_id BIGINT NOT NULL REFERENCES characters(id), digest TEXT NOT NULL,
 request BLOB NOT NULL CHECK(json_valid(request)), receipt BLOB NOT NULL CHECK(json_valid(receipt)),
 created_at TIMESTAMP NOT NULL DEFAULT (CAST((julianday('now') - 2440587.5) * 86400000000 AS INTEGER)),
 PRIMARY KEY(account_id,order_key));
 CREATE TABLE IF NOT EXISTS cash_inventory(
 id INTEGER PRIMARY KEY AUTOINCREMENT,
 account_id BIGINT NOT NULL REFERENCES accounts(id), character_id BIGINT NOT NULL REFERENCES characters(id),
 order_key TEXT NOT NULL, line_index INTEGER NOT NULL CHECK(line_index>=0),
 product BIGINT NOT NULL CHECK(product>0), template BIGINT NOT NULL CHECK(template>0),
 amount BIGINT NOT NULL CHECK(amount>0 AND amount<=4294967295),
 claimed_at TIMESTAMP, created_at TIMESTAMP NOT NULL DEFAULT (CAST((julianday('now') - 2440587.5) * 86400000000 AS INTEGER)),
 UNIQUE(account_id,order_key,line_index),
 FOREIGN KEY(account_id,order_key) REFERENCES cash_orders(account_id,order_key));
 CREATE TABLE IF NOT EXISTS account_premiums(
 account_id BIGINT NOT NULL REFERENCES accounts(id),
 premium_type SMALLINT NOT NULL CHECK(premium_type BETWEEN 1 AND 255),
 end_time BIGINT NOT NULL CHECK(end_time>0),
 updated_at TIMESTAMP NOT NULL DEFAULT (CAST((julianday('now') - 2440587.5) * 86400000000 AS INTEGER)),
 PRIMARY KEY(account_id,premium_type));
 CREATE INDEX IF NOT EXISTS account_premiums_expiry ON account_premiums(account_id,end_time);

-- end migration

-- migration: 0030_omen_state.sql
CREATE TABLE IF NOT EXISTS character_omen_state (
 character_id BIGINT NOT NULL REFERENCES characters(id), dungeon_id BIGINT NOT NULL CHECK(dungeon_id>0),
 held INTEGER NOT NULL DEFAULT 0 CHECK(held>=0),
 orthaire_pending BOOLEAN NOT NULL DEFAULT 0,
 updated_at TIMESTAMP NOT NULL DEFAULT (CAST((julianday('now') - 2440587.5) * 86400000000 AS INTEGER)),
 PRIMARY KEY(character_id,dungeon_id));

-- end migration

-- migration: 0031_oath_progress.sql
CREATE TABLE IF NOT EXISTS character_oath_progress (
 character_id BIGINT NOT NULL REFERENCES characters(id), dungeon_id BIGINT NOT NULL CHECK(dungeon_id>0),
 clears INTEGER NOT NULL DEFAULT 0 CHECK(clears>=0),
 updated_at TIMESTAMP NOT NULL DEFAULT (CAST((julianday('now') - 2440587.5) * 86400000000 AS INTEGER)),
 PRIMARY KEY(character_id,dungeon_id));

-- end migration

-- migration: 0032_oath_options.sql
CREATE TABLE IF NOT EXISTS character_oath_options (
 character_id BIGINT NOT NULL REFERENCES characters(id),
 core_instance_key TEXT NOT NULL CHECK(length(core_instance_key)>0),
 selected_option INTEGER NOT NULL CHECK(selected_option>=0),
 revision BIGINT NOT NULL DEFAULT 1 CHECK(revision>0),
 updated_at TIMESTAMP NOT NULL DEFAULT (CAST((julianday('now') - 2440587.5) * 86400000000 AS INTEGER)),
 PRIMARY KEY(character_id,core_instance_key));
 CREATE INDEX IF NOT EXISTS character_oath_options_updated_idx
 ON character_oath_options(character_id,updated_at);

-- end migration

-- migration: 0033_bleeding_mine.sql
-- members is a PostgreSQL bigint[]; SQLite has no array type, so it becomes a
-- JSON array in a BLOB and the length check is spelled json_array_length (D27).
CREATE TABLE IF NOT EXISTS account_bleeding_mine_teams (
 account_id BIGINT NOT NULL REFERENCES accounts(id),
 team INTEGER NOT NULL CHECK(team BETWEEN 0 AND 2),
 members BLOB NOT NULL CHECK(json_valid(members) AND json_array_length(members)=4),
 updated_at TIMESTAMP NOT NULL DEFAULT (CAST((julianday('now') - 2440587.5) * 86400000000 AS INTEGER)),
 PRIMARY KEY(account_id,team));
CREATE TABLE IF NOT EXISTS account_bleeding_mine_rewards (
 account_id BIGINT PRIMARY KEY REFERENCES accounts(id),
 state BLOB NOT NULL DEFAULT (CAST('{}' AS BLOB)) CHECK(json_valid(state)),
 updated_at TIMESTAMP NOT NULL DEFAULT (CAST((julianday('now') - 2440587.5) * 86400000000 AS INTEGER)));

-- end migration

-- migration: 0034_tower_grief.sql
CREATE TABLE IF NOT EXISTS account_tower_grief_progress (
 account_id BIGINT PRIMARY KEY REFERENCES accounts(id),
 highest_cleared SMALLINT NOT NULL DEFAULT 0 CHECK(highest_cleared BETWEEN 0 AND 100),
 cleared_day DATE,
 last_run_id TEXT NOT NULL DEFAULT ''
 );

-- end migration

-- migration: 0035_tower_progress.sql
-- The PostgreSQL section seeds this table from account_tower_grief_progress. That
-- is a one-time data migration from the superseded table, so it is not replayed
-- here (converted data already carries the seeded rows).
CREATE TABLE IF NOT EXISTS account_tower_progress (
 account_id BIGINT NOT NULL REFERENCES accounts(id),
 tower_key TEXT NOT NULL,
 highest_cleared SMALLINT NOT NULL DEFAULT 0 CHECK(highest_cleared >= 0),
 entry_day DATE,
 entries_today SMALLINT NOT NULL DEFAULT 0 CHECK(entries_today >= 0),
 last_run_id TEXT NOT NULL DEFAULT '',
 PRIMARY KEY(account_id,tower_key));

-- end migration

-- migration: 0036_mailbox.sql
-- The PostgreSQL section creates mailbox_id_seq and defaults character_mail.id to
-- nextval(...). AUTOINCREMENT is the exact equivalent (monotonic, never reused)
-- and removes the schema's only sequence. ALTER ... ALTER COLUMN sender_id DROP
-- NOT NULL is a no-op here because the column is already nullable.
CREATE TABLE IF NOT EXISTS character_mail (
 id INTEGER PRIMARY KEY AUTOINCREMENT,
 sender_id BIGINT REFERENCES characters(id),
 recipient_id BIGINT NOT NULL REFERENCES characters(id),
 sender_name TEXT NOT NULL, body TEXT NOT NULL,
 status SMALLINT NOT NULL DEFAULT 1 CHECK(status IN (1,2,3)),
 assets BLOB NOT NULL DEFAULT (CAST('[]' AS BLOB)) CHECK(json_type(assets)='array'),
 created_at TIMESTAMP NOT NULL DEFAULT (CAST((julianday('now') - 2440587.5) * 86400000000 AS INTEGER)),
 expires_at TIMESTAMP NOT NULL,
 deleted_at TIMESTAMP,
 CHECK(length(CAST(sender_name AS BLOB))<=29), CHECK(length(CAST(body AS BLOB))<=512));
CREATE INDEX IF NOT EXISTS character_mail_inbox ON character_mail(recipient_id,id) WHERE deleted_at IS NULL;

-- end migration

-- migration: 0037_gm_mail.sql
-- Management dashboard queue; independent of the game's character_mail.
CREATE TABLE IF NOT EXISTS gm_mail (
 id INTEGER PRIMARY KEY AUTOINCREMENT,
 to_account_id BIGINT NOT NULL, to_character_id BIGINT NOT NULL DEFAULT 0,
 template BIGINT NOT NULL CHECK (template > 0),
 amount BIGINT NOT NULL CHECK (amount > 0 AND amount <= 4294967295),
 title TEXT NOT NULL DEFAULT '', body TEXT NOT NULL DEFAULT '',
 status TEXT NOT NULL DEFAULT 'unread', claimed_at TIMESTAMP, expires_at TIMESTAMP,
 created_at TIMESTAMP NOT NULL DEFAULT (CAST((julianday('now') - 2440587.5) * 86400000000 AS INTEGER)),
 CONSTRAINT gm_mail_status_check CHECK (status IN ('unread','read','claimed','expired','revoked'))
);
CREATE INDEX IF NOT EXISTS gm_mail_to_account_idx ON gm_mail(to_account_id, status);
CREATE INDEX IF NOT EXISTS gm_mail_to_char_idx ON gm_mail(to_character_id, status);

-- end migration
