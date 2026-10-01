package storage

import (
	"context"
	"dfolan/internal/game/protocol"
	"fmt"

	"github.com/jackc/pgx/v5"
)

// This additive table keeps account favorites separate from ordinary numeric
// options and character state. Each upload is a delta, including empty slots.
func (s *Store) migrateWarpFavorites(ctx context.Context) error {
	_, err := s.DB.Exec(ctx, `CREATE TABLE IF NOT EXISTS account_warp_favorites (
 account_id bigint NOT NULL REFERENCES accounts(id),
 slot_index smallint NOT NULL CHECK(slot_index BETWEEN 0 AND 9),
 value bytea NOT NULL CHECK(octet_length(value) = 12),
 updated_at timestamptz NOT NULL DEFAULT now(),
 PRIMARY KEY(account_id,slot_index));`)
	return err
}

func (s *Store) SaveAccountWarpFavorites(ctx context.Context, account int64, entries []protocol.WarpFavoriteEntry) error {
	if account <= 0 {
		return fmt.Errorf("warp favorites require an account")
	}
	if err := protocol.ValidateWarpFavorites(entries); err != nil {
		return err
	}
	if len(entries) == 0 {
		return nil
	}
	return pgx.BeginFunc(ctx, s.DB, func(tx pgx.Tx) error {
		if err := tx.QueryRow(ctx, `SELECT 1 FROM accounts WHERE id=$1 FOR UPDATE`, account).Scan(new(int)); err != nil {
			return err
		}
		for _, entry := range entries {
			if _, err := tx.Exec(ctx, `INSERT INTO account_warp_favorites(account_id,slot_index,value) VALUES($1,$2,$3)
 ON CONFLICT(account_id,slot_index) DO UPDATE SET value=EXCLUDED.value,updated_at=now()
 WHERE account_warp_favorites.value IS DISTINCT FROM EXCLUDED.value`, account, int(entry.Position), entry.Value[:]); err != nil {
				return err
			}
		}
		return nil
	})
}

func (s *Store) AccountWarpFavorites(ctx context.Context, account int64) ([]protocol.WarpFavoriteEntry, error) {
	if account <= 0 {
		return nil, fmt.Errorf("warp favorites require an account")
	}
	rows, err := s.DB.Query(ctx, `SELECT slot_index,value FROM account_warp_favorites WHERE account_id=$1 ORDER BY slot_index`, account)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var entries []protocol.WarpFavoriteEntry
	for rows.Next() {
		var slot int
		var value []byte
		if err := rows.Scan(&slot, &value); err != nil {
			return nil, err
		}
		if slot < 0 || slot >= protocol.WarpFavoriteSlots || len(value) != protocol.WarpFavoriteRecordSize {
			return nil, fmt.Errorf("invalid stored warp favorite slot %d", slot)
		}
		entry := protocol.WarpFavoriteEntry{Position: uint16(slot)}
		copy(entry.Value[:], value)
		entries = append(entries, entry)
	}
	return entries, rows.Err()
}
