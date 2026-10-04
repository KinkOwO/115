package database

import "testing"

func TestReachNPCProgressMigrationPostgres(t *testing.T) {
	store, ctx := sqlcTestStore(t)
	query, err := migrationQuery("0023_quests.sql")
	if err != nil {
		t.Fatal(err)
	}
	pool, poolErr := store.rawPool()
	if poolErr != nil {
		t.Fatal(poolErr)
	}
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)
	_, err = tx.Exec(ctx, `
 CREATE TABLE character_quests (
 character_id bigint, quest_id integer, status text, progress bigint,
 config_version text, progress_model text, accepted_at timestamptz, completed_at timestamptz);
 CREATE TABLE character_quest_repairs (
 character_id bigint, quest_id integer, reason text, before_state jsonb, after_state jsonb,
 UNIQUE(character_id,quest_id,reason));
 INSERT INTO character_quests VALUES
 (1,3281,'accepted',1,'source','stormpass-3281-reach-npc-remaining-v1','2026-09-24',NULL),
 (2,3281,'accepted',0,'source','stormpass-3281-reach-npc-remaining-v1','2026-09-23',NULL),
 (3,3281,'completed',0,'source','stormpass-3281-reach-npc-remaining-v1','2026-09-23','2026-09-24'),
 (4,3281,'accepted',2,'source','stormpass-3281-reach-npc-remaining-v1','2026-09-24',NULL),
 (5,3252,'accepted',1,'source','alflyra-3252-reach-npc-remaining-v1','2026-09-24',NULL),
 (6,3281,'accepted',1,'source','unknown-model','2026-09-24',NULL);
 CREATE TEMP TABLE before_quests AS SELECT * FROM character_quests;`)
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		if _, err = tx.Exec(ctx, string(query)); err != nil {
			t.Fatal(err)
		}
		var bad, audits int
		err = tx.QueryRow(ctx, `SELECT count(*) FROM character_quests q JOIN before_quests b USING(character_id)
 WHERE (to_jsonb(q)-'progress_model') IS DISTINCT FROM (to_jsonb(b)-'progress_model')
 OR q.progress_model IS DISTINCT FROM CASE WHEN q.character_id IN (1,2)
 THEN 'alflyra-3252-reach-npc-remaining-v1' ELSE b.progress_model END`).Scan(&bad)
		if err != nil || bad != 0 {
			t.Fatalf("pass %d: changed player state or wrong model: %d %v", i, bad, err)
		}
		err = tx.QueryRow(ctx, `SELECT count(*) FROM character_quest_repairs r
 JOIN before_quests b USING(character_id) JOIN character_quests q USING(character_id)
 WHERE r.before_state=to_jsonb(b) AND r.after_state=to_jsonb(q)`).Scan(&audits)
		if err != nil || audits != 2 {
			t.Fatalf("pass %d: audit count=%d err=%v", i, audits, err)
		}
	}
}
