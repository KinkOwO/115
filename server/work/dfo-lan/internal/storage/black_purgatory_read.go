package storage

import (
	"context"
	"encoding/json"
)

// ReadPendingBlackPurgatoryRewards visits frozen card plans which do not yet
// have their recovery marker. The callback keeps decoding in the workflow
// while this adapter owns the SQL cursor and its error order.
func (s *Store) ReadPendingBlackPurgatoryRewards(ctx context.Context, account, character int64, model string, consume func(json.RawMessage) error) error {
	rows, err := s.DB.Query(ctx, `SELECT e.outcome FROM character_events e
 JOIN characters c ON c.id=e.character_id
 WHERE c.account_id=$1 AND c.id=$2 AND e.model=$3 AND e.event_key LIKE 'cardplan:%'
 AND NOT EXISTS(SELECT 1 FROM character_events g WHERE g.character_id=e.character_id
 AND g.event_key='black-purgatory-recovered:'||substr(e.event_key,10)) ORDER BY e.created_at LIMIT 16`, account, character, model)
	if err != nil {
		return err
	}
	for rows.Next() {
		var raw json.RawMessage
		if err = rows.Scan(&raw); err == nil {
			err = consume(raw)
		}
		if err != nil {
			rows.Close()
			return err
		}
	}
	err = rows.Err()
	rows.Close()
	return err
}
