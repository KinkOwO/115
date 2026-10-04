package database

import (
	"context"
	"dfolan/internal/database/sqlcgen"
	"fmt"
)

func (s *Store) MigrateTutorial(ctx context.Context) error {
	return s.execMigration(ctx, "0004_tutorial.sql")
}
func (s *Store) SaveTutorialFlag(ctx context.Context, account, id int64, index byte, completed bool) error {
	if index >= 101 {
		return fmt.Errorf("tutorial index out of range")
	}
	count, e := s.queries.SaveTutorialFlag(ctx, sqlcgen.SaveTutorialFlagParams{
		AccountID: account, CharacterID: id, Flag: int32(index), Completed: completed,
	})
	if e == nil && count != 1 {
		return fmt.Errorf("tutorial character is not owned")
	}
	return e
}
func (s *Store) TutorialFlags(ctx context.Context, account, id int64) ([]byte, error) {
	owned, e := s.queries.CharacterOwned(ctx, sqlcgen.CharacterOwnedParams{AccountID: account, CharacterID: id})
	if e != nil {
		return nil, e
	}
	if !owned {
		return nil, fmt.Errorf("tutorial character is not owned")
	}
	flags, e := s.queries.TutorialFlags(ctx, id)
	if e != nil {
		return nil, e
	}
	out := make([]byte, len(flags))
	for i, flag := range flags {
		out[i] = byte(flag)
	}
	return out, nil
}
