package storage

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/jackc/pgx/v5"
)

func (s *Store) MigrateCharacterEvents(ctx context.Context) error {
	_, e := s.DB.Exec(ctx, `CREATE TABLE IF NOT EXISTS character_events (
 character_id bigint NOT NULL REFERENCES characters(id), event_key text NOT NULL,
 config_version text NOT NULL, model text NOT NULL, outcome jsonb NOT NULL,
 created_at timestamptz NOT NULL DEFAULT now(), PRIMARY KEY(character_id,event_key));`)
	return e
}

func (s *Store) CharacterEventReceipt(ctx context.Context, account, id int64, key string) (json.RawMessage, error) {
	var p json.RawMessage
	e := s.DB.QueryRow(ctx, `SELECT e.outcome FROM character_events e JOIN characters c ON c.id=e.character_id WHERE c.account_id=$1 AND c.id=$2 AND e.event_key=$3`, account, id, key).Scan(&p)
	return p, e
}

// CommitCharacterEvent owns atomic state/receipt persistence. Its callback is
// pure domain code and runs only under the owning character's PostgreSQL lock.
// Retries never recompute a reward using a later level or changed rules.
func (s *Store) CommitCharacterEvent(ctx context.Context, account, id int64, version, key, model string, apply func(Character) (json.RawMessage, json.RawMessage, error)) (Character, bool, error) {
	var role Character
	decoded, e := hex.DecodeString(version)
	if e != nil || len(decoded) != 32 || key == "" || len(key) > 200 || model == "" || len(model) > 100 || apply == nil {
		return role, false, fmt.Errorf("invalid character event")
	}
	tx, e := s.DB.Begin(ctx)
	if e != nil {
		return role, false, e
	}
	defer tx.Rollback(ctx)
	e = tx.QueryRow(ctx, `SELECT id,account_id,wire_id,name,profession,create_request,config_version,state,created_at FROM characters WHERE account_id=$1 AND id=$2 AND deleted_at IS NULL FOR UPDATE`, account, id).Scan(&role.ID, &role.AccountID, &role.WireID, &role.Name, &role.Profession, &role.Request, &role.ConfigVersion, &role.State, &role.CreatedAt)
	if e != nil {
		return role, false, e
	}
	if role.ConfigVersion != version {
		return role, false, fmt.Errorf("character event source mismatch")
	}
	var prior string
	e = tx.QueryRow(ctx, `SELECT model FROM character_events WHERE character_id=$1 AND event_key=$2`, id, key).Scan(&prior)
	if e == nil {
		if prior != model {
			return role, false, fmt.Errorf("character event model mismatch")
		}
		return role, false, tx.Commit(ctx)
	}
	if !errors.Is(e, pgx.ErrNoRows) {
		return role, false, e
	}
	state, outcome, e := apply(role)
	if e != nil {
		return role, false, e
	}
	if !json.Valid(state) || !json.Valid(outcome) {
		return role, false, fmt.Errorf("invalid event JSON")
	}
	_, e = tx.Exec(ctx, `INSERT INTO character_events(character_id,event_key,config_version,model,outcome) VALUES($1,$2,$3,$4,$5)`, id, key, version, model, outcome)
	if e != nil {
		return role, false, e
	}
	_, e = tx.Exec(ctx, `UPDATE characters SET state=$2 WHERE id=$1`, id, state)
	if e != nil {
		return role, false, e
	}
	if e = tx.Commit(ctx); e != nil {
		return role, false, e
	}
	role.State = state
	s.Cache.Del(ctx, fmt.Sprintf("%scharacters:%d", s.prefix, account))
	return role, true, nil
}
