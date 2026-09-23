package storage

import (
	"context"
	"encoding/json"
	"fmt"
)

// AdvanceStoryDigest records the level at which the character finished the digest.
// The character row lock keeps concurrent reports from moving progress backwards.
func (s *Store) AdvanceStoryDigest(ctx context.Context, account, id int64, level uint32) (Character, bool, error) {
	var role Character
	tx, err := s.DB.Begin(ctx)
	if err != nil {
		return role, false, err
	}
	defer tx.Rollback(ctx)
	if err = tx.QueryRow(ctx, `SELECT id,account_id,wire_id,name,profession,create_request,config_version,state,created_at FROM characters WHERE account_id=$1 AND id=$2 AND deleted_at IS NULL FOR UPDATE`, account, id).Scan(&role.ID, &role.AccountID, &role.WireID, &role.Name, &role.Profession, &role.Request, &role.ConfigVersion, &role.State, &role.CreatedAt); err != nil {
		return role, false, err
	}
	state, advanced, err := advanceStoryDigestState(role.State, level)
	if err != nil {
		return role, false, err
	}
	if advanced {
		if _, err = tx.Exec(ctx, `UPDATE characters SET state=$2 WHERE id=$1`, id, state); err != nil {
			return role, false, err
		}
		role.State = state
	}
	if err = tx.Commit(ctx); err != nil {
		return role, false, err
	}
	if advanced {
		s.Cache.Del(ctx, fmt.Sprintf("%scharacters:%d", s.prefix, account))
	}
	return role, advanced, nil
}

func advanceStoryDigestState(raw json.RawMessage, level uint32) (json.RawMessage, bool, error) {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil {
		return nil, false, err
	}
	if fields == nil {
		return nil, false, fmt.Errorf("character state is not an object")
	}
	var current uint32
	if value, ok := fields["story_digest_level"]; ok {
		if err := json.Unmarshal(value, &current); err != nil {
			return nil, false, err
		}
	}
	if level <= current {
		return raw, false, nil
	}
	fields["story_digest_level"], _ = json.Marshal(level)
	updated, err := json.Marshal(fields)
	return updated, err == nil, err
}
