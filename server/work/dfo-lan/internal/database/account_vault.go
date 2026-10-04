package database

import (
	"context"
	"dfolan/internal/database/sqlcgen"
	"dfolan/internal/inventory"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
)

type AccountVaultState = inventory.AccountVaultState

func lockSharedVault(ctx context.Context, q *sqlcgen.Queries, account int64) (AccountVaultState, error) {
	row, err := q.LockAccountVault(ctx, account)
	return AccountVaultState{Slots: uint16(row.Slots), Gold: uint32(row.Gold), Items: row.Items}, err
}

func (s *Store) MigrateAccountVault(ctx context.Context) error {
	return s.execMigration(ctx, "0027_account_vault.sql")
}

func (s *Store) LoadAccountVault(ctx context.Context, account, character int64) (AccountVaultState, error) {
	var v AccountVaultState
	// 不存在的金库只是返回未开通状态，登录本身不会免费创建。
	row, err := s.queries.LoadAccountVault(ctx, sqlcgen.LoadAccountVaultParams{AccountID: account, CharacterID: character})
	v = AccountVaultState{Slots: uint16(row.Slots), Gold: uint32(row.Gold), Items: row.Items}
	return v, storageError(err)
}

func (s *Store) CommitAccountVaultSort(ctx context.Context, account, character int64, sortItems func(AccountVaultState) (json.RawMessage, error)) (AccountVaultState, error) {
	var vault AccountVaultState
	if sortItems == nil {
		return vault, errors.New("nil account vault sort")
	}
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return vault, err
	}
	defer tx.Rollback(ctx)
	queries := s.queries.WithTx(tx)
	owned, err := queries.ActiveCharacterOwned(ctx, sqlcgen.ActiveCharacterOwnedParams{AccountID: account, CharacterID: character})
	if err != nil {
		return vault, err
	}
	if !owned {
		return vault, errors.New("角色不属于账号")
	}
	if err = queries.EnsureAccountVault(ctx, account); err != nil {
		return vault, err
	}
	if vault, err = lockSharedVault(ctx, queries, account); err != nil {
		return vault, err
	}
	items, err := sortItems(vault)
	if err != nil {
		return vault, err
	}
	var rows []json.RawMessage
	if json.Unmarshal(items, &rows) != nil || rows == nil {
		return vault, errors.New("账号金库排序产生无效存档")
	}
	if err = queries.SaveAccountVaultItems(ctx, sqlcgen.SaveAccountVaultItemsParams{AccountID: account, Items: items}); err != nil {
		return vault, err
	}
	if err = tx.Commit(ctx); err != nil {
		return vault, err
	}
	vault.Items = items
	return vault, nil
}

// 固定锁顺序为角色、共享材料、账号金库；沿用材料归集的顺序，跨角色
// 操作共享同一金库行锁。扣费、升级与事件回执一并提交，失败全部回滚。
func (s *Store) CommitAccountVault(ctx context.Context, account, character int64, version, key string, operation uint16,
	apply func(Character, json.RawMessage, AccountVaultState) (json.RawMessage, json.RawMessage, AccountVaultState, error),
) (Character, json.RawMessage, AccountVaultState, bool, error) {
	var role Character
	var materials json.RawMessage
	var vault AccountVaultState
	if apply == nil || account <= 0 || character <= 0 || len(version) != 64 || key == "" || len(key) > 200 {
		return role, materials, vault, false, fmt.Errorf("账号金库事务参数无效")
	}
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return role, materials, vault, false, err
	}
	defer tx.Rollback(ctx)
	role, err = lockCharacter(ctx, tx, account, character)
	if err != nil {
		return role, materials, vault, false, err
	}
	if role.ConfigVersion != version {
		return role, materials, vault, false, fmt.Errorf("账号金库角色配置版本不匹配")
	}
	queries := s.queries.WithTx(tx)
	if err = queries.InitializeAccountMaterials(ctx, account); err != nil {
		return role, materials, vault, false, err
	}
	if materials, err = queries.LockAccountMaterials(ctx, account); err != nil {
		return role, materials, vault, false, err
	}
	if err = queries.EnsureAccountVault(ctx, account); err != nil {
		return role, materials, vault, false, err
	}
	if vault, err = lockSharedVault(ctx, queries, account); err != nil {
		return role, materials, vault, false, err
	}
	prior, err := queries.AccountVaultEvent(ctx, sqlcgen.AccountVaultEventParams{AccountID: account, EventKey: key})
	if err == nil {
		if prior.CharacterID != character || prior.Operation != int32(operation) {
			return role, materials, vault, false, fmt.Errorf("账号金库请求流水冲突")
		}
		return role, materials, vault, false, tx.Commit(ctx)
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return role, materials, vault, false, err
	}
	state, updated, next, err := apply(role, materials, vault)
	if err != nil {
		return role, materials, vault, false, err
	}
	var fields map[string]json.RawMessage
	var counts map[string]json.RawMessage
	var items []json.RawMessage
	if json.Unmarshal(state, &fields) != nil || fields == nil || json.Unmarshal(updated, &counts) != nil || counts == nil || json.Unmarshal(next.Items, &items) != nil || items == nil || next.Slots%8 != 0 || next.Slots > 320 || next.Gold > 800000000 || (next.Slots == 0 && (next.Gold != 0 || len(items) != 0)) {
		return role, materials, vault, false, fmt.Errorf("账号金库事务生成了无效存档")
	}
	if err = sqlcgen.New(tx).UpdateCharacterState(ctx, sqlcgen.UpdateCharacterStateParams{CharacterID: character, State: state}); err != nil {
		return role, materials, vault, false, err
	}
	if err = queries.SaveAccountMaterials(ctx, sqlcgen.SaveAccountMaterialsParams{AccountID: account, Counts: updated}); err != nil {
		return role, materials, vault, false, err
	}
	if err = queries.SaveAccountVault(ctx, sqlcgen.SaveAccountVaultParams{AccountID: account, Slots: int32(next.Slots), Gold: int64(next.Gold), Items: next.Items}); err != nil {
		return role, materials, vault, false, err
	}
	if err = queries.RecordAccountVaultEvent(ctx, sqlcgen.RecordAccountVaultEventParams{AccountID: account, EventKey: key, CharacterID: character, Operation: int32(operation)}); err != nil {
		return role, materials, vault, false, err
	}
	if err = tx.Commit(ctx); err != nil {
		return role, materials, vault, false, err
	}
	role.State = state
	return role, updated, next, true, nil
}

// CommitAccountVaultCrossMove updates a character's personal and shared vault
// in one transaction. Lock order is character, personal vault, account vault.
func (s *Store) CommitAccountVaultCrossMove(ctx context.Context, account, character int64, version, key string, space byte,
	apply func(Character, AccountVaultState, VaultState) (json.RawMessage, AccountVaultState, json.RawMessage, error),
) (Character, AccountVaultState, VaultState, bool, error) {
	var role Character
	var shared AccountVaultState
	var personal VaultState
	secondary, err := secondaryPersonalVault([]byte{space})
	if err != nil {
		return role, shared, personal, false, err
	}
	if apply == nil || account <= 0 || character <= 0 || len(version) != 64 || key == "" || len(key) > 200 {
		return role, shared, personal, false, errors.New("账号金库跨库事务参数无效")
	}
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return role, shared, personal, false, err
	}
	defer tx.Rollback(ctx)
	role, err = lockCharacter(ctx, tx, account, character)
	if err != nil {
		return role, shared, personal, false, err
	}
	if role.ConfigVersion != version {
		return role, shared, personal, false, errors.New("角色配置版本不匹配")
	}
	queries := s.queries.WithTx(tx)
	personal, err = lockPersonalVault(ctx, queries, character, secondary)
	if err != nil {
		return role, shared, personal, false, err
	}
	if err = queries.EnsureAccountVault(ctx, account); err != nil {
		return role, shared, personal, false, err
	}
	shared, err = lockSharedVault(ctx, queries, account)
	if err != nil {
		return role, shared, personal, false, err
	}
	prior, err := queries.AccountVaultEvent(ctx, sqlcgen.AccountVaultEventParams{AccountID: account, EventKey: key})
	if err == nil {
		if prior.CharacterID != character || prior.Operation != 19 {
			return role, shared, personal, false, errors.New("账号金库请求流水冲突")
		}
		return role, shared, personal, false, tx.Commit(ctx)
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return role, shared, personal, false, err
	}
	state, next, items, err := apply(role, shared, personal)
	if err != nil {
		return role, shared, personal, false, err
	}
	var fields map[string]json.RawMessage
	var arows, prows []json.RawMessage
	if json.Unmarshal(state, &fields) != nil || fields == nil || json.Unmarshal(next.Items, &arows) != nil || arows == nil || json.Unmarshal(items, &prows) != nil || prows == nil || next.Slots%8 != 0 || next.Slots > 320 || next.Gold > 800000000 || (next.Slots == 0 && (next.Gold != 0 || len(arows) != 0)) {
		return role, shared, personal, false, errors.New("账号金库跨库事务生成无效存档")
	}
	if err = sqlcgen.New(tx).UpdateCharacterState(ctx, sqlcgen.UpdateCharacterStateParams{CharacterID: character, State: state}); err != nil {
		return role, shared, personal, false, err
	}
	if err = savePersonalVaultItems(ctx, queries, character, items, secondary); err != nil {
		return role, shared, personal, false, err
	}
	if err = queries.SaveAccountVault(ctx, sqlcgen.SaveAccountVaultParams{AccountID: account, Slots: int32(next.Slots), Gold: int64(next.Gold), Items: next.Items}); err != nil {
		return role, shared, personal, false, err
	}
	if err = queries.RecordAccountVaultEvent(ctx, sqlcgen.RecordAccountVaultEventParams{AccountID: account, EventKey: key, CharacterID: character, Operation: 19}); err != nil {
		return role, shared, personal, false, err
	}
	if err = tx.Commit(ctx); err != nil {
		return role, shared, personal, false, err
	}
	role.State = state
	personal.Items = items
	return role, next, personal, true, nil
}
