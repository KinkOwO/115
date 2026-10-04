package database

import (
	"context"
	"encoding/json"

	"dfolan/internal/database/sqlcgen"
)

// ReadPendingBlackPurgatoryRewards visits frozen card plans which do not yet
// have their recovery marker. The callback keeps decoding in the workflow
// while storage reads at most 16 plans before invoking callbacks in creation
// order. The database connection is released before recovery writes begin.
func (s *Store) ReadPendingBlackPurgatoryRewards(ctx context.Context, account, character int64, model string, consume func(json.RawMessage) error) error {
	plans, err := s.queries.PendingBlackPurgatoryRewards(ctx, sqlcgen.PendingBlackPurgatoryRewardsParams{AccountID: account, CharacterID: character, Model: model})
	if err != nil {
		return err
	}
	for _, raw := range plans {
		if err = consume(raw); err != nil {
			return err
		}
	}
	return nil
}
