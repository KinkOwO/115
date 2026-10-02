package storage

import (
	"context"
	charstate "dfolan/internal/character"
	"encoding/json"
)

func (s *Store) MigrateProfileSkins(ctx context.Context) error {
	_, err := s.DB.Exec(ctx, `CREATE TABLE IF NOT EXISTS character_profile_skins (
 character_id bigint PRIMARY KEY REFERENCES characters(id),
 state jsonb NOT NULL,
 updated_at timestamptz NOT NULL DEFAULT now());`)
	return err
}

// RestoreProfileSkins initializes a missing character snapshot, then reads it
// back in the same transaction. Existing selections are never reset on login.
// The role lock also serializes bootstrap with character deletion/mutations.
func (s *Store) RestoreProfileSkins(ctx context.Context, account, character int64) (charstate.ProfileSkinState, error) {
	var result charstate.ProfileSkinState
	tx, err := s.DB.Begin(ctx)
	if err != nil {
		return result, err
	}
	defer tx.Rollback(ctx)
	var owned int64
	if err = tx.QueryRow(ctx, `SELECT id FROM characters WHERE account_id=$1 AND id=$2 AND deleted_at IS NULL FOR UPDATE`, account, character).Scan(&owned); err != nil {
		return result, err
	}
	seed, err := json.Marshal(charstate.ProfileSkinDefaults())
	if err != nil {
		return result, err
	}
	if _, err = tx.Exec(ctx, `INSERT INTO character_profile_skins(character_id,state) VALUES($1,$2) ON CONFLICT(character_id) DO NOTHING`, character, seed); err != nil {
		return result, err
	}
	var raw []byte
	if err = tx.QueryRow(ctx, `SELECT state FROM character_profile_skins WHERE character_id=$1`, character).Scan(&raw); err != nil {
		return result, err
	}
	if err = json.Unmarshal(raw, &result); err != nil {
		return charstate.ProfileSkinState{}, err
	}
	if err = result.Validate(); err != nil {
		return charstate.ProfileSkinState{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return charstate.ProfileSkinState{}, err
	}
	return result, nil
}
