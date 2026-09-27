package storage

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
)

// MigrateOathProgress 建「隐藏 BOSS 通关保底」的计数表。
//
// 客户端那套阶梯没有掷骰：`oath_max == 45` 就每次通关都召唤奥尔泰尔
// （docs/protocol/endkeeper-of-order-primer-20260926.md §32.1/§32.2）。
// 所以「稀有」只能由服务端表达 —— 这张表按 (角色, 副本) 记「自上次保底以来
// 通关了几场」。放在库里而不是内存里，是因为保底要跨会话、跨重启都在。
func (s *Store) MigrateOathProgress(ctx context.Context) error {
	_, err := s.DB.Exec(ctx, `CREATE TABLE IF NOT EXISTS character_oath_progress (
 character_id bigint NOT NULL REFERENCES characters(id), dungeon_id bigint NOT NULL CHECK(dungeon_id>0),
 clears integer NOT NULL DEFAULT 0 CHECK(clears>=0),
 updated_at timestamptz NOT NULL DEFAULT now(), PRIMARY KEY(character_id,dungeon_id));`)
	return err
}

// OathProgressClears 返回该角色在这个副本上「自上次保底以来」的通关数。
// 没有记录就是 0（还没打过，或上一次保底刚兑现过）。
func (s *Store) OathProgressClears(ctx context.Context, characterID, dungeonID int64) (int, error) {
	if characterID <= 0 || dungeonID <= 0 {
		return 0, errors.New("invalid oath progress key")
	}
	var clears int
	err := s.DB.QueryRow(ctx, `SELECT clears FROM character_oath_progress WHERE character_id=$1 AND dungeon_id=$2`,
		characterID, dungeonID).Scan(&clears)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	return clears, nil
}

// BumpOathProgress 记一次通关，返回 (推进前, 推进后) 的场次。
//
// 推进前已经到 needed ⇒ 这次通关就是把保底兑现掉，归零；否则只 +1。
// 归零刻意放在**通关时**而不是进本时：进本就归零会让掉线/退出吞掉已攒的场次。
func (s *Store) BumpOathProgress(ctx context.Context, characterID, dungeonID int64, needed int) (int, int, error) {
	if characterID <= 0 || dungeonID <= 0 || needed <= 0 {
		return 0, 0, errors.New("invalid oath progress bump")
	}
	tx, err := s.DB.Begin(ctx)
	if err != nil {
		return 0, 0, err
	}
	defer tx.Rollback(ctx)
	var before int
	err = tx.QueryRow(ctx, `SELECT clears FROM character_oath_progress WHERE character_id=$1 AND dungeon_id=$2 FOR UPDATE`,
		characterID, dungeonID).Scan(&before)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return 0, 0, err
	}
	next := before + 1
	if before >= needed {
		next = 0
	}
	if _, err = tx.Exec(ctx, `INSERT INTO character_oath_progress(character_id,dungeon_id,clears,updated_at) VALUES($1,$2,$3,now())
 ON CONFLICT(character_id,dungeon_id) DO UPDATE SET clears=EXCLUDED.clears,updated_at=now()`,
		characterID, dungeonID, next); err != nil {
		return 0, 0, err
	}
	if err = tx.Commit(ctx); err != nil {
		return 0, 0, err
	}
	return before, next, nil
}
