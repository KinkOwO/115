package database

import (
	"context"
	charstate "dfolan/internal/character"
	"dfolan/internal/database/sqlcgen"
	"encoding/json"
)

func (s *Store) MigrateProfileSkins(ctx context.Context) error {
	return s.execMigration(ctx, "0009_profile_skins.sql")
}

// RestoreProfileSkins initializes a missing character snapshot, then reads it
// back in the same transaction. Existing selections are never reset on login.
// The role lock also serializes bootstrap with character deletion/mutations.
func (s *Store) RestoreProfileSkins(ctx context.Context, account, character int64) (charstate.ProfileSkinState, error) {
	var result charstate.ProfileSkinState
	tx, err := s.engine.begin(ctx)
	if err != nil {
		return result, err
	}
	defer tx.rollback(ctx)
	queries := tx.queries()
	if _, err = queries.LockCharacterOwner(ctx, sqlcgen.LockCharacterOwnerParams{AccountID: account, CharacterID: character}); err != nil {
		return result, err
	}
	seed, err := json.Marshal(charstate.ProfileSkinDefaults())
	if err != nil {
		return result, err
	}
	if err = queries.InitializeProfileSkins(ctx, sqlcgen.InitializeProfileSkinsParams{CharacterID: character, State: seed}); err != nil {
		return result, err
	}
	raw, err := queries.ProfileSkins(ctx, character)
	if err != nil {
		return result, err
	}
	if err = json.Unmarshal(raw, &result); err != nil {
		return charstate.ProfileSkinState{}, err
	}
	if err = result.Validate(); err != nil {
		return charstate.ProfileSkinState{}, err
	}
	if err = tx.commit(ctx); err != nil {
		return charstate.ProfileSkinState{}, err
	}
	return result, nil
}
