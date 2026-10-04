package database

import (
	"context"
	"dfolan/internal/database/sqlcgen"
	"fmt"
)

// Birth stages for a character's one-time starting route.
const (
	BirthPending  = 0 // created, has not entered its job tutorial
	BirthEntered  = 1 // inside the tutorial dungeon
	BirthReturned = 2 // tutorial finished, not yet settled in town
	BirthComplete = 3 // finished or intentionally skipped; ordinary play
)

// MigrateBirth creates the starting-route table and backfills every character
// that already exists as complete. Characters made before this build have
// already been played past their starting route, and the project's standing
// rule is that existing progress is never reset or replayed: only characters
// created from here on begin at BirthPending.
func (s *Store) MigrateBirth(ctx context.Context) error {
	if err := s.execMigration(ctx, "0020_birth.sql"); err != nil {
		return err
	}
	// Keep the existing backfill on each initialization; recorded stages survive.
	return s.queries.BackfillBirth(ctx, BirthComplete)
}

// BirthStage reports a character's starting-route stage. A character without
// a row is treated as complete: an absent row can only mean the character
// predates this table, and such a character must never be sent back through
// a starting route it already finished.
func (s *Store) BirthStage(ctx context.Context, account, id int64) (byte, uint32, error) {
	row, e := s.queries.BirthStage(ctx, sqlcgen.BirthStageParams{AccountID: account, CharacterID: id})
	if e != nil {
		owned, q := s.queries.ActiveCharacterOwned(ctx, sqlcgen.ActiveCharacterOwnedParams{AccountID: account, CharacterID: id})
		if q != nil {
			return BirthComplete, 0, q
		}
		if !owned {
			return BirthComplete, 0, fmt.Errorf("starting route character is not owned")
		}
		return BirthComplete, 0, nil
	}
	return byte(row.Stage), uint32(row.Dungeon), nil
}

// StartBirth records a newly created character as owing its starting route.
func (s *Store) StartBirth(ctx context.Context, account, id int64) error {
	changed, e := s.queries.StartBirth(ctx, sqlcgen.StartBirthParams{AccountID: account, CharacterID: id, Stage: BirthPending})
	if e == nil && changed != 1 {
		return fmt.Errorf("starting route already recorded")
	}
	return e
}

// AdvanceBirth moves a character's starting route forward. The transition is
// one-way and refuses to move backwards, so a reconnect or a replayed request
// can never send a character through its starting route a second time.
func (s *Store) AdvanceBirth(ctx context.Context, account, id int64, stage byte, dungeon uint32) (bool, error) {
	if stage > BirthComplete {
		return false, fmt.Errorf("invalid starting route stage")
	}
	changed, e := s.queries.AdvanceBirth(ctx, sqlcgen.AdvanceBirthParams{AccountID: account, CharacterID: id, Stage: int16(stage), Dungeon: int64(dungeon)})
	if e != nil {
		return false, e
	}
	return changed == 1, nil
}
