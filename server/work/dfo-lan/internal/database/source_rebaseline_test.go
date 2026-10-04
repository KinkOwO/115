package database

import (
	"testing"

	"dfolan/internal/savecontract"
)

// 2026-10-01（next146）：存档身份归一的回归。
//
// 关键不变量（见 source_rebaseline.go 文件头）：
//  1. **任何不等于当次契约身份的取值**都被归一（内层归档哈希 / 更早的批次标记 / 空串 / NULL）；
//  2. **其它来源身份**（vault 的 `fda6c33f…`）**绝不能**被碰；
//  3. 跑到第二遍是 no-op（幂等）；
//  4. current 必须是**合法契约身份**（64 hex）。
//
// 为什么判据**不限定形状**：玩家可能从任意更老的版本升级，
// 而这 6 列的**唯一**语义就是「目录来源身份」，任何取值都该被归一。
func TestMigrateSaveIdentityNormalizesAnyHistoricalIdentity(t *testing.T) {
	store, ctx := sqlcTestStore(t)
	pool := store.db

	const (
		historical = "7ef2db59331f7e5b18b2f250b8b907526bf2c94b17a7312036cf599644d88e80"
		previous   = "b2b503b58ca12ed88befaa2d44b6d222cfb15e94aaff7a7dfa32f6fa0133b13e"
		vaultID    = "fda6c33f2a155124a4262f3e7cd6f7a18d4e2191e5f147aec5754716afe5aa7c"
		annotation = "manual:legacy-source-annotation"
	)
	contract := savecontract.Identity()
	_, err := pool.Exec(ctx, `CREATE TABLE characters(id bigint,config_version text);
 CREATE TABLE character_world(character_id bigint,config_version text);
 CREATE TABLE character_quests(character_id bigint,quest_id integer,config_version text);
 CREATE TABLE character_events(character_id bigint,event_key text,config_version text);
 CREATE TABLE character_map_clears(character_id bigint,map_id bigint,source_version text);
 CREATE TABLE character_quest_rewards(character_id bigint,quest_id integer,source_version text);
 CREATE TABLE character_vaults(character_id bigint,config_version text);
 INSERT INTO characters VALUES (11,'`+historical+`'),(12,'`+previous+`'),(13,'`+annotation+`'),(14,''),(15,NULL);
 INSERT INTO character_world VALUES (11,'`+historical+`'),(12,'`+previous+`');
 INSERT INTO character_quests VALUES (11,3151,'`+historical+`'),(12,3145,'`+previous+`');
 INSERT INTO character_events VALUES (11,'k','`+historical+`'),(12,'k','`+previous+`');
 INSERT INTO character_map_clears VALUES (11,1,'`+historical+`');
 INSERT INTO character_quest_rewards VALUES (11,3151,'`+historical+`');
 INSERT INTO character_vaults VALUES (11,'`+vaultID+`'),(12,'`+vaultID+`');`)
	if err != nil {
		t.Fatal(err)
	}

	if _, err := store.MigrateSaveIdentity(ctx, "short"); err == nil {
		t.Fatal("non-sha256 current identity accepted")
	}

	n, err := store.MigrateSaveIdentity(ctx, contract)
	if err != nil {
		t.Fatal(err)
	}
	// 5 characters (含空串与 NULL) + 2 world + 2 quests + 2 events + 1 map_clears + 1 quest_rewards = 13
	if n != 13 {
		t.Fatalf("expected 13 normalized rows, got %d", n)
	}

	// characters：5 行全部归一（historical / previous / 标注 / 空串 / NULL）。
	var stray int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM characters WHERE config_version IS DISTINCT FROM $1`, contract).Scan(&stray); err != nil {
		t.Fatal(err)
	}
	if stray != 0 {
		t.Fatalf("characters: %d row(s) still differ from the contract identity", stray)
	}
	for _, tbl := range []string{"character_world", "character_quests", "character_events"} {
		var bad int
		if err := pool.QueryRow(ctx, `SELECT count(*) FROM `+tbl+` WHERE config_version IS DISTINCT FROM $1`, contract).Scan(&bad); err != nil {
			t.Fatal(err)
		}
		if bad != 0 {
			t.Fatalf("%s: %d row(s) still differ from the contract identity", tbl, bad)
		}
	}
	var mapBad int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM character_map_clears WHERE source_version IS DISTINCT FROM $1`, contract).Scan(&mapBad); err != nil {
		t.Fatal(err)
	}
	if mapBad != 0 {
		t.Fatalf("character_map_clears: %d row(s) still differ", mapBad)
	}
	var rewardBad int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM character_quest_rewards WHERE source_version IS DISTINCT FROM $1`, contract).Scan(&rewardBad); err != nil {
		t.Fatal(err)
	}
	if rewardBad != 0 {
		t.Fatalf("character_quest_rewards: %d row(s) still differ", rewardBad)
	}

	// vaults 用的是另一套来源身份（服务端生成物），绝不能被归一。
	var vaultUntouched int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM character_vaults WHERE config_version=$1`, vaultID).Scan(&vaultUntouched); err != nil {
		t.Fatal(err)
	}
	if vaultUntouched != 2 {
		t.Fatalf("vault identity was touched: %d/2 rows still carry the vault source", vaultUntouched)
	}

	// 幂等：第二遍没有可归一的行。
	again, err := store.MigrateSaveIdentity(ctx, contract)
	if err != nil {
		t.Fatal(err)
	}
	if again != 0 {
		t.Fatalf("normalization is not idempotent: %d rows on second pass", again)
	}
}
