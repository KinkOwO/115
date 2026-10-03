package storage

import (
	"context"
	"encoding/json"
	"fmt"
)

// 赤红铁矿编队独立存储，不覆盖冒险团精锐配置、角色装备或技能。
// 成员保存稳定角色ID，零表示空槽；索引仅用于网络投影。
func (s *Store) MigrateBleedingMine(ctx context.Context) error {
	_, err := s.DB.Exec(ctx, `CREATE TABLE IF NOT EXISTS account_bleeding_mine_teams (
 account_id bigint NOT NULL REFERENCES accounts(id),
 team integer NOT NULL CHECK(team BETWEEN 0 AND 2),
 members bigint[] NOT NULL CHECK(array_length(members,1)=4),
 updated_at timestamptz NOT NULL DEFAULT now(),
 PRIMARY KEY(account_id,team));
CREATE TABLE IF NOT EXISTS account_bleeding_mine_rewards (
 account_id bigint PRIMARY KEY REFERENCES accounts(id),
 state jsonb NOT NULL DEFAULT '{}'::jsonb,
 updated_at timestamptz NOT NULL DEFAULT now());`)
	return err
}

// 角色背包与账号矿区奖励同时提交；固定角色→矿区账号行的锁顺序。
// 回调必须在奖励存档中校验领取标记，重复请求不能再发物品。
func (s *Store) UpdateBleedingMineRewards(ctx context.Context, account, actor int64, version string,
	apply func(Character, json.RawMessage) (json.RawMessage, json.RawMessage, []MailAsset, error),
) (Character, json.RawMessage, error) {
	var role Character
	if account <= 0 || actor <= 0 || apply == nil {
		return role, nil, fmt.Errorf("矿区奖励事务参数无效")
	}
	tx, err := s.DB.Begin(ctx)
	if err != nil {
		return role, nil, err
	}
	defer tx.Rollback(ctx)
	err = tx.QueryRow(ctx, `SELECT id,account_id,wire_id,name,profession,create_request,config_version,state,created_at
 FROM characters WHERE account_id=$1 AND id=$2 AND deleted_at IS NULL FOR UPDATE`, account, actor).
		Scan(&role.ID, &role.AccountID, &role.WireID, &role.Name, &role.Profession, &role.Request, &role.ConfigVersion, &role.State, &role.CreatedAt)
	if err != nil {
		return role, nil, err
	}
	if role.ConfigVersion != version {
		return role, nil, fmt.Errorf("矿区奖励角色配置版本不匹配")
	}
	if _, err = tx.Exec(ctx, `INSERT INTO account_bleeding_mine_rewards(account_id) VALUES($1) ON CONFLICT DO NOTHING`, account); err != nil {
		return role, nil, err
	}
	var prior json.RawMessage
	if err = tx.QueryRow(ctx, `SELECT state FROM account_bleeding_mine_rewards WHERE account_id=$1 FOR UPDATE`, account).Scan(&prior); err != nil {
		return role, nil, err
	}
	state, rewards, assets, err := apply(role, prior)
	if err != nil {
		return role, nil, err
	}
	if !json.Valid(state) || !json.Valid(rewards) {
		return role, nil, fmt.Errorf("矿区奖励产生无效存档")
	}
	// 领取标记与邮件同事务提交，失败时奖励袋保持原状；不调用嵌套邮件事务。
	if len(assets) > 0 {
		if len(assets) > 11 {
			return role, nil, fmt.Errorf("矿区奖励邮件附件过多")
		}
		for i := range assets {
			if assets[i].Claimed || assets[i].Gold != 0 || !json.Valid(assets[i].Item) {
				return role, nil, fmt.Errorf("矿区邮件附件无效")
			}
		}
		if _, err = insertSystemMailTx(ctx, pgxTx{tx}, actor, "赤红铁矿", "本次探索获得的奖励，请领取附件。", assets); err != nil {
			return role, nil, err
		}
	}
	if _, err = tx.Exec(ctx, `UPDATE characters SET state=$2 WHERE id=$1`, actor, state); err != nil {
		return role, nil, err
	}
	if _, err = tx.Exec(ctx, `UPDATE account_bleeding_mine_rewards SET state=$2,updated_at=now() WHERE account_id=$1`, account, rewards); err != nil {
		return role, nil, err
	}
	if err = tx.Commit(ctx); err != nil {
		return role, nil, err
	}
	role.State = state
	return role, rewards, nil
}

func (s *Store) BleedingMineTeams(ctx context.Context, account int64) ([3][4]int64, error) {
	var result [3][4]int64
	rows, err := s.DB.Query(ctx, `SELECT team,members FROM account_bleeding_mine_teams WHERE account_id=$1`, account)
	if err != nil {
		return result, err
	}
	defer rows.Close()
	for rows.Next() {
		var group int
		var ids []int64
		if err = rows.Scan(&group, &ids); err != nil {
			return result, err
		}
		if group < 0 || group >= len(result) || len(ids) != 4 {
			return result, fmt.Errorf("赤红铁矿编队存档无效")
		}
		copy(result[group][:], ids)
	}
	return result, rows.Err()
}

// 与创建、删除和排序共用账号锁；整个编队一次替换，重复保存不会累计副作用。
func (s *Store) SaveBleedingMineTeam(ctx context.Context, account, actor int64, group uint32, ids [4]int64) error {
	if account <= 0 || actor <= 0 || group >= 3 {
		return fmt.Errorf("赤红铁矿编队保存参数无效")
	}
	tx, err := s.DB.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	var owner int64
	if err = tx.QueryRow(ctx, `SELECT id FROM accounts WHERE id=$1 FOR UPDATE`, account).Scan(&owner); err != nil {
		return err
	}
	rows, err := tx.Query(ctx, `SELECT id FROM characters WHERE account_id=$1 AND deleted_at IS NULL ORDER BY id FOR SHARE`, account)
	if err != nil {
		return err
	}
	owned := map[int64]bool{}
	for rows.Next() {
		var id int64
		if err = rows.Scan(&id); err != nil {
			rows.Close()
			return err
		}
		owned[id] = true
	}
	rows.Close()
	if err = rows.Err(); err != nil {
		return err
	}
	if !owned[actor] {
		return fmt.Errorf("赤红铁矿当前角色已失效")
	}
	seen := map[int64]bool{}
	for _, id := range ids {
		if id == 0 {
			continue
		}
		if !owned[id] || seen[id] {
			return fmt.Errorf("赤红铁矿成员已删除、不属于当前账号或重复")
		}
		seen[id] = true
	}
	if _, err = tx.Exec(ctx, `INSERT INTO account_bleeding_mine_teams(account_id,team,members)
 VALUES($1,$2,$3) ON CONFLICT(account_id,team) DO UPDATE SET members=EXCLUDED.members,updated_at=now()`, account, group, ids[:]); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
