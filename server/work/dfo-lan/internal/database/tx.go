package database

import (
	"dfolan/internal/database/sqlcgen"

	"github.com/jackc/pgx/v5"
)

// Tx offers only the persistence operations needed by a transaction callback.
// SQL, generated types, driver handles and transaction completion stay private.
// A callback cannot commit independently of its character save and receipt.
type Tx struct {
	tx          pgx.Tx
	queries     *sqlcgen.Queries
	accountID   int64
	characterID int64
}

func newTx(tx pgx.Tx, account, character int64) *Tx {
	return &Tx{tx: tx, queries: sqlcgen.New(tx), accountID: account, characterID: character}
}
