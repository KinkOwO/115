package database

import (
	"context"
	"dfolan/internal/database/sqlcgen"
	"dfolan/internal/game/protocol"
	"fmt"

)

// This additive table keeps account favorites separate from ordinary numeric
// options and character state. Each upload is a delta, including empty slots.
func (s *Store) migrateWarpFavorites(ctx context.Context) error {
	return s.execMigration(ctx, "0005_warp_favorites.sql")
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
	return inTx(ctx, s.engine, func(tx txHandle) error {
		queries := tx.queries()
		if _, err := queries.LockAccount(ctx, account); err != nil {
			return err
		}
		for _, entry := range entries {
			if err := queries.SaveAccountWarpFavorite(ctx, sqlcgen.SaveAccountWarpFavoriteParams{
				AccountID: account, SlotIndex: int16(entry.Position), Value: entry.Value[:],
			}); err != nil {
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
	rows, err := s.queries.AccountWarpFavorites(ctx, account)
	if err != nil {
		return nil, err
	}
	var entries []protocol.WarpFavoriteEntry
	for _, row := range rows {
		slot, value := int(row.SlotIndex), row.Value
		if slot < 0 || slot >= protocol.WarpFavoriteSlots || len(value) != protocol.WarpFavoriteRecordSize {
			return nil, fmt.Errorf("invalid stored warp favorite slot %d", slot)
		}
		entry := protocol.WarpFavoriteEntry{Position: uint16(slot)}
		copy(entry.Value[:], value)
		entries = append(entries, entry)
	}
	return entries, nil
}
