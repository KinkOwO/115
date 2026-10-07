package main

import (
	"context"
	"dfolan/internal/database"
	"encoding/json"
)

// gmStore is the persistence surface the GM dashboard consumes: the GM mail
// queue, read-only account and character projections, cera balances, grant
// history, and one atomic bag+vault move.
//
// The consumer declares the interface and the composition root injects the
// concrete store (docs/architecture-contract.md R3). It exists so the
// PostgreSQL/SQLite split stays behind internal/database.
type gmStore interface {
	Close()
	MigrateGMMail(context.Context) error
	Accounts(context.Context) ([]database.Account, error)
	AdminCharacters(context.Context, int64) ([]database.Character, error)
	AdminCharacter(context.Context, int64) (database.Character, error)
	AccountCera(context.Context, int64) (uint64, error)
	GrantHistory(context.Context, int64, int) ([]database.GrantHistoryEntry, error)
	GMMails(context.Context, int64, string) ([]database.GMMail, error)
	SendGMMail(context.Context, int64, int64, int64, int64, string, string) (int64, error)
	RevokeGMMail(context.Context, int64) error
	CommitVaultMove(context.Context, int64, int64, func(database.Character, database.VaultState) (json.RawMessage, json.RawMessage, error), ...byte) (database.Character, database.VaultState, error)
}
