package storage

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/jackc/pgx/v5"
)

// CommitVaultTransfer locks character before vault, matching character writes.
// The callback validates and pre-encodes replies before either state is saved.
// Replay returns current states; it never reinstalls an earlier bag snapshot.
func (s *Store) CommitVaultTransfer(ctx context.Context, account, id int64, source, vaultVersion, key string, request []byte,
	apply func(Character, VaultState) (json.RawMessage, json.RawMessage, error)) (Character, VaultState, bool, error) {
	var role Character
	var v VaultState
	fail := func(e error) (Character, VaultState, bool, error) { return Character{}, VaultState{}, false, e }
	a, e := hex.DecodeString(source)
	b, f := hex.DecodeString(vaultVersion)
	if e != nil || f != nil || len(a) != 32 || len(b) != 32 || account <= 0 || id <= 0 || len(key) == 0 || len(key) > 200 || len(request) == 0 || apply == nil {
		return fail(fmt.Errorf("invalid vault transfer"))
	}
	tx, e := s.DB.Begin(ctx)
	if e != nil {
		return fail(e)
	}
	defer tx.Rollback(ctx)
	e = tx.QueryRow(ctx, `SELECT id,account_id,wire_id,name,profession,create_request,config_version,state,created_at FROM characters WHERE id=$1 AND account_id=$2 AND deleted_at IS NULL FOR UPDATE`, id, account).Scan(&role.ID, &role.AccountID, &role.WireID, &role.Name, &role.Profession, &role.Request, &role.ConfigVersion, &role.State, &role.CreatedAt)
	if e != nil {
		return fail(e)
	}
	e = tx.QueryRow(ctx, `SELECT slots,items,config_version FROM character_vaults WHERE character_id=$1 FOR UPDATE`, id).Scan(&v.Slots, &v.Items, &v.ConfigVersion)
	if e != nil {
		return fail(e)
	}
	if role.ConfigVersion != source || v.ConfigVersion != vaultVersion {
		return fail(fmt.Errorf("vault transfer source mismatch"))
	}
	model := fmt.Sprintf("vault-stack-v1:%x", sha256.Sum256(request))
	var prior string
	e = tx.QueryRow(ctx, `SELECT model FROM character_events WHERE character_id=$1 AND event_key=$2`, id, key).Scan(&prior)
	if e == nil {
		if prior != model {
			return fail(fmt.Errorf("vault transfer replay conflict"))
		}
		if e = tx.Commit(ctx); e != nil {
			return fail(e)
		}
		return role, v, false, nil
	}
	if !errors.Is(e, pgx.ErrNoRows) {
		return fail(e)
	}
	state, items, e := apply(role, v)
	if e != nil {
		return fail(e)
	}
	var obj map[string]json.RawMessage
	var rows []json.RawMessage
	if json.Unmarshal(state, &obj) != nil || obj == nil || json.Unmarshal(items, &rows) != nil || rows == nil {
		return fail(fmt.Errorf("invalid vault transfer state"))
	}
	if _, e = tx.Exec(ctx, `UPDATE characters SET state=$2 WHERE id=$1`, id, state); e != nil {
		return fail(e)
	}
	if _, e = tx.Exec(ctx, `UPDATE character_vaults SET items=$2,updated_at=now() WHERE character_id=$1`, id, items); e != nil {
		return fail(e)
	}
	if _, e = tx.Exec(ctx, `INSERT INTO character_events(character_id,event_key,config_version,model,outcome) VALUES($1,$2,$3,$4,'{}')`, id, key, source, model); e != nil {
		return fail(e)
	}
	if e = tx.Commit(ctx); e != nil {
		return fail(e)
	}
	role.State = state
	v.Items = items
	return role, v, true, nil
}
