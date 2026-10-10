package quest

import "errors"

// Acceptance diagnostics preserve the existing refusal semantics. In particular,
// a completed quest's graduation model is not an active objective migration.
var (
	ErrAlreadyCompleted          = errors.New("quest is already completed")
	ErrSourceMigrationRequired   = errors.New("quest requires source configuration migration")
	ErrProgressMigrationRequired = errors.New("quest requires progress model migration")
)
