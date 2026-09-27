package storage

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
)

// Account material storage keeps nineteen account-shared material
// stacks per account. The client pins seventeen to list 35 at 363..379
// and two radiant souls to list 42 at 0..1; see
// docs/protocol/next43-account-material-storage.md.
func (s *Store) MigrateAccountMaterials(ctx context.Context) error {
	_, e := s.DB.Exec(ctx, `CREATE TABLE IF NOT EXISTS account_material_storage (
 account_id bigint PRIMARY KEY REFERENCES accounts(id),
 counts jsonb NOT NULL DEFAULT '{}'::jsonb,
 updated_at timestamptz NOT NULL DEFAULT now());`)
	return e
}

// AccountMaterials returns the stored counts document, or NULL when the
// account never used the storage.
func (s *Store) AccountMaterials(ctx context.Context, account int64) (json.RawMessage, error) {
	var counts json.RawMessage
	e := s.DB.QueryRow(ctx, `SELECT counts FROM account_material_storage WHERE account_id=$1`, account).Scan(&counts)
	if e != nil {
		return nil, e
	}
	return counts, nil
}

// CommitAccountMaterialSweep atomically rewrites one character's state and
// the owning account's material counts. Unlike CommitCharacterEvent it takes
// no event key: a sweep is a pure function of current state (once the stacks
// move out of the bag nothing is left to sweep), so replaying it cannot
// double-apply.
func (s *Store) CommitAccountMaterialSweep(ctx context.Context, account, id int64, version string, apply func(Character, json.RawMessage) (json.RawMessage, json.RawMessage, error)) (Character, json.RawMessage, error) {
	var role Character
	decoded, e := hex.DecodeString(version)
	if e != nil || len(decoded) != 32 || apply == nil {
		return role, nil, fmt.Errorf("invalid account material sweep")
	}
	tx, e := s.DB.Begin(ctx)
	if e != nil {
		return role, nil, e
	}
	defer tx.Rollback(ctx)
	e = tx.QueryRow(ctx, `SELECT id,account_id,wire_id,name,profession,create_request,config_version,state,created_at FROM characters WHERE account_id=$1 AND id=$2 AND deleted_at IS NULL FOR UPDATE`, account, id).Scan(&role.ID, &role.AccountID, &role.WireID, &role.Name, &role.Profession, &role.Request, &role.ConfigVersion, &role.State, &role.CreatedAt)
	if e != nil {
		return role, nil, e
	}
	if role.ConfigVersion != version {
		return role, nil, fmt.Errorf("account material sweep source mismatch")
	}
	if _, e = tx.Exec(ctx, `INSERT INTO account_material_storage(account_id) VALUES($1) ON CONFLICT(account_id) DO NOTHING`, account); e != nil {
		return role, nil, e
	}
	var counts json.RawMessage
	if e = tx.QueryRow(ctx, `SELECT counts FROM account_material_storage WHERE account_id=$1 FOR UPDATE`, account).Scan(&counts); e != nil {
		return role, nil, e
	}
	state, updated, e := apply(role, counts)
	if e != nil {
		return role, nil, e
	}
	if !json.Valid(state) || !json.Valid(updated) {
		return role, nil, fmt.Errorf("invalid account material JSON")
	}
	if _, e = tx.Exec(ctx, `UPDATE characters SET state=$2 WHERE id=$1`, id, state); e != nil {
		return role, nil, e
	}
	if _, e = tx.Exec(ctx, `UPDATE account_material_storage SET counts=$2,updated_at=now() WHERE account_id=$1`, account, updated); e != nil {
		return role, nil, e
	}
	if e = tx.Commit(ctx); e != nil {
		return role, nil, e
	}
	role.State = state
	s.Cache.Del(ctx, fmt.Sprintf("%scharacters:%d", s.prefix, account))
	return role, updated, nil
}
