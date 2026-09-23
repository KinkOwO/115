package storage

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
)

type VaultState struct {
	Slots         uint16
	Items         json.RawMessage
	ConfigVersion string
}

func (s *Store) MigrateVault(ctx context.Context) error {
	_, e := s.DB.Exec(ctx, `CREATE TABLE IF NOT EXISTS character_vaults (
      character_id bigint PRIMARY KEY REFERENCES characters(id),
      slots integer NOT NULL CHECK(slots BETWEEN 1 AND 65535),
      items jsonb NOT NULL DEFAULT '[]' CHECK(jsonb_typeof(items)='array'),
      config_version text NOT NULL, updated_at timestamptz NOT NULL DEFAULT now());
CREATE TABLE IF NOT EXISTS character_secondary_vaults (
      character_id bigint PRIMARY KEY REFERENCES characters(id),
      slots integer NOT NULL CHECK(slots BETWEEN 1 AND 65535),
      items jsonb NOT NULL DEFAULT '[]' CHECK(jsonb_typeof(items)='array'),
      config_version text NOT NULL, updated_at timestamptz NOT NULL DEFAULT now());`)
	return e
}

// 表名只取自固定容器映射；缺省保持金库 1，旧调用和旧存档均不迁移。
func personalVaultTable(space []byte) (string, error) {
	if len(space) == 0 || (len(space) == 1 && space[0] == 2) {
		return "character_vaults", nil
	}
	if len(space) == 1 && space[0] == 45 {
		return "character_secondary_vaults", nil
	}
	return "", errors.New("个人金库容器无效")
}

// Initialization inserts only absent state; reconnect never resets stored items.
func (s *Store) LoadVault(ctx context.Context, account, id int64, initial uint16, version string, space ...byte) (VaultState, error) {
	var v VaultState
	table, e := personalVaultTable(space)
	if e != nil {
		return v, e
	}
	if initial == 0 || len(version) != 64 {
		return v, errors.New("invalid vault source")
	}
	_, e = s.DB.Exec(ctx, `INSERT INTO `+table+`(character_id,slots,config_version)
      SELECT id,$3,$4 FROM characters WHERE id=$2 AND account_id=$1 AND deleted_at IS NULL ON CONFLICT DO NOTHING`, account, id, initial, version)
	if e != nil {
		return v, e
	}
	e = s.DB.QueryRow(ctx, `SELECT v.slots,v.items,v.config_version FROM `+table+` v JOIN characters c ON c.id=v.character_id WHERE c.id=$2 AND c.account_id=$1 AND c.deleted_at IS NULL`, account, id).Scan(&v.Slots, &v.Items, &v.ConfigVersion)
	return v, e
}

// CommitVaultMove performs an atomic character bag + vault state modification
// under row locks on characters and character_vaults.
func (s *Store) CommitVaultMove(ctx context.Context, account, id int64,
	apply func(role Character, vault VaultState) (newRoleState json.RawMessage, newVaultItems json.RawMessage, err error), space ...byte) (Character, VaultState, error) {
	var role Character
	var vault VaultState
	table, e := personalVaultTable(space)
	if e != nil {
		return role, vault, e
	}
	if apply == nil {
		return role, vault, errors.New("nil vault move apply function")
	}
	tx, e := s.DB.Begin(ctx)
	if e != nil {
		return role, vault, e
	}
	defer tx.Rollback(ctx)

	e = tx.QueryRow(ctx, `SELECT id,account_id,wire_id,name,profession,create_request,config_version,state,created_at FROM characters WHERE account_id=$1 AND id=$2 AND deleted_at IS NULL FOR UPDATE`, account, id).Scan(&role.ID, &role.AccountID, &role.WireID, &role.Name, &role.Profession, &role.Request, &role.ConfigVersion, &role.State, &role.CreatedAt)
	if e != nil {
		return role, vault, e
	}

	e = tx.QueryRow(ctx, `SELECT slots, items, config_version FROM `+table+` WHERE character_id=$1 FOR UPDATE`, id).Scan(&vault.Slots, &vault.Items, &vault.ConfigVersion)
	if e != nil {
		return role, vault, e
	}

	newRoleState, newVaultItems, e := apply(role, vault)
	if e != nil {
		return role, vault, e
	}
	if !json.Valid(newRoleState) || !json.Valid(newVaultItems) {
		return role, vault, errors.New("invalid JSON state in vault move")
	}

	_, e = tx.Exec(ctx, `UPDATE characters SET state=$2 WHERE id=$1`, id, newRoleState)
	if e != nil {
		return role, vault, e
	}
	_, e = tx.Exec(ctx, `UPDATE `+table+` SET items=$2, updated_at=now() WHERE character_id=$1`, id, newVaultItems)
	if e != nil {
		return role, vault, e
	}

	if e = tx.Commit(ctx); e != nil {
		return role, vault, e
	}

	role.State = newRoleState
	vault.Items = newVaultItems
	s.Cache.Del(ctx, fmt.Sprintf("%scharacters:%d", s.prefix, account))
	return role, vault, nil
}
