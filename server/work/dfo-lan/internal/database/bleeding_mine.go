package database

import (
	"context"
	"dfolan/internal/database/sqlcgen"
	"encoding/json"
	"fmt"
)

// 赤红铁矿编队独立存储，不覆盖冒险团精锐配置、角色装备或技能。
// 成员保存稳定角色ID，零表示空槽；索引仅用于网络投影。
func (s *Store) MigrateBleedingMine(ctx context.Context) error {
	return s.execMigration(ctx, "0033_bleeding_mine.sql")
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
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return role, nil, err
	}
	defer tx.Rollback(ctx)
	role, err = lockCharacter(ctx, tx, account, actor)
	if err != nil {
		return role, nil, err
	}
	if role.ConfigVersion != version {
		return role, nil, fmt.Errorf("矿区奖励角色配置版本不匹配")
	}
	queries := s.queries.WithTx(tx)
	if err = queries.EnsureBleedingMineRewards(ctx, account); err != nil {
		return role, nil, err
	}
	var prior json.RawMessage
	if prior, err = queries.LockBleedingMineRewards(ctx, account); err != nil {
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
		if _, err = insertSystemMailTx(ctx, tx, actor, "赤红铁矿", "本次探索获得的奖励，请领取附件。", assets); err != nil {
			return role, nil, err
		}
	}
	if err = sqlcgen.New(tx).UpdateCharacterState(ctx, sqlcgen.UpdateCharacterStateParams{CharacterID: actor, State: state}); err != nil {
		return role, nil, err
	}
	if err = queries.SaveBleedingMineRewards(ctx, sqlcgen.SaveBleedingMineRewardsParams{AccountID: account, State: rewards}); err != nil {
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
	rows, err := s.queries.BleedingMineTeams(ctx, account)
	if err != nil {
		return result, err
	}
	for _, row := range rows {
		group, ids := int(row.Team), row.Members
		if group < 0 || group >= len(result) || len(ids) != 4 {
			return result, fmt.Errorf("赤红铁矿编队存档无效")
		}
		copy(result[group][:], ids)
	}
	return result, nil
}

// 与创建、删除和排序共用账号锁；整个编队一次替换，重复保存不会累计副作用。
func (s *Store) SaveBleedingMineTeam(ctx context.Context, account, actor int64, group uint32, ids [4]int64) error {
	if account <= 0 || actor <= 0 || group >= 3 {
		return fmt.Errorf("赤红铁矿编队保存参数无效")
	}
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	queries := s.queries.WithTx(tx)
	if _, err = queries.LockAccount(ctx, account); err != nil {
		return err
	}
	rows, err := queries.ShareActiveCharacterIDs(ctx, account)
	if err != nil {
		return err
	}
	owned := map[int64]bool{}
	for _, id := range rows {
		owned[id] = true
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
	if err = queries.SaveBleedingMineTeam(ctx, sqlcgen.SaveBleedingMineTeamParams{AccountID: account, Team: int32(group), Members: ids[:]}); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
