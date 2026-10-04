package database

import (
	"context"
	"dfolan/internal/character"
	"dfolan/internal/database/sqlcgen"
	"fmt"
	"math"
	"time"

	"github.com/jackc/pgx/v5"
)

// MigrateRosterBackgrounds 仅新增账号表，保留原有角色与物品存档。
func (s *Store) MigrateRosterBackgrounds(ctx context.Context) error {
	return s.execMigration(ctx, "0012_roster_backgrounds.sql")
}

// RosterBackgrounds 在同一个快照中读取选择和授权；未设置的页使用原版基础背景 0。
func (s *Store) RosterBackgrounds(ctx context.Context, account int64) (character.RosterBackgroundState, error) {
	tx, err := s.db.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		return character.RosterBackgroundState{}, err
	}
	defer tx.Rollback(ctx)
	state, err := readRosterBackgrounds(ctx, s.queries.WithTx(tx), account)
	if err != nil {
		return character.RosterBackgroundState{}, err
	}
	return state, tx.Commit(ctx)
}

func readRosterBackgrounds(ctx context.Context, queries *sqlcgen.Queries, account int64) (character.RosterBackgroundState, error) {
	var state character.RosterBackgroundState
	if account <= 0 {
		return state, fmt.Errorf("选角背景缺少账号")
	}
	unlocks, err := queries.RosterBackgroundUnlocks(ctx, sqlcgen.RosterBackgroundUnlocksParams{AccountID: account, NowUnix: time.Now().Unix()})
	if err != nil {
		return state, err
	}
	for _, row := range unlocks {
		state.Owned = append(state.Owned, character.RosterBackgroundUnlock{
			RosterBackground: character.RosterBackground{Category: byte(row.Category), ID: uint16(row.BackgroundID)},
			ExpiresAt:        uint32(row.ExpiresAt),
		})
	}
	pages, err := queries.RosterBackgrounds(ctx, account)
	if err != nil {
		return state, err
	}
	for _, row := range pages {
		page := int(row.Page)
		if page < 0 || page >= character.RosterBackgroundPages {
			return state, fmt.Errorf("选角背景存档页号无效")
		}
		b := character.RosterBackground{Category: byte(row.Category), ID: uint16(row.BackgroundID)}
		// Revoked authorization only falls back on this page, without rewriting saves.
		if state.CanSelect(b) {
			state.Selected[page] = b
		}
	}
	return state, state.Validate()
}

func (s *Store) SelectRosterBackground(ctx context.Context, account int64, page uint16, b character.RosterBackground) (character.RosterBackgroundState, error) {
	var state character.RosterBackgroundState
	if account <= 0 || page >= character.RosterBackgroundPages || !b.Valid() {
		return state, fmt.Errorf("选角背景请求无效")
	}
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return state, err
	}
	defer tx.Rollback(ctx)
	// 锁账号行，防止多连接分别保存不同页时覆盖对方的选择。
	queries := s.queries.WithTx(tx)
	if _, err = queries.LockAccount(ctx, account); err != nil {
		return state, err
	}
	state, err = readRosterBackgrounds(ctx, s.queries.WithTx(tx), account)
	if err != nil {
		return state, err
	}
	if !state.CanSelect(b) {
		return state, fmt.Errorf("该特殊背景尚未解锁")
	}
	err = queries.SelectRosterBackground(ctx, sqlcgen.SelectRosterBackgroundParams{AccountID: account, Page: int16(page), Category: int16(b.Category), BackgroundID: int32(b.ID)})
	if err != nil {
		return state, err
	}
	state.Selected[page] = b
	return state, tx.Commit(ctx)
}

// UnlockRosterBackground 必须在扣券的同一角色事件事务中调用。
// 与背景选择共用账号行锁；已有永久或未到期授权不覆盖，防止跨角色重复扣券。
func (tx *Tx) UnlockRosterBackground(ctx context.Context, grant character.RosterBackgroundUnlock, now time.Time) error {
	if tx.accountID <= 0 || grant.Category != 1 || !grant.Valid() || grant.ExpiresAt > math.MaxInt32 ||
		grant.ExpiresAt != 0 && int64(grant.ExpiresAt) <= now.Unix() {
		return fmt.Errorf("背景授权或到期时间无效")
	}
	// The event entry point already holds the account lock before the actor lock.
	changed, err := tx.queries.UnlockRosterBackground(ctx, sqlcgen.UnlockRosterBackgroundParams{
		AccountID: tx.accountID, Category: int16(grant.Category), BackgroundID: int32(grant.ID),
		ExpiresAt: int64(grant.ExpiresAt), NowUnix: now.Unix(),
	})
	if err != nil {
		return err
	}
	if changed != 1 {
		return fmt.Errorf("该背景已经解锁，未消耗道具")
	}
	return nil
}
