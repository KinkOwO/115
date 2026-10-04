package database

import (
	"context"
	"dfolan/internal/database/sqlcgen"
	"encoding/hex"
	"encoding/json"
	"fmt"
)

// Account material storage keeps nineteen account-shared material
// stacks per account. The client pins seventeen to list 35 at 363..379
// and two radiant souls to list 42 at 0..1; see
// docs/protocol/next43-account-material-storage.md.
func (s *Store) MigrateAccountMaterials(ctx context.Context) error {
	return s.execMigration(ctx, "0017_account_materials.sql")
}

// AccountMaterials returns the stored counts document, or NULL when the
// account never used the storage.
func (s *Store) AccountMaterials(ctx context.Context, account int64) (json.RawMessage, error) {
	return s.queries.AccountMaterials(ctx, account)
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
	tx, e := s.db.Begin(ctx)
	if e != nil {
		return role, nil, e
	}
	defer tx.Rollback(ctx)
	role, e = lockCharacter(ctx, tx, account, id)
	if e != nil {
		return role, nil, e
	}
	if role.ConfigVersion != version {
		return role, nil, fmt.Errorf("account material sweep source mismatch")
	}
	if e = sqlcgen.New(tx).InitializeAccountMaterials(ctx, account); e != nil {
		return role, nil, e
	}
	counts, e := sqlcgen.New(tx).LockAccountMaterials(ctx, account)
	if e != nil {
		return role, nil, e
	}
	state, updated, e := apply(role, counts)
	if e != nil {
		return role, nil, e
	}
	if !json.Valid(state) || !json.Valid(updated) {
		return role, nil, fmt.Errorf("invalid account material JSON")
	}
	if e = sqlcgen.New(tx).UpdateCharacterState(ctx, sqlcgen.UpdateCharacterStateParams{CharacterID: id, State: state}); e != nil {
		return role, nil, e
	}
	if e = sqlcgen.New(tx).SaveAccountMaterials(ctx, sqlcgen.SaveAccountMaterialsParams{AccountID: account, Counts: updated}); e != nil {
		return role, nil, e
	}
	if e = tx.Commit(ctx); e != nil {
		return role, nil, e
	}
	role.State = state
	return role, updated, nil
}
