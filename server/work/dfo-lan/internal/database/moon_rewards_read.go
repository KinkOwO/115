package database

import (
	"context"
	"dfolan/internal/database/sqlcgen"
)

// PendingMoonRewardRuns lists frozen rewards without a grant receipt in replay order.
func (s *Store) PendingMoonRewardRuns(ctx context.Context, characterID int64, model string) ([]string, error) {
	return s.queries.PendingMoonRewardRuns(ctx, sqlcgen.PendingMoonRewardRunsParams{CharacterID: characterID, Model: model})
}
