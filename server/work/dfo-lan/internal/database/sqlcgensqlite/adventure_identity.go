package sqlcgensqlite

import (
	"context"
	"encoding/json"
)

func (q *Queries) AdventureDataByName(ctx context.Context, name string) (json.RawMessage, error) {
	var data json.RawMessage
	err := q.db.QueryRowContext(ctx, `SELECT data FROM account_adventures WHERE name=$1`, name).Scan(&data)
	return data, err
}

func (q *Queries) AdventureNameAvailable(ctx context.Context, account int64, name string) (bool, error) {
	var available bool
	err := q.db.QueryRowContext(ctx, `SELECT NOT EXISTS(SELECT 1 FROM account_adventures WHERE name=$1 AND account_id<>$2)`, name, account).Scan(&available)
	return available, err
}
func (q *Queries) UpdateAdventureName(ctx context.Context, account int64, name string) error {
	_, err := q.db.ExecContext(ctx, `UPDATE account_adventures SET name=$1 WHERE account_id=$2`, name, account)
	return err
}
