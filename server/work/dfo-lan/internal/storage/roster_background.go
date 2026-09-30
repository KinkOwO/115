package storage

import (
	"context"
	"dfolan/internal/rosterbg"
	"fmt"
	"math"
	"time"

	"github.com/jackc/pgx/v5"
)

// MigrateRosterBackgrounds 仅新增账号表，保留原有角色与物品存档。
func (s *Store) MigrateRosterBackgrounds(ctx context.Context) error {
	_, err := s.DB.Exec(ctx, `CREATE TABLE IF NOT EXISTS account_roster_backgrounds (
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
 CHECK(expires_at BETWEEN 0 AND 2147483647);`)
	return err
}

// RosterBackgrounds 在同一个快照中读取选择和授权；未设置的页使用原版基础背景 0。
func (s *Store) RosterBackgrounds(ctx context.Context, account int64) (rosterbg.State, error) {
	tx, err := s.DB.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		return rosterbg.State{}, err
	}
	defer tx.Rollback(ctx)
	state, err := readRosterBackgrounds(ctx, tx, account)
	if err != nil {
		return rosterbg.State{}, err
	}
	return state, tx.Commit(ctx)
}

func readRosterBackgrounds(ctx context.Context, tx pgx.Tx, account int64) (rosterbg.State, error) {
	var state rosterbg.State
	if account <= 0 {
		return state, fmt.Errorf("选角背景缺少账号")
	}
	rows, err := tx.Query(ctx, `SELECT category,background_id,expires_at FROM account_roster_background_unlocks
 WHERE account_id=$1 AND (expires_at=0 OR expires_at>$2) ORDER BY category,background_id`, account, time.Now().Unix())
	if err != nil {
		return state, err
	}
	for rows.Next() {
		var b rosterbg.Unlock
		if err = rows.Scan(&b.Category, &b.ID, &b.ExpiresAt); err != nil {
			rows.Close()
			return state, err
		}
		state.Owned = append(state.Owned, b)
	}
	rows.Close()
	if err = rows.Err(); err != nil {
		return state, err
	}
	rows, err = tx.Query(ctx, `SELECT page,category,background_id FROM account_roster_backgrounds WHERE account_id=$1 ORDER BY page`, account)
	if err != nil {
		return state, err
	}
	defer rows.Close()
	for rows.Next() {
		var page int
		var b rosterbg.Background
		if err = rows.Scan(&page, &b.Category, &b.ID); err != nil {
			return state, err
		}
		if page < 0 || page >= rosterbg.Pages {
			return state, fmt.Errorf("选角背景存档页号无效")
		}
		// 授权被撤销或资源不再存在时只回退此页，不覆盖其它页存档。
		if state.CanSelect(b) {
			state.Selected[page] = b
		}
	}
	if err = rows.Err(); err != nil {
		return state, err
	}
	return state, state.Validate()
}

func (s *Store) SelectRosterBackground(ctx context.Context, account int64, page uint16, b rosterbg.Background) (rosterbg.State, error) {
	var state rosterbg.State
	if account <= 0 || page >= rosterbg.Pages || !b.Valid() {
		return state, fmt.Errorf("选角背景请求无效")
	}
	tx, err := s.DB.Begin(ctx)
	if err != nil {
		return state, err
	}
	defer tx.Rollback(ctx)
	// 锁账号行，防止多连接分别保存不同页时覆盖对方的选择。
	if err = tx.QueryRow(ctx, `SELECT id FROM accounts WHERE id=$1 FOR UPDATE`, account).Scan(new(int64)); err != nil {
		return state, err
	}
	state, err = readRosterBackgrounds(ctx, tx, account)
	if err != nil {
		return state, err
	}
	if !state.CanSelect(b) {
		return state, fmt.Errorf("该特殊背景尚未解锁")
	}
	_, err = tx.Exec(ctx, `INSERT INTO account_roster_backgrounds(account_id,page,category,background_id) VALUES($1,$2,$3,$4)
 ON CONFLICT(account_id,page) DO UPDATE SET category=EXCLUDED.category,background_id=EXCLUDED.background_id,updated_at=now()`, account, page, b.Category, b.ID)
	if err != nil {
		return state, err
	}
	state.Selected[page] = b
	return state, tx.Commit(ctx)
}

// UnlockRosterBackground 必须在扣券的同一角色事件事务中调用。
// 与背景选择共用账号行锁；已有永久或未到期授权不覆盖，防止跨角色重复扣券。
func (s *Store) UnlockRosterBackground(ctx context.Context, tx pgx.Tx, account int64, grant rosterbg.Unlock, now time.Time) error {
	if account <= 0 || grant.Category != 1 || !grant.Valid() || grant.ExpiresAt > math.MaxInt32 ||
		grant.ExpiresAt != 0 && int64(grant.ExpiresAt) <= now.Unix() {
		return fmt.Errorf("背景授权或到期时间无效")
	}
	var locked int64
	if err := tx.QueryRow(ctx, `SELECT id FROM accounts WHERE id=$1 FOR UPDATE`, account).Scan(&locked); err != nil {
		return err
	}
	result, err := tx.Exec(ctx, `INSERT INTO account_roster_background_unlocks(account_id,category,background_id,expires_at)
 VALUES($1,$2,$3,$4) ON CONFLICT(account_id,category,background_id) DO UPDATE SET expires_at=EXCLUDED.expires_at
 WHERE account_roster_background_unlocks.expires_at>0 AND account_roster_background_unlocks.expires_at<=$5`,
		account, grant.Category, grant.ID, grant.ExpiresAt, now.Unix())
	if err != nil {
		return err
	}
	if result.RowsAffected() != 1 {
		return fmt.Errorf("该背景已经解锁，未消耗道具")
	}
	return nil
}
