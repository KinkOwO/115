package sqlcgen

import "context"

// The retained internal query layer still satisfies querySet; SQLite is the
// active engine. These additive operations use its database/sql driver seam.
func (q *Queries) CharacterCapacity(ctx context.Context, account int64) (int64, error) {
	var capacity int64
	err := q.db.QueryRowContext(ctx, "SELECT COALESCE((SELECT capacity FROM account_character_slots WHERE account_id=?),0)", account).Scan(&capacity)
	return capacity, err
}
func (q *Queries) SaveCharacterCapacity(ctx context.Context, account, capacity int64) error {
	_, err := q.db.ExecContext(ctx, "INSERT INTO account_character_slots(account_id,capacity) VALUES(?,?) ON CONFLICT(account_id) DO UPDATE SET capacity=excluded.capacity", account, capacity)
	return err
}
