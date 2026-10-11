package database

import (
	"context"
	"dfolan/internal/database/sqlcgen"
	"dfolan/internal/database/sqlcgensqlite"
)

// mainlineSkipQueries extends the shared generated surface with the local
// atomic skip operations. Both dialects are generated from mainline_skip.sql;
// these wrappers only bridge package-local parameter types.
type mainlineSkipQueries interface {
	HasMainlineSweep(context.Context, int64) (bool, error)
	SaveMainlineTicketOutcome(context.Context, sqlcgen.SaveMainlineTicketOutcomeParams) error
	UpsertMainlineSkippedQuest(context.Context, sqlcgen.UpsertMainlineSkippedQuestParams) error
}

func (q *sqliteQueries) SaveMainlineTicketOutcome(ctx context.Context, p sqlcgen.SaveMainlineTicketOutcomeParams) error {
	return storageError(q.Queries.SaveMainlineTicketOutcome(ctx, sqlcgensqlite.SaveMainlineTicketOutcomeParams(p)))
}
func (q *sqliteQueries) UpsertMainlineSkippedQuest(ctx context.Context, p sqlcgen.UpsertMainlineSkippedQuestParams) error {
	return storageError(q.Queries.UpsertMainlineSkippedQuest(ctx, sqlcgensqlite.UpsertMainlineSkippedQuestParams(p)))
}
