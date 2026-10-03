package storage

import (
	"context"
	"errors"
)

// DeleteCharacter retains all role state and dependent receipts. A name stays
// reserved while archived. Matching slot AND exact confirmation name prevents
// a retried old slot from deleting its newly shifted neighbour.
func (s *Store) DeleteCharacter(ctx context.Context, account int64, slot uint16, name string) (int64, error) {
	tx, e := s.DB.Begin(ctx)
	if e != nil {
		return 0, e
	}
	defer tx.Rollback(ctx)
	var owner int64
	if e = tx.QueryRow(ctx, `SELECT id FROM accounts WHERE id=$1 FOR UPDATE`, account).Scan(&owner); e != nil {
		return 0, e
	}
	var id int64
	var actual string
	e = tx.QueryRow(ctx, `SELECT id,name FROM characters WHERE account_id=$1 AND deleted_at IS NULL ORDER BY coalesce(roster_order,wire_id),wire_id OFFSET $2 LIMIT 1 FOR UPDATE`, account, int(slot)).Scan(&id, &actual)
	if e != nil {
		return 0, e
	}
	if name == "" || actual != name {
		return 0, errors.New("delete confirmation does not match current roster")
	}
	if _, e = tx.Exec(ctx, `UPDATE characters SET deleted_at=now() WHERE id=$1 AND account_id=$2 AND deleted_at IS NULL`, id, account); e != nil {
		return 0, e
	}
	if e = tx.Commit(ctx); e != nil {
		return 0, e
	}
	return id, nil
}
