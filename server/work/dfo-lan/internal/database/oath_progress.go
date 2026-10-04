package database

import (
	"context"
	"dfolan/internal/database/sqlcgen"
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
	return s.execMigration(ctx, "0031_oath_progress.sql")
}

// OathProgressClears 返回该角色在这个副本上「自上次保底以来」的通关数。
// 没有记录就是 0（还没打过，或上一次保底刚兑现过）。
func (s *Store) OathProgressClears(ctx context.Context, characterID, dungeonID int64) (int, error) {
	if characterID <= 0 || dungeonID <= 0 {
		return 0, errors.New("invalid oath progress key")
	}
	clears, err := s.queries.OathProgressClears(ctx, sqlcgen.OathProgressClearsParams{CharacterID: characterID, DungeonID: dungeonID})
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	return int(clears), nil
}

// BumpOathProgress 记一次通关，返回 (推进前, 推进后) 的场次。
//
// 推进前已经到 needed ⇒ 这次通关就是把保底兑现掉，归零；否则只 +1。
// 归零刻意放在**通关时**而不是进本时：进本就归零会让掉线/退出吞掉已攒的场次。
func (s *Store) BumpOathProgress(ctx context.Context, characterID, dungeonID int64, needed int) (int, int, error) {
	if characterID <= 0 || dungeonID <= 0 || needed <= 0 {
		return 0, 0, errors.New("invalid oath progress bump")
	}
	tx, err := s.engine.begin(ctx)
	if err != nil {
		return 0, 0, err
	}
	defer tx.rollback(ctx)
	queries := tx.queries()
	stored, err := queries.LockOathProgress(ctx, sqlcgen.LockOathProgressParams{CharacterID: characterID, DungeonID: dungeonID})
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return 0, 0, err
	}
	before := int(stored)
	next := before + 1
	if before >= needed {
		next = 0
	}
	if err = queries.SaveOathProgress(ctx, sqlcgen.SaveOathProgressParams{CharacterID: characterID, DungeonID: dungeonID, Clears: int64(next)}); err != nil {
		return 0, 0, err
	}
	if err = tx.commit(ctx); err != nil {
		return 0, 0, err
	}
	return before, next, nil
}
