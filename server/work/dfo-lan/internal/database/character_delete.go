package database

import (
	"context"
	"dfolan/internal/database/sqlcgen"
	"errors"
)

// DeleteCharacter retains all role state and dependent receipts. A name stays
// reserved while archived. Matching slot AND exact confirmation name prevents
// a retried old slot from deleting its newly shifted neighbour.
func (s *Store) DeleteCharacter(ctx context.Context, account int64, slot uint16, name string) (int64, error) {
	tx, e := s.engine.begin(ctx)
	if e != nil {
		return 0, e
	}
	defer tx.rollback(ctx)
	queries := tx.queries()
	if _, e = queries.LockAccount(ctx, account); e != nil {
		return 0, e
	}
	row, e := queries.CharacterAtRosterSlot(ctx, sqlcgen.CharacterAtRosterSlotParams{AccountID: account, RosterSlot: int64(slot)})
	if e != nil {
		return 0, e
	}
	if name == "" || row.Name != name {
		return 0, errors.New("delete confirmation does not match current roster")
	}
	if e = queries.ArchiveCharacter(ctx, sqlcgen.ArchiveCharacterParams{CharacterID: row.ID, AccountID: account}); e != nil {
		return 0, e
	}
	if e = tx.commit(ctx); e != nil {
		return 0, e
	}
	return row.ID, nil
}
