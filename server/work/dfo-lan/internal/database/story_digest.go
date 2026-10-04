package database

import (
	"context"
	"dfolan/internal/database/sqlcgen"
	"encoding/json"
	"fmt"
)

// AdvanceStoryDigest records the level at which the character finished the digest.
// The character row lock keeps concurrent reports from moving progress backwards.
func (s *Store) AdvanceStoryDigest(ctx context.Context, account, id int64, level uint32) (Character, bool, error) {
	var role Character
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return role, false, err
	}
	defer tx.Rollback(ctx)
	if role, err = lockCharacter(ctx, tx, account, id); err != nil {
		return role, false, err
	}
	state, advanced, err := advanceStoryDigestState(role.State, level)
	if err != nil {
		return role, false, err
	}
	if advanced {
		if err = sqlcgen.New(tx).UpdateCharacterState(ctx, sqlcgen.UpdateCharacterStateParams{CharacterID: id, State: state}); err != nil {
			return role, false, err
		}
		role.State = state
	}
	if err = tx.Commit(ctx); err != nil {
		return role, false, err
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
