package storage

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
)

// CommitAccountMaterialEvent applies an account-scoped material change exactly
// once.
//
// CommitAccountMaterialSweep cannot serve a spend: it is safe to retry only
// because it is state-derived - once swept there is nothing left to move - while
// replaying a spend would deduct the same cubes twice. This variant therefore
// carries the same receipt CommitCharacterEvent uses, keyed by the acting
// character, and locks the account material row alongside the character so two
// characters on one account cannot interleave.
//
// The returned bool reports whether the change was applied (false on an
// idempotent replay). The returned counts are always the post-commit storage
// content, so a replay can still answer the client authoritatively.
func (s *Store) CommitAccountMaterialEvent(ctx context.Context, account, id int64, version, key, model string, apply func(Character, json.RawMessage) (json.RawMessage, json.RawMessage, error)) (Character, json.RawMessage, bool, error) {
	var role Character
	decoded, e := hex.DecodeString(version)
	if e != nil || len(decoded) != 32 || key == "" || len(key) > 200 || model == "" || len(model) > 100 || apply == nil {
		return role, nil, false, fmt.Errorf("invalid account material event")
	}
	tx, e := s.DB.Begin(ctx)
	if e != nil {
		return role, nil, false, e
	}
	defer tx.Rollback(ctx)
	e = tx.QueryRow(ctx, `SELECT id,account_id,wire_id,name,profession,create_request,config_version,state,created_at FROM characters WHERE account_id=$1 AND id=$2 AND deleted_at IS NULL FOR UPDATE`, account, id).Scan(&role.ID, &role.AccountID, &role.WireID, &role.Name, &role.Profession, &role.Request, &role.ConfigVersion, &role.State, &role.CreatedAt)
	if e != nil {
		return role, nil, false, e
	}
	if role.ConfigVersion != version {
		return role, nil, false, fmt.Errorf("account material event source mismatch")
	}
	if _, e = tx.Exec(ctx, `INSERT INTO account_material_storage(account_id) VALUES($1) ON CONFLICT(account_id) DO NOTHING`, account); e != nil {
		return role, nil, false, e
	}
	var counts json.RawMessage
	if e = tx.QueryRow(ctx, `SELECT counts FROM account_material_storage WHERE account_id=$1 FOR UPDATE`, account).Scan(&counts); e != nil {
		return role, nil, false, e
	}
	var prior string
	e = tx.QueryRow(ctx, `SELECT model FROM character_events WHERE character_id=$1 AND event_key=$2`, id, key).Scan(&prior)
	if e == nil {
		if prior != model {
			return role, nil, false, fmt.Errorf("account material event model mismatch")
		}
		// Replayed request: the deduction already happened. Hand back the state
		// as it stands so the caller can still acknowledge with real counts.
		return role, counts, false, tx.Commit(ctx)
	}
	if !errors.Is(e, pgx.ErrNoRows) {
		return role, nil, false, e
	}
	state, updated, e := apply(role, counts)
	if e != nil {
		return role, nil, false, e
	}
	if !json.Valid(state) || !json.Valid(updated) {
		return role, nil, false, fmt.Errorf("invalid account material JSON")
	}
	if _, e = tx.Exec(ctx, `INSERT INTO character_events(character_id,event_key,config_version,model,outcome) VALUES($1,$2,$3,$4,$5)`, id, key, version, model, updated); e != nil {
		return role, nil, false, e
	}
	if _, e = tx.Exec(ctx, `UPDATE characters SET state=$2 WHERE id=$1`, id, state); e != nil {
		return role, nil, false, e
	}
	if _, e = tx.Exec(ctx, `UPDATE account_material_storage SET counts=$2,updated_at=now() WHERE account_id=$1`, account, updated); e != nil {
		return role, nil, false, e
	}
	if e = tx.Commit(ctx); e != nil {
		return role, nil, false, e
	}
	role.State = state
	s.Cache.Del(ctx, fmt.Sprintf("%scharacters:%d", s.prefix, account))
	return role, updated, true, nil
}
