package storage

import (
	"context"
	"encoding/json"
	"os"
	"testing"

	"github.com/jackc/pgx/v5"
)

func TestSingleHuntProgressMigrationPostgres(t *testing.T) {
	path := os.Getenv("DFO_TEST_STORAGE_CONFIG")
	if path == "" {
		t.Skip("set DFO_TEST_STORAGE_CONFIG for PostgreSQL temporary-table regression")
	}
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var cfg struct {
		DSN string `json:"postgres_dsn"`
	}
	if err = json.Unmarshal(b, &cfg); err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	c, err := pgx.Connect(ctx, cfg.DSN)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close(ctx)
	tx, err := c.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)
	_, err = tx.Exec(ctx, `
 CREATE TEMP TABLE character_quests (
 character_id bigint, quest_id integer, status text, progress bigint,
 config_version text, progress_model text, accepted_at timestamptz, completed_at timestamptz);
 CREATE TEMP TABLE character_quest_repairs (
 character_id bigint, quest_id integer, reason text, before_state jsonb, after_state jsonb,
 UNIQUE(character_id,quest_id,reason));
 INSERT INTO character_quests VALUES
 (1,3526,'accepted',1,'source','antber-3526-hunt-enemy-remaining-v1','2026-09-25',NULL),
 (2,3526,'accepted',0,'source','antber-3526-hunt-enemy-remaining-v1','2026-09-25',NULL),
 (3,3526,'completed',0,'source','antber-3526-hunt-enemy-remaining-v1','2026-09-25','2026-09-25'),
 (4,3526,'accepted',2,'source','antber-3526-hunt-enemy-remaining-v1','2026-09-25',NULL),
 (5,3586,'accepted',1,'source','single-hunt-enemy-remaining-v1','2026-09-25',NULL);
 CREATE TEMP TABLE before_quests AS SELECT * FROM character_quests;`)
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		if _, err = tx.Exec(ctx, migrateSingleHuntProgressSQL); err != nil {
			t.Fatal(err)
		}
		var bad, audits int
		err = tx.QueryRow(ctx, `SELECT count(*) FROM character_quests q JOIN before_quests b USING(character_id)
 WHERE (to_jsonb(q)-'progress_model') IS DISTINCT FROM (to_jsonb(b)-'progress_model')
 OR q.progress_model IS DISTINCT FROM CASE WHEN q.character_id IN (1,2)
 THEN 'single-hunt-enemy-remaining-v1' ELSE b.progress_model END`).Scan(&bad)
		if err != nil || bad != 0 {
			t.Fatalf("pass %d: changed save fields or wrong model: %d %v", i, bad, err)
		}
		err = tx.QueryRow(ctx, `SELECT count(*) FROM character_quest_repairs r
 JOIN before_quests b USING(character_id) JOIN character_quests q USING(character_id)
 WHERE r.before_state=to_jsonb(b) AND r.after_state=to_jsonb(q)`).Scan(&audits)
		if err != nil || audits != 2 {
			t.Fatalf("pass %d: audit count=%d err=%v", i, audits, err)
		}
	}
}
