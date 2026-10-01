package storage

import (
	"context"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
)

// 2026-10-01（next146）：存档来源身份重钉的回归。
//
// 关键不变量（见 source_rebaseline.go 文件头）：
//  1. **已知历史内层哈希** 的行被重钉到 current；
//  2. **其它来源身份**（vault 的 `fda6c33f…`）**绝不能**被碰；
//  3. 跑到第二遍是 no-op（幂等）；
//  4. current 必须是 64 hex。
func TestMigrateSourceIdentityRebaselinesOnlyHistoricalInner(t *testing.T) {
	path := os.Getenv("DFO_TEST_STORAGE_CONFIG")
	if path == "" {
		t.Skip("set DFO_TEST_STORAGE_CONFIG for PostgreSQL temporary-table regression")
	}
	config, err := LoadConfig(path)
	if err != nil {
		t.Fatal(err)
	}
	poolConfig, err := pgxpool.ParseConfig(config.PostgresDSN)
	if err != nil {
		t.Fatal(err)
	}
	poolConfig.MaxConns = 1
	ctx := context.Background()
	pool, err := pgxpool.NewWithConfig(ctx, poolConfig)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()

	const (
		historical = "7ef2db59331f7e5b18b2f250b8b907526bf2c94b17a7312036cf599644d88e80"
		current    = "b2b503b58ca12ed88befaa2d44b6d222cfb15e94aaff7a7dfa32f6fa0133b13e"
		vaultID    = "fda6c33f2a155124a4262f3e7cd6f7a18d4e2191e5f147aec5754716afe5aa7c"
		unknown    = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	)
	_, err = pool.Exec(ctx, `CREATE TEMP TABLE characters(id bigint,config_version text);
 CREATE TEMP TABLE character_world(character_id bigint,config_version text);
 CREATE TEMP TABLE character_quests(character_id bigint,quest_id integer,config_version text);
 CREATE TEMP TABLE character_events(character_id bigint,event_key text,config_version text);
 CREATE TEMP TABLE character_map_clears(character_id bigint,map_id bigint,source_version text);
 CREATE TEMP TABLE character_quest_rewards(character_id bigint,quest_id integer,source_version text);
 CREATE TEMP TABLE character_vaults(character_id bigint,config_version text);
 INSERT INTO characters VALUES (11,'`+historical+`'),(12,'`+unknown+`');
 INSERT INTO character_world VALUES (11,'`+historical+`'),(12,'`+unknown+`');
 INSERT INTO character_quests VALUES (11,3151,'`+historical+`'),(12,3145,'`+unknown+`');
 INSERT INTO character_events VALUES (11,'k','`+historical+`'),(12,'k','`+unknown+`');
 INSERT INTO character_map_clears VALUES (11,1,'`+historical+`');
 INSERT INTO character_quest_rewards VALUES (11,3151,'`+historical+`');
 INSERT INTO character_vaults VALUES (11,'`+vaultID+`'),(12,'`+vaultID+`');`)
	if err != nil {
		t.Fatal(err)
	}
	store := &Store{DB: pool}

	if _, err := store.MigrateSourceIdentity(ctx, "short"); err == nil {
		t.Fatal("non-sha256 current identity accepted")
	}

	n, err := store.MigrateSourceIdentity(ctx, current)
	if err != nil {
		t.Fatal(err)
	}
	// 1 characters + 1 world + 1 quests + 1 events + 1 map_clears + 1 quest_rewards = 6
	if n != 6 {
		t.Fatalf("expected 6 rebaselined rows, got %d", n)
	}

	check := func(table string) {
		t.Helper()
		var stale, fresh, other int
		if err := pool.QueryRow(ctx, `SELECT count(*) FILTER (WHERE config_version='`+historical+`'),
 count(*) FILTER (WHERE config_version='`+current+`'),
 count(*) FILTER (WHERE config_version='`+unknown+`') FROM `+table).Scan(&stale, &fresh, &other); err != nil {
			t.Fatal(err)
		}
		if stale != 0 || fresh != 1 {
			t.Fatalf("%s: stale=%d fresh=%d (want 0/1)", table, stale, fresh)
		}
		if table == "characters" || table == "character_world" || table == "character_quests" ||
			table == "character_events" {
			if other != 1 {
				t.Fatalf("%s: unknown identity must be preserved, got %d", table, other)
			}
		}
	}
	for _, tbl := range []string{"characters", "character_world", "character_quests", "character_events"} {
		check(tbl)
	}
	var mv, vr int
	if err := pool.QueryRow(ctx, `SELECT count(*) FILTER (WHERE source_version='`+historical+`'),
 count(*) FILTER (WHERE source_version='`+current+`') FROM character_map_clears`).Scan(&mv, &vr); err != nil {
		t.Fatal(err)
	}
	if mv != 0 || vr != 1 {
		t.Fatalf("character_map_clears: stale=%d fresh=%d", mv, vr)
	}

	// vaults 用的是另一套来源身份，绝不能被重钉。
	var vaultUntouched int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM character_vaults WHERE config_version=$1`, vaultID).Scan(&vaultUntouched); err != nil {
		t.Fatal(err)
	}
	if vaultUntouched != 2 {
		t.Fatalf("vault identity was touched: %d/2 rows still carry the vault source", vaultUntouched)
	}

	// 幂等：第二遍没有可重钉的行。
	again, err := store.MigrateSourceIdentity(ctx, current)
	if err != nil {
		t.Fatal(err)
	}
	if again != 0 {
		t.Fatalf("rebaseline is not idempotent: %d rows on second pass", again)
	}
}
