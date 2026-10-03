package storage

import "context"

// PendingMoonRewardRuns lists frozen rewards without a grant receipt in replay order.
func (s *Store) PendingMoonRewardRuns(ctx context.Context, characterID int64, model string) ([]string, error) {
	rows, err := s.DB.Query(ctx, `SELECT substr(event_key,12) FROM character_events e WHERE character_id=$1 AND model=$2 AND event_key LIKE 'moon-clear:%' AND NOT EXISTS(SELECT 1 FROM character_events g WHERE g.character_id=e.character_id AND g.event_key='moon-grant:'||substr(e.event_key,12)) ORDER BY created_at LIMIT 16`, characterID, model)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var runs []string
	for rows.Next() {
		var run string
		if err := rows.Scan(&run); err != nil {
			return nil, err
		}
		runs = append(runs, run)
	}
	return runs, rows.Err()
}
