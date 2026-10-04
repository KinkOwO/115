package database

import (
	"context"
	"dfolan/internal/database/sqlcgen"
	"fmt"
)

// RunMonsterExperience reads the committed monster event total for one run.
func (s *Store) RunMonsterExperience(ctx context.Context, account, character int64, run string) (uint64, error) {
	total, err := s.queries.RunMonsterExperience(ctx, sqlcgen.RunMonsterExperienceParams{AccountID: account, CharacterID: character, EventPattern: "monster:" + run + ":%"})
	if err != nil {
		return 0, err
	}
	if total < 0 {
		return 0, fmt.Errorf("negative committed monster experience: %d", total)
	}
	return uint64(total), nil
}

// RunFatigueLedger returns charged fatigue and the first-loaded-room count.
func (s *Store) RunFatigueLedger(ctx context.Context, account, character int64, run string) (int64, int64, error) {
	row, err := s.queries.RunFatigueLedger(ctx, sqlcgen.RunFatigueLedgerParams{AccountID: account, CharacterID: character, RunID: run})
	return row.Charged, row.Rooms, err
}
