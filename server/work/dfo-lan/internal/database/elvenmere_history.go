package database

import (
	"context"
	"encoding/json"
	"fmt"
)

type CharacterEventHistory struct {
	Key       string          `json:"key"`
	Outcome   json.RawMessage `json:"outcome"`
	CreatedAt int64           `json:"created_at"`
}

// This account-wide read stays on the SQLite write transaction already holding
// the owning account and character, including archived characters' receipts.
// It migrates old receipts without opening another connection or
// adding a schema. The removed engine is not reintroduced for this operation.
func (tx *Tx) CharacterEventHistory(ctx context.Context, model string) ([]CharacterEventHistory, error) {
	q, ok := tx.queries.(*sqliteQueries)
	if !ok {
		return nil, fmt.Errorf("character event history requires SQLite transaction")
	}
	var encoded string
	err := q.queryRow(ctx, `SELECT COALESCE(json_group_array(json_object('key',e.event_key,'outcome',json(e.outcome),'created_at',CAST(e.created_at/1000000 AS INTEGER))),'[]')
FROM character_events e JOIN characters c ON c.id=e.character_id
WHERE c.account_id=? AND e.model=?`, tx.accountID, model).Scan(&encoded)
	if err != nil {
		return nil, storageError(err)
	}
	var rows []CharacterEventHistory
	if err = json.Unmarshal([]byte(encoded), &rows); err != nil {
		return nil, err
	}
	return rows, nil
}
