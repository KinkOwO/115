package storage

import (
	"context"
	"dfolan/internal/inventory"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
)

type AccountVaultState = inventory.AccountVaultState

func (s *Store) MigrateAccountVault(ctx context.Context) error {
	_, err := s.DB.Exec(ctx, `CREATE TABLE IF NOT EXISTS account_vaults (
 account_id bigint PRIMARY KEY REFERENCES accounts(id),
 slots integer NOT NULL DEFAULT 0 CHECK(slots BETWEEN 0 AND 320 AND slots%8=0),
 gold bigint NOT NULL DEFAULT 0 CHECK(gold BETWEEN 0 AND 800000000),
 items jsonb NOT NULL DEFAULT '[]' CHECK(jsonb_typeof(items)='array'),
 updated_at timestamptz NOT NULL DEFAULT now());
CREATE TABLE IF NOT EXISTS account_vault_events (
 account_id bigint NOT NULL REFERENCES accounts(id), event_key text NOT NULL,
 character_id bigint NOT NULL REFERENCES characters(id), operation integer NOT NULL,
 created_at timestamptz NOT NULL DEFAULT now(), PRIMARY KEY(account_id,event_key));`)
	return err
}

func (s *Store) LoadAccountVault(ctx context.Context, account, character int64) (AccountVaultState, error) {
	var v AccountVaultState
	// 不存在的金库只是返回未开通状态，登录本身不会免费创建。
	err := s.DB.QueryRow(ctx, `SELECT coalesce(v.slots,0),coalesce(v.gold,0),coalesce(v.items,'[]'::jsonb)
 FROM characters c LEFT JOIN account_vaults v ON v.account_id=c.account_id
 WHERE c.id=$2 AND c.account_id=$1 AND c.deleted_at IS NULL`, account, character).Scan(&v.Slots, &v.Gold, &v.Items)
	return v, err
}

func (s *Store) CommitAccountVaultSort(ctx context.Context, account, character int64, sortItems func(AccountVaultState) (json.RawMessage, error)) (AccountVaultState, error) {
	var vault AccountVaultState
	if sortItems == nil {
		return vault, errors.New("nil account vault sort")
	}
	tx, err := s.DB.Begin(ctx)
	if err != nil {
		return vault, err
	}
	defer tx.Rollback(ctx)
	var owned bool
	if err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM characters WHERE id=$2 AND account_id=$1 AND deleted_at IS NULL)`, account, character).Scan(&owned); err != nil {
		return vault, err
	}
	if !owned {
		return vault, errors.New("角色不属于账号")
	}
	if _, err = tx.Exec(ctx, `INSERT INTO account_vaults(account_id) VALUES($1) ON CONFLICT DO NOTHING`, account); err != nil {
		return vault, err
	}
	if err = tx.QueryRow(ctx, `SELECT slots,gold,items FROM account_vaults WHERE account_id=$1 FOR UPDATE`, account).Scan(&vault.Slots, &vault.Gold, &vault.Items); err != nil {
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
	if _, err = tx.Exec(ctx, `UPDATE account_vaults SET items=$2,updated_at=now() WHERE account_id=$1`, account, items); err != nil {
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
	tx, err := s.DB.Begin(ctx)
	if err != nil {
		return role, materials, vault, false, err
	}
	defer tx.Rollback(ctx)
	err = tx.QueryRow(ctx, `SELECT id,account_id,wire_id,name,profession,create_request,config_version,state,created_at FROM characters WHERE account_id=$1 AND id=$2 AND deleted_at IS NULL FOR UPDATE`, account, character).Scan(&role.ID, &role.AccountID, &role.WireID, &role.Name, &role.Profession, &role.Request, &role.ConfigVersion, &role.State, &role.CreatedAt)
	if err != nil {
		return role, materials, vault, false, err
	}
	if role.ConfigVersion != version {
		return role, materials, vault, false, fmt.Errorf("账号金库角色配置版本不匹配")
	}
	if _, err = tx.Exec(ctx, `INSERT INTO account_material_storage(account_id) VALUES($1) ON CONFLICT DO NOTHING`, account); err != nil {
		return role, materials, vault, false, err
	}
	if err = tx.QueryRow(ctx, `SELECT counts FROM account_material_storage WHERE account_id=$1 FOR UPDATE`, account).Scan(&materials); err != nil {
		return role, materials, vault, false, err
	}
	if _, err = tx.Exec(ctx, `INSERT INTO account_vaults(account_id) VALUES($1) ON CONFLICT DO NOTHING`, account); err != nil {
		return role, materials, vault, false, err
	}
	if err = tx.QueryRow(ctx, `SELECT slots,gold,items FROM account_vaults WHERE account_id=$1 FOR UPDATE`, account).Scan(&vault.Slots, &vault.Gold, &vault.Items); err != nil {
		return role, materials, vault, false, err
	}
	var priorCharacter int64
	var priorOperation uint16
	err = tx.QueryRow(ctx, `SELECT character_id,operation FROM account_vault_events WHERE account_id=$1 AND event_key=$2`, account, key).Scan(&priorCharacter, &priorOperation)
	if err == nil {
		if priorCharacter != character || priorOperation != operation {
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
	if _, err = tx.Exec(ctx, `UPDATE characters SET state=$2 WHERE id=$1`, character, state); err != nil {
		return role, materials, vault, false, err
	}
	if _, err = tx.Exec(ctx, `UPDATE account_material_storage SET counts=$2,updated_at=now() WHERE account_id=$1`, account, updated); err != nil {
		return role, materials, vault, false, err
	}
	if _, err = tx.Exec(ctx, `UPDATE account_vaults SET slots=$2,gold=$3,items=$4,updated_at=now() WHERE account_id=$1`, account, next.Slots, next.Gold, next.Items); err != nil {
		return role, materials, vault, false, err
	}
	if _, err = tx.Exec(ctx, `INSERT INTO account_vault_events(account_id,event_key,character_id,operation) VALUES($1,$2,$3,$4)`, account, key, character, operation); err != nil {
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
	table, err := personalVaultTable([]byte{space})
	if err != nil {
		return role, shared, personal, false, err
	}
	if apply == nil || account <= 0 || character <= 0 || len(version) != 64 || key == "" || len(key) > 200 {
		return role, shared, personal, false, errors.New("账号金库跨库事务参数无效")
	}
	tx, err := s.DB.Begin(ctx)
	if err != nil {
		return role, shared, personal, false, err
	}
	defer tx.Rollback(ctx)
	err = tx.QueryRow(ctx, `SELECT id,account_id,wire_id,name,profession,create_request,config_version,state,created_at FROM characters WHERE account_id=$1 AND id=$2 AND deleted_at IS NULL FOR UPDATE`, account, character).Scan(&role.ID, &role.AccountID, &role.WireID, &role.Name, &role.Profession, &role.Request, &role.ConfigVersion, &role.State, &role.CreatedAt)
	if err != nil {
		return role, shared, personal, false, err
	}
	if role.ConfigVersion != version {
		return role, shared, personal, false, errors.New("角色配置版本不匹配")
	}
	err = tx.QueryRow(ctx, `SELECT slots,items,config_version FROM `+table+` WHERE character_id=$1 FOR UPDATE`, character).Scan(&personal.Slots, &personal.Items, &personal.ConfigVersion)
	if err != nil {
		return role, shared, personal, false, err
	}
	if _, err = tx.Exec(ctx, `INSERT INTO account_vaults(account_id) VALUES($1) ON CONFLICT DO NOTHING`, account); err != nil {
		return role, shared, personal, false, err
	}
	err = tx.QueryRow(ctx, `SELECT slots,gold,items FROM account_vaults WHERE account_id=$1 FOR UPDATE`, account).Scan(&shared.Slots, &shared.Gold, &shared.Items)
	if err != nil {
		return role, shared, personal, false, err
	}
	var priorCharacter int64
	var priorOperation uint16
	err = tx.QueryRow(ctx, `SELECT character_id,operation FROM account_vault_events WHERE account_id=$1 AND event_key=$2`, account, key).Scan(&priorCharacter, &priorOperation)
	if err == nil {
		if priorCharacter != character || priorOperation != 19 {
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
	if _, err = tx.Exec(ctx, `UPDATE characters SET state=$2 WHERE id=$1`, character, state); err != nil {
		return role, shared, personal, false, err
	}
	if _, err = tx.Exec(ctx, `UPDATE `+table+` SET items=$2,updated_at=now() WHERE character_id=$1`, character, items); err != nil {
		return role, shared, personal, false, err
	}
	if _, err = tx.Exec(ctx, `UPDATE account_vaults SET slots=$2,gold=$3,items=$4,updated_at=now() WHERE account_id=$1`, account, next.Slots, next.Gold, next.Items); err != nil {
		return role, shared, personal, false, err
	}
	if _, err = tx.Exec(ctx, `INSERT INTO account_vault_events(account_id,event_key,character_id,operation) VALUES($1,$2,$3,19)`, account, key, character); err != nil {
		return role, shared, personal, false, err
	}
	if err = tx.Commit(ctx); err != nil {
		return role, shared, personal, false, err
	}
	role.State = state
	personal.Items = items
	return role, next, personal, true, nil
}
