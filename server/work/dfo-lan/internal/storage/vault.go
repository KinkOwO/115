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
      config_version text NOT NULL, updated_at timestamptz NOT NULL DEFAULT now());`)
	return e
}

// Initialization inserts only absent state; reconnect never resets stored items.
func (s *Store) LoadVault(ctx context.Context, account, id int64, initial uint16, version string) (VaultState, error) {
	var v VaultState
	if initial == 0 || len(version) != 64 {
		return v, errors.New("invalid vault source")
	}
	_, e := s.DB.Exec(ctx, `INSERT INTO character_vaults(character_id,slots,config_version)
      SELECT id,$3,$4 FROM characters WHERE id=$2 AND account_id=$1 ON CONFLICT DO NOTHING`, account, id, initial, version)
	if e != nil {
		return v, e
	}
	e = s.DB.QueryRow(ctx, `SELECT v.slots,v.items,v.config_version FROM character_vaults v JOIN characters c ON c.id=v.character_id WHERE c.id=$2 AND c.account_id=$1`, account, id).Scan(&v.Slots, &v.Items, &v.ConfigVersion)
	return v, e
}

// CommitVaultMove performs an atomic character bag + vault state modification
// under row locks on characters and character_vaults.
func (s *Store) CommitVaultMove(ctx context.Context, account, id int64,
	apply func(role Character, vault VaultState) (newRoleState json.RawMessage, newVaultItems json.RawMessage, err error)) (Character, VaultState, error) {
	var role Character
	var vault VaultState
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

	e = tx.QueryRow(ctx, `SELECT slots, items, config_version FROM character_vaults WHERE character_id=$1 FOR UPDATE`, id).Scan(&vault.Slots, &vault.Items, &vault.ConfigVersion)
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
	_, e = tx.Exec(ctx, `UPDATE character_vaults SET items=$2, updated_at=now() WHERE character_id=$1`, id, newVaultItems)
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
