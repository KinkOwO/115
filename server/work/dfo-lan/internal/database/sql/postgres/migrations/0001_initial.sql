-- Initial PostgreSQL schema, consolidated from the original module migrations.
-- Sections retain their historical ledger IDs, checksums and execution gates.
-- Preserve section bodies; future upgrades belong in new incremental files.

-- migration: 0000_migration_ledger.sql
-- Infrastructure ledger. Existing save tables are adopted by running their
-- original additive SQL before a successful migration is recorded.
CREATE TABLE IF NOT EXISTS storage_migrations (
 name text PRIMARY KEY,
 checksum text NOT NULL CHECK(length(checksum)=64),
 applied_at timestamptz NOT NULL DEFAULT now(),
 last_applied_at timestamptz NOT NULL DEFAULT now(),
 runs bigint NOT NULL DEFAULT 1 CHECK(runs>0)
);

-- end migration

-- migration: 0001_core.sql
-- Extracted without schema changes from store.go; additive and rerunnable.
CREATE TABLE IF NOT EXISTS accounts (
 id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
 username text NOT NULL UNIQUE,
 password_hash text,
 development_only boolean NOT NULL DEFAULT true,
 created_at timestamptz NOT NULL DEFAULT now(),
 CHECK(development_only OR password_hash IS NOT NULL)
 );
 CREATE TABLE IF NOT EXISTS characters (
 id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
 account_id bigint NOT NULL REFERENCES accounts(id),
 wire_id integer NOT NULL CHECK(wire_id BETWEEN 1 AND 65534),
 name text NOT NULL,
 profession integer NOT NULL CHECK(profession BETWEEN 0 AND 255),
 create_request bytea NOT NULL,
 config_version text NOT NULL,
 state jsonb NOT NULL,
 created_at timestamptz NOT NULL DEFAULT now(),
 UNIQUE(account_id,wire_id));
 ALTER TABLE characters ADD COLUMN IF NOT EXISTS deleted_at timestamptz;
 ALTER TABLE characters ADD COLUMN IF NOT EXISTS max_fame integer NOT NULL DEFAULT 0 CHECK(max_fame >= 0);
 ALTER TABLE characters ADD COLUMN IF NOT EXISTS roster_order bigint CHECK(roster_order > 0);
 ALTER TABLE characters ADD COLUMN IF NOT EXISTS fixed_slot smallint NOT NULL DEFAULT 0 CHECK(fixed_slot BETWEEN 0 AND 255);
 CREATE UNIQUE INDEX IF NOT EXISTS characters_name_unique ON characters(lower(name));
 CREATE TABLE IF NOT EXISTS npc_favor (
 character_id bigint NOT NULL REFERENCES characters(id),
 npc_id bigint NOT NULL,
 point bigint NOT NULL DEFAULT 0,
 daily_count integer NOT NULL DEFAULT 0,
 last_gift_day date,
 updated_at timestamptz NOT NULL DEFAULT now(),
 PRIMARY KEY(character_id,npc_id));

-- end migration

-- migration: 0002_character_events.sql
-- Extracted without schema changes from character_event.go; additive and rerunnable.
CREATE TABLE IF NOT EXISTS character_events (
 character_id bigint NOT NULL REFERENCES characters(id), event_key text NOT NULL,
 config_version text NOT NULL, model text NOT NULL, outcome jsonb NOT NULL,
 created_at timestamptz NOT NULL DEFAULT now(), PRIMARY KEY(character_id,event_key));

-- end migration

-- migration: 0003_character_notices.sql
-- Extracted without schema changes from character_notice_store.go; additive and rerunnable.
CREATE TABLE IF NOT EXISTS character_notice_seen (
 account_id bigint NOT NULL REFERENCES accounts(id),
 character_id bigint NOT NULL,
 tree smallint NOT NULL,
 notice_id smallint NOT NULL,
 updated_at timestamptz NOT NULL DEFAULT now(),
 PRIMARY KEY(account_id, character_id, tree, notice_id));

-- end migration

-- migration: 0004_tutorial.sql
-- Extracted without schema changes from tutorial.go; additive and rerunnable.
CREATE TABLE IF NOT EXISTS character_tutorial_flags(
 character_id bigint NOT NULL REFERENCES characters(id),
 flag integer NOT NULL CHECK(flag BETWEEN 0 AND 100),
 completed boolean NOT NULL, updated_at timestamptz NOT NULL DEFAULT now(),
 PRIMARY KEY(character_id,flag));

-- end migration

-- migration: 0005_warp_favorites.sql
-- Extracted without schema changes from warp_favorites.go; additive and rerunnable.
CREATE TABLE IF NOT EXISTS account_warp_favorites (
 account_id bigint NOT NULL REFERENCES accounts(id),
 slot_index smallint NOT NULL CHECK(slot_index BETWEEN 0 AND 9),
 value bytea NOT NULL CHECK(octet_length(value) = 12),
 updated_at timestamptz NOT NULL DEFAULT now(),
 PRIMARY KEY(account_id,slot_index));

-- end migration

-- migration: 0006_adventure.sql
-- Extracted from adventure.go without schema changes.
CREATE TABLE IF NOT EXISTS account_adventures (
 account_id bigint PRIMARY KEY REFERENCES accounts(id),
 name text NOT NULL CHECK(length(name)>0),
 level integer NOT NULL DEFAULT 1 CHECK(level BETWEEN 1 AND 50),
 experience bigint NOT NULL DEFAULT 0 CHECK(experience>=0),
 created_at timestamptz NOT NULL DEFAULT now());
 ALTER TABLE account_adventures ADD COLUMN IF NOT EXISTS data jsonb NOT NULL DEFAULT '{}';

-- end migration

-- migration: 0007_equipment_skill.sql
-- Extracted from equipment_skill.go without schema changes.
CREATE TABLE IF NOT EXISTS character_equipment_skill (
 character_id bigint PRIMARY KEY REFERENCES characters(id),
 skills bytea,
 commands bytea,
 updated_at timestamptz NOT NULL DEFAULT now());

-- end migration

-- migration: 0008_skill_locks.sql
-- Extracted from skill_lock.go without schema changes.
CREATE TABLE IF NOT EXISTS character_skill_locks (
 character_id bigint NOT NULL REFERENCES characters(id),
 skill_id integer NOT NULL CHECK(skill_id BETWEEN 1 AND 1023),
 PRIMARY KEY(character_id, skill_id));

-- end migration

-- migration: 0009_profile_skins.sql
-- Extracted from profile_skin.go without schema changes.
CREATE TABLE IF NOT EXISTS character_profile_skins (
 character_id bigint PRIMARY KEY REFERENCES characters(id),
 state jsonb NOT NULL,
 updated_at timestamptz NOT NULL DEFAULT now());

-- end migration

-- migration: 0010_gamepad.sql
-- Extracted from gamepad.go without schema changes.
CREATE TABLE IF NOT EXISTS account_gamepad_settings (
    account_id bigint NOT NULL REFERENCES accounts(id) PRIMARY KEY,
    mapping_tsv bytea NOT NULL DEFAULT ''::bytea,
    options bytea NOT NULL DEFAULT ''::bytea,
    updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE TABLE IF NOT EXISTS character_gamepad_settings (
    character_id bigint NOT NULL REFERENCES characters(id) PRIMARY KEY,
    account_id bigint NOT NULL REFERENCES accounts(id),
    mapping_tsv bytea NOT NULL DEFAULT ''::bytea,
    options bytea NOT NULL DEFAULT ''::bytea,
    updated_at timestamptz NOT NULL DEFAULT now()
);

-- end migration

-- migration: 0011_shop_purchases.sql
-- Extracted from shop_purchase.go without schema changes.
CREATE TABLE IF NOT EXISTS character_shop_purchases (
 character_id bigint NOT NULL REFERENCES characters(id),
 account_id bigint NOT NULL,
 npc_id integer NOT NULL,
 template integer NOT NULL,
 bought_at timestamptz NOT NULL DEFAULT now());
CREATE INDEX IF NOT EXISTS character_shop_purchases_window
 ON character_shop_purchases(character_id, npc_id, template, bought_at);

-- end migration

-- migration: 0012_roster_backgrounds.sql
-- Extracted from roster_background.go without schema changes.
CREATE TABLE IF NOT EXISTS account_roster_backgrounds (
 account_id bigint NOT NULL REFERENCES accounts(id),
 page smallint NOT NULL CHECK(page BETWEEN 0 AND 4),
 category smallint NOT NULL CHECK(category BETWEEN 0 AND 1),
 background_id integer NOT NULL CHECK(background_id BETWEEN 0 AND 65535),
 updated_at timestamptz NOT NULL DEFAULT now(),
 PRIMARY KEY(account_id,page));
CREATE TABLE IF NOT EXISTS account_roster_background_unlocks (
 account_id bigint NOT NULL REFERENCES accounts(id),
 category smallint NOT NULL CHECK(category=1),
 background_id integer NOT NULL CHECK(background_id BETWEEN 0 AND 65535),
 created_at timestamptz NOT NULL DEFAULT now(),
 PRIMARY KEY(account_id,category,background_id));
ALTER TABLE account_roster_background_unlocks ADD COLUMN IF NOT EXISTS expires_at bigint NOT NULL DEFAULT 0
 CHECK(expires_at BETWEEN 0 AND 2147483647);

-- end migration

-- migration: 0013_skin_selection.sql
-- Extracted from skin_selection.go without schema changes.
CREATE TABLE IF NOT EXISTS character_skin_selection (
 character_id bigint NOT NULL REFERENCES characters(id),
 page bigint NOT NULL,
 skin_key bigint NOT NULL,
 updated_at timestamptz NOT NULL DEFAULT now(),
 PRIMARY KEY(character_id,page));

-- end migration

-- migration: 0014_skin_lists.sql
-- Extracted from skin_selection_list.go without schema changes.
CREATE TABLE IF NOT EXISTS character_skin_selection_list(
 character_id bigint NOT NULL REFERENCES characters(id),
 category bigint NOT NULL,
 skin_key bigint NOT NULL,
 updated_at timestamptz NOT NULL DEFAULT now(),
 PRIMARY KEY(character_id,category,skin_key));
CREATE TABLE IF NOT EXISTS character_skin_selection_slot(
 character_id bigint NOT NULL REFERENCES characters(id),
 category bigint NOT NULL,
 slot bigint NOT NULL,
 skin_key bigint NOT NULL,
 updated_at timestamptz NOT NULL DEFAULT now(),
 PRIMARY KEY(character_id,category,slot));
CREATE TABLE IF NOT EXISTS character_skin_favorite(
 character_id bigint NOT NULL REFERENCES characters(id),
 page bigint NOT NULL,
 skin_key bigint NOT NULL,
 updated_at timestamptz NOT NULL DEFAULT now(),
 PRIMARY KEY(character_id,page,skin_key));

-- end migration

-- migration: 0015_skin_cargo.sql
-- Legacy upgrade extracted from skin_cargo.go; no dropped columns.
CREATE TABLE IF NOT EXISTS account_skin_cargo (
 account_id bigint NOT NULL REFERENCES accounts(id),
 source_template bigint NOT NULL,
 skin_key bigint NOT NULL,
 unlocked_at timestamptz NOT NULL DEFAULT now(),
 PRIMARY KEY(account_id, source_template));
ALTER TABLE account_skin_cargo ADD COLUMN IF NOT EXISTS skin_key bigint NOT NULL DEFAULT 0;
-- Keep legacy columns and only repair rows with a missing v2 key.
DO $legacy$
BEGIN
 IF EXISTS(SELECT 1 FROM information_schema.columns WHERE table_schema=current_schema()
 AND table_name='account_skin_cargo' AND column_name='action_param') THEN
  UPDATE account_skin_cargo SET skin_key=action_param WHERE skin_key=0 AND action_param<>0;
  ALTER TABLE account_skin_cargo ALTER COLUMN skin_index SET DEFAULT 0;
 END IF;
END $legacy$;

-- end migration

-- migration: 0016_unified_options.sql
-- Extracted from unified_option.go without schema changes.
CREATE TABLE IF NOT EXISTS account_unified_options(
 account_id bigint NOT NULL REFERENCES accounts(id),
 opt_index integer NOT NULL CHECK(opt_index BETWEEN 0 AND 285),
 value integer NOT NULL,
 updated_at timestamptz NOT NULL DEFAULT now(),
 PRIMARY KEY(account_id,opt_index));
CREATE TABLE IF NOT EXISTS character_unified_options(
 character_id bigint NOT NULL REFERENCES characters(id),
 opt_index integer NOT NULL CHECK(opt_index BETWEEN 0 AND 285),
 value integer NOT NULL,
 updated_at timestamptz NOT NULL DEFAULT now(),
 PRIMARY KEY(character_id,opt_index));
CREATE TABLE IF NOT EXISTS account_hotkeys(
 account_id bigint NOT NULL REFERENCES accounts(id),
 subtype smallint NOT NULL CHECK(subtype IN (3, 4)),
 slot_index smallint NOT NULL CHECK(slot_index BETWEEN 0 AND 156),
 keycode integer NOT NULL,
 updated_at timestamptz NOT NULL DEFAULT now(),
 PRIMARY KEY(account_id,subtype,slot_index));
CREATE TABLE IF NOT EXISTS character_hotkeys(
 character_id bigint NOT NULL REFERENCES characters(id),
 subtype smallint NOT NULL CHECK(subtype IN (3, 4)),
 slot_index smallint NOT NULL CHECK(slot_index BETWEEN 0 AND 156),
 keycode integer NOT NULL,
 updated_at timestamptz NOT NULL DEFAULT now(),
	 PRIMARY KEY(character_id,subtype,slot_index));
CREATE TABLE IF NOT EXISTS character_unified_option_groups(
 character_id bigint NOT NULL REFERENCES characters(id),
 subtype smallint NOT NULL CHECK(subtype = 18),
 opt_index integer NOT NULL CHECK(opt_index BETWEEN 0 AND 5),
 value integer NOT NULL CHECK(value BETWEEN 0 AND 65535),
 updated_at timestamptz NOT NULL DEFAULT now(),
 PRIMARY KEY(character_id,subtype,opt_index));

-- end migration

-- migration: 0017_account_materials.sql
-- Extracted without schema changes from account_materials.go.
CREATE TABLE IF NOT EXISTS account_material_storage (
 account_id bigint PRIMARY KEY REFERENCES accounts(id),
 counts jsonb NOT NULL DEFAULT '{}'::jsonb,
 updated_at timestamptz NOT NULL DEFAULT now());

-- end migration

-- migration: 0018_grants.sql
-- Extracted from grant.go without schema changes.
CREATE TABLE IF NOT EXISTS account_currency(
 account_id bigint PRIMARY KEY REFERENCES accounts(id),
 cera bigint NOT NULL DEFAULT 0 CHECK(cera>=0),
 updated_at timestamptz NOT NULL DEFAULT now());
CREATE TABLE IF NOT EXISTS admin_grants(
 grant_id text PRIMARY KEY,
 account_id bigint NOT NULL REFERENCES accounts(id),
 character_id bigint REFERENCES characters(id),
 request jsonb NOT NULL, receipt jsonb NOT NULL,
 operator text NOT NULL, reason text NOT NULL,
 created_at timestamptz NOT NULL DEFAULT now());

-- end migration

-- migration: 0019_world.sql
CREATE TABLE IF NOT EXISTS character_world (
 character_id bigint PRIMARY KEY REFERENCES characters(id),
 position jsonb NOT NULL, config_version text NOT NULL,
 revision bigint NOT NULL DEFAULT 1 CHECK(revision>0),
 updated_at timestamptz NOT NULL DEFAULT now());

CREATE TABLE IF NOT EXISTS character_channel_world (
 character_id bigint NOT NULL REFERENCES characters(id),
 channel_type integer NOT NULL CHECK(channel_type>0 AND channel_type<=255),
 position jsonb NOT NULL, config_version text NOT NULL,
 revision bigint NOT NULL DEFAULT 1 CHECK(revision>0),
 updated_at timestamptz NOT NULL DEFAULT now(),
 PRIMARY KEY(character_id, channel_type));

-- end migration

-- migration: 0020_birth.sql
CREATE TABLE IF NOT EXISTS character_birth(
 character_id bigint PRIMARY KEY REFERENCES characters(id),
 stage smallint NOT NULL CHECK(stage BETWEEN 0 AND 3),
 dungeon bigint NOT NULL DEFAULT 0 CHECK(dungeon>=0),
 updated_at timestamptz NOT NULL DEFAULT now());

-- end migration

-- migration: 0021_fatigue.sql
CREATE TABLE IF NOT EXISTS character_fatigue (
 character_id bigint PRIMARY KEY REFERENCES characters(id),
 day date NOT NULL, used integer NOT NULL DEFAULT 0 CHECK(used BETWEEN 0 AND 65535),
 daily_limit integer NOT NULL CHECK(daily_limit BETWEEN 1 AND 65535),
 used_max integer NOT NULL DEFAULT 0 CHECK(used_max BETWEEN 0 AND 65535),
 updated_at timestamptz NOT NULL DEFAULT now());
 CREATE TABLE IF NOT EXISTS character_fatigue_rooms (
 character_id bigint NOT NULL REFERENCES characters(id), run_id text NOT NULL,
 map_id bigint NOT NULL CHECK(map_id > 0), day date NOT NULL,
 cost integer NOT NULL CHECK(cost BETWEEN 0 AND 65535),
 created_at timestamptz NOT NULL DEFAULT now(),
 PRIMARY KEY(character_id,run_id,map_id));
 CREATE TABLE IF NOT EXISTS character_fatigue_recovery (
 character_id bigint NOT NULL REFERENCES characters(id), template bigint NOT NULL,
 day date NOT NULL, ordinal integer NOT NULL, restored integer NOT NULL,
 used_at timestamptz NOT NULL,
 PRIMARY KEY(character_id,template,day,ordinal));

-- end migration

-- migration: 0022_premiums.sql
CREATE TABLE IF NOT EXISTS account_premiums(
 account_id bigint NOT NULL REFERENCES accounts(id),
 premium_type smallint NOT NULL CHECK(premium_type BETWEEN 1 AND 255),
 end_time bigint NOT NULL CHECK(end_time>0),
 updated_at timestamptz NOT NULL DEFAULT now(),
 PRIMARY KEY(account_id,premium_type));
CREATE INDEX IF NOT EXISTS account_premiums_expiry ON account_premiums(account_id,end_time);

-- end migration

-- migration: 0023_quests.sql
CREATE TABLE IF NOT EXISTS character_quests (
 character_id bigint NOT NULL REFERENCES characters(id), quest_id integer NOT NULL CHECK(quest_id BETWEEN 1 AND 65535),
 status text NOT NULL CHECK(status IN ('accepted','completed')), progress bigint NOT NULL DEFAULT 0 CHECK(progress BETWEEN 0 AND 4294967295),
 config_version text NOT NULL, accepted_at timestamptz NOT NULL DEFAULT now(), completed_at timestamptz,
 PRIMARY KEY(character_id,quest_id));
 ALTER TABLE character_quests ADD COLUMN IF NOT EXISTS progress_model text NOT NULL DEFAULT 'legacy-zero';
 CREATE TABLE IF NOT EXISTS character_quest_repairs (
   character_id bigint NOT NULL, quest_id integer NOT NULL, reason text NOT NULL,
   before_state jsonb NOT NULL, after_state jsonb NOT NULL, repaired_at timestamptz NOT NULL DEFAULT now(),
   PRIMARY KEY(character_id,quest_id,reason));

 -- attempt 1 inserted every recursively reachable Act quest. Rows created by
 -- that path have identical accepted/completed timestamps; a genuinely
 -- accepted quest has an earlier accepted_at. Preserve an audit copy before
 -- removing only those synthetic, never-accepted rows.
 INSERT INTO character_quest_repairs(character_id,quest_id,reason,before_state,after_state)
 SELECT q.character_id,q.quest_id,'act-clear-v1-unaccepted-insert',to_jsonb(q),'{"deleted":true}'::jsonb
 FROM character_quests q
 WHERE q.progress_model='act-clear-v1' AND q.status='completed' AND q.accepted_at=q.completed_at
 ON CONFLICT (character_id,quest_id,reason) DO NOTHING;

 DELETE FROM character_quests
 WHERE progress_model='act-clear-v1' AND status='completed' AND accepted_at=completed_at;
 -- Historical model translation, not a new quest rule. The first 3281
 -- candidate saved a quest-specific name before the generic NPC reader.
 -- Preserve progress and timestamps and record the complete original row.
 INSERT INTO character_quest_repairs(character_id,quest_id,reason,before_state,after_state)
 SELECT q.character_id,q.quest_id,'reach-npc-model-v1',to_jsonb(q),
        jsonb_set(to_jsonb(q),'{progress_model}','"alflyra-3252-reach-npc-remaining-v1"'::jsonb)
 FROM character_quests q
 WHERE q.quest_id=3281 AND q.status='accepted' AND q.progress BETWEEN 0 AND 1
   AND q.progress_model='stormpass-3281-reach-npc-remaining-v1'
 ON CONFLICT (character_id,quest_id,reason) DO NOTHING;

 UPDATE character_quests
 SET progress_model='alflyra-3252-reach-npc-remaining-v1'
 WHERE quest_id=3281 AND status='accepted' AND progress BETWEEN 0 AND 1
   AND progress_model='stormpass-3281-reach-npc-remaining-v1';

 -- The first single-target hunt candidate also saved a quest-specific name.
 -- Only translate its accepted 0/1 saves to the existing source-shape model.
 INSERT INTO character_quest_repairs(character_id,quest_id,reason,before_state,after_state)
 SELECT q.character_id,q.quest_id,'single-hunt-model-v1',to_jsonb(q),
        jsonb_set(to_jsonb(q),'{progress_model}','"single-hunt-enemy-remaining-v1"'::jsonb)
 FROM character_quests q
 WHERE q.status='accepted' AND q.progress BETWEEN 0 AND 1
   AND q.progress_model='antber-3526-hunt-enemy-remaining-v1'
 ON CONFLICT (character_id,quest_id,reason) DO NOTHING;

 UPDATE character_quests
 SET progress_model='single-hunt-enemy-remaining-v1'
 WHERE status='accepted' AND progress BETWEEN 0 AND 1
   AND progress_model='antber-3526-hunt-enemy-remaining-v1';


-- end migration

-- migration: 0024_quest_objectives.sql
CREATE TABLE IF NOT EXISTS character_map_clears (
 character_id bigint NOT NULL REFERENCES characters(id), run_id text NOT NULL,
 map_id bigint NOT NULL CHECK(map_id>0), source_version text NOT NULL,
 cleared_at timestamptz NOT NULL DEFAULT now(), PRIMARY KEY(character_id,run_id,map_id));

-- end migration

-- migration: 0025_quest_rewards.sql
CREATE TABLE IF NOT EXISTS character_quest_rewards (
 character_id bigint NOT NULL, quest_id integer NOT NULL, source_version text NOT NULL,
 model text NOT NULL, receipt jsonb NOT NULL, created_at timestamptz NOT NULL DEFAULT now(),
 PRIMARY KEY(character_id,quest_id), FOREIGN KEY(character_id,quest_id) REFERENCES character_quests(character_id,quest_id));

-- end migration

-- migration: 0026_vaults.sql
CREATE TABLE IF NOT EXISTS character_vaults (
      character_id bigint PRIMARY KEY REFERENCES characters(id),
      slots integer NOT NULL CHECK(slots BETWEEN 1 AND 65535),
      items jsonb NOT NULL DEFAULT '[]' CHECK(jsonb_typeof(items)='array'),
      config_version text NOT NULL, updated_at timestamptz NOT NULL DEFAULT now());
CREATE TABLE IF NOT EXISTS character_secondary_vaults (
      character_id bigint PRIMARY KEY REFERENCES characters(id),
      slots integer NOT NULL CHECK(slots BETWEEN 1 AND 65535),
      items jsonb NOT NULL DEFAULT '[]' CHECK(jsonb_typeof(items)='array'),
      config_version text NOT NULL, updated_at timestamptz NOT NULL DEFAULT now());

-- end migration

-- migration: 0027_account_vault.sql
CREATE TABLE IF NOT EXISTS account_vaults (
 account_id bigint PRIMARY KEY REFERENCES accounts(id),
 slots integer NOT NULL DEFAULT 0 CHECK(slots BETWEEN 0 AND 320 AND slots%8=0),
 gold bigint NOT NULL DEFAULT 0 CHECK(gold BETWEEN 0 AND 800000000),
 items jsonb NOT NULL DEFAULT '[]' CHECK(jsonb_typeof(items)='array'),
 updated_at timestamptz NOT NULL DEFAULT now());
CREATE TABLE IF NOT EXISTS account_vault_events (
 account_id bigint NOT NULL REFERENCES accounts(id), event_key text NOT NULL,
 character_id bigint NOT NULL REFERENCES characters(id), operation integer NOT NULL,
 created_at timestamptz NOT NULL DEFAULT now(), PRIMARY KEY(account_id,event_key));

-- end migration

-- migration: 0028_secondary_vault_upgrade.sql
UPDATE character_secondary_vaults SET slots=24,updated_at=now() WHERE slots<24;

DO $$ BEGIN
 IF to_regclass('character_cargos') IS NOT NULL THEN
  EXECUTE 'UPDATE character_secondary_vaults s SET slots=c.slots,updated_at=now() FROM character_cargos c WHERE c.kind=45 AND c.character_id=s.character_id AND c.slots>s.slots';
 END IF;
END $$;

-- end migration

-- migration: 0029_cash_shop.sql
CREATE TABLE IF NOT EXISTS cash_orders(
 account_id bigint NOT NULL REFERENCES accounts(id), order_key text NOT NULL,
 character_id bigint NOT NULL REFERENCES characters(id), digest text NOT NULL,
 request jsonb NOT NULL, receipt jsonb NOT NULL, created_at timestamptz NOT NULL DEFAULT now(),
 PRIMARY KEY(account_id,order_key));
 CREATE TABLE IF NOT EXISTS cash_inventory(
 id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
 account_id bigint NOT NULL REFERENCES accounts(id), character_id bigint NOT NULL REFERENCES characters(id),
 order_key text NOT NULL, line_index integer NOT NULL CHECK(line_index>=0),
 product bigint NOT NULL CHECK(product>0), template bigint NOT NULL CHECK(template>0),
 amount bigint NOT NULL CHECK(amount>0 AND amount<=4294967295),
 claimed_at timestamptz, created_at timestamptz NOT NULL DEFAULT now(),
 UNIQUE(account_id,order_key,line_index),
 FOREIGN KEY(account_id,order_key) REFERENCES cash_orders(account_id,order_key));
 CREATE TABLE IF NOT EXISTS account_premiums(
 account_id bigint NOT NULL REFERENCES accounts(id),
 premium_type smallint NOT NULL CHECK(premium_type BETWEEN 1 AND 255),
 end_time bigint NOT NULL CHECK(end_time>0),
 updated_at timestamptz NOT NULL DEFAULT now(),
 PRIMARY KEY(account_id,premium_type));
 CREATE INDEX IF NOT EXISTS account_premiums_expiry ON account_premiums(account_id,end_time);

-- end migration

-- migration: 0030_omen_state.sql
CREATE TABLE IF NOT EXISTS character_omen_state (
 character_id bigint NOT NULL REFERENCES characters(id), dungeon_id bigint NOT NULL CHECK(dungeon_id>0),
 held integer NOT NULL DEFAULT 0 CHECK(held>=0),
 orthaire_pending boolean NOT NULL DEFAULT false,
 updated_at timestamptz NOT NULL DEFAULT now(), PRIMARY KEY(character_id,dungeon_id));

-- end migration

-- migration: 0031_oath_progress.sql
CREATE TABLE IF NOT EXISTS character_oath_progress (
 character_id bigint NOT NULL REFERENCES characters(id), dungeon_id bigint NOT NULL CHECK(dungeon_id>0),
 clears integer NOT NULL DEFAULT 0 CHECK(clears>=0),
 updated_at timestamptz NOT NULL DEFAULT now(), PRIMARY KEY(character_id,dungeon_id));

-- end migration

-- migration: 0032_oath_options.sql
CREATE TABLE IF NOT EXISTS character_oath_options (
 character_id bigint NOT NULL REFERENCES characters(id),
 core_instance_key text NOT NULL CHECK(length(core_instance_key)>0),
 selected_option integer NOT NULL CHECK(selected_option>=0),
 revision bigint NOT NULL DEFAULT 1 CHECK(revision>0),
 updated_at timestamptz NOT NULL DEFAULT now(),
 PRIMARY KEY(character_id,core_instance_key));
 CREATE INDEX IF NOT EXISTS character_oath_options_updated_idx
 ON character_oath_options(character_id,updated_at);

-- end migration

-- migration: 0033_bleeding_mine.sql
CREATE TABLE IF NOT EXISTS account_bleeding_mine_teams (
 account_id bigint NOT NULL REFERENCES accounts(id),
 team integer NOT NULL CHECK(team BETWEEN 0 AND 2),
 members bigint[] NOT NULL CHECK(array_length(members,1)=4),
 updated_at timestamptz NOT NULL DEFAULT now(),
 PRIMARY KEY(account_id,team));
CREATE TABLE IF NOT EXISTS account_bleeding_mine_rewards (
 account_id bigint PRIMARY KEY REFERENCES accounts(id),
 state jsonb NOT NULL DEFAULT '{}'::jsonb,
 updated_at timestamptz NOT NULL DEFAULT now());

-- end migration

-- migration: 0034_tower_grief.sql
CREATE TABLE IF NOT EXISTS account_tower_grief_progress (
 account_id bigint PRIMARY KEY REFERENCES accounts(id),
 highest_cleared smallint NOT NULL DEFAULT 0 CHECK(highest_cleared BETWEEN 0 AND 100),
 cleared_day date,
 last_run_id text NOT NULL DEFAULT ''
 );

-- end migration

-- migration: 0035_tower_progress.sql
CREATE TABLE IF NOT EXISTS account_tower_progress (
 account_id bigint NOT NULL REFERENCES accounts(id),
 tower_key text NOT NULL,
 highest_cleared smallint NOT NULL DEFAULT 0 CHECK(highest_cleared >= 0),
 entry_day date,
 entries_today smallint NOT NULL DEFAULT 0 CHECK(entries_today >= 0),
 last_run_id text NOT NULL DEFAULT '',
 PRIMARY KEY(account_id,tower_key));

-- Preserve the original Grief table and only seed absent tower progress.
INSERT INTO account_tower_progress(account_id,tower_key,highest_cleared,entry_day,entries_today,last_run_id)
SELECT account_id,'grief',highest_cleared,cleared_day,
CASE WHEN cleared_day IS NULL THEN 0 ELSE 1 END,last_run_id FROM account_tower_grief_progress
ON CONFLICT(account_id,tower_key) DO NOTHING;

-- end migration

-- migration: 0036_mailbox.sql
CREATE SEQUENCE IF NOT EXISTS mailbox_id_seq;
CREATE TABLE IF NOT EXISTS character_mail (
 id bigint PRIMARY KEY DEFAULT nextval('mailbox_id_seq'),
 sender_id bigint REFERENCES characters(id),
 recipient_id bigint NOT NULL REFERENCES characters(id),
 sender_name text NOT NULL, body text NOT NULL,
 status smallint NOT NULL DEFAULT 1 CHECK(status IN (1,2,3)),
 assets jsonb NOT NULL DEFAULT '[]' CHECK(jsonb_typeof(assets)='array'),
 created_at timestamptz NOT NULL DEFAULT now(), expires_at timestamptz NOT NULL,
 deleted_at timestamptz,
 CHECK(octet_length(sender_name)<=29), CHECK(octet_length(body)<=512));
ALTER TABLE character_mail ALTER COLUMN sender_id DROP NOT NULL;
CREATE INDEX IF NOT EXISTS character_mail_inbox ON character_mail(recipient_id,id) WHERE deleted_at IS NULL;

-- end migration

-- migration: 0037_gm_mail.sql
-- Management dashboard queue; independent of the game's character_mail.
-- Preserve the existing Python-created table and pending administrative mail.
CREATE TABLE IF NOT EXISTS gm_mail (
 id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
 to_account_id bigint NOT NULL, to_character_id bigint NOT NULL DEFAULT 0,
 template bigint NOT NULL CHECK (template > 0),
 amount bigint NOT NULL CHECK (amount > 0 AND amount <= 4294967295),
 title text NOT NULL DEFAULT '', body text NOT NULL DEFAULT '',
 status text NOT NULL DEFAULT 'unread', claimed_at timestamptz, expires_at timestamptz,
 created_at timestamptz NOT NULL DEFAULT now(),
 CONSTRAINT gm_mail_status_check CHECK (status IN ('unread','read','claimed','expired','revoked'))
);
CREATE INDEX IF NOT EXISTS gm_mail_to_account_idx ON gm_mail(to_account_id, status);
CREATE INDEX IF NOT EXISTS gm_mail_to_char_idx ON gm_mail(to_character_id, status);

-- end migration
