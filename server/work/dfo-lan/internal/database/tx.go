package database

// Tx offers only the persistence operations needed by a transaction callback.
// SQL, generated types, driver handles and transaction completion stay private.
// A callback cannot commit independently of its character save and receipt.
//
// It holds the engine-neutral query surface rather than a driver handle, which is
// what lets one callback signature serve both engines.
type Tx struct {
	queries     querySet
	accountID   int64
	characterID int64
}

func newTx(q querySet, account, character int64) *Tx {
	return &Tx{queries: q, accountID: account, characterID: character}
}
