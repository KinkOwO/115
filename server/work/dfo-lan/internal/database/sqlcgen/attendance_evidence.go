package sqlcgen

import (
	"context"
	"encoding/json"
)

// Account ownership includes receipts of other characters on the same account.
// Prefix matching is literal, so source-derived keys cannot broaden a LIKE query.
func (q *Queries) AttendanceEvidence(ctx context.Context, account int64, prefix string) (json.RawMessage, error) {
	rows, err := q.db.QueryContext(ctx, `SELECT e.event_key,e.outcome FROM character_events e JOIN characters c ON c.id=e.character_id WHERE c.account_id=$1 AND substr(e.event_key,1,length($2))=$2 AND e.model='account-adventure-v1' ORDER BY e.created_at,e.event_key`, account, prefix)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	type evidence struct {
		Key     string          `json:"key"`
		Outcome json.RawMessage `json:"outcome"`
	}
	out := []evidence{}
	for rows.Next() {
		var r evidence
		if err = rows.Scan(&r.Key, &r.Outcome); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	if err = rows.Err(); err != nil {
		return nil, err
	}
	return json.Marshal(out)
}
