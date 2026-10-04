package database

import (
	"context"
	"dfolan/internal/database/sqlcgen"
	"errors"
	"fmt"
)

// Skin favourites are the per-tab 收藏 star rows the 概要 page renders.
//
// The table has no client-side write path to mirror: the list's single writer is NOTI2641's
// reader (sub_1444E8DF0 has exactly one caller, sub_1444ED1B0 — analysis/ida-work/df42.log
// section 5), and the CMD1565 star request is never applied by its own echo because the
// command reply core runs its whole category switch only when the result word is 0. The
// server therefore owns this state outright: it stores the toggle and answers with the
// absolute 10-group frame.
//
// A row's group is the skin's registry page, which is also the selection category
// (protocol.SkinFavorites / df41_fav_flush_DD510_1441dd510.c:116 passes `*(row+36)` — the
// registry record's page — as the group key), so the key here is (character, page, skin)
// and the client's per-group cap is enforced per page.
//
// The DDL lives in MigrateSkinSelectionList so the gateway still calls one migration for
// this feature; see skin_selection_list.go.

// SetSkinFavorite applies or clears one star. It reports false when the add was refused
// because that page already holds cap favourites — the same condition under which the
// client's own star handler shows message 101037008 instead of storing the row.
func (s *Store) SetSkinFavorite(ctx context.Context, character int64, page uint32, key uint32, starred bool, limit int) (bool, error) {
	if character == 0 {
		return false, errors.New("invalid skin favourite")
	}
	if limit < 0 {
		return false, fmt.Errorf("invalid favourite cap")
	}
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return false, err
	}
	defer tx.Rollback(ctx)
	queries := s.queries.WithTx(tx)
	if !starred {
		if err = queries.RemoveSkinFavorite(ctx, sqlcgen.RemoveSkinFavoriteParams{CharacterID: character, Page: int64(page), SkinKey: int64(key)}); err != nil {
			return false, err
		}
		return true, tx.Commit(ctx)
	}
	held, err := queries.CountSkinFavorites(ctx, sqlcgen.CountSkinFavoritesParams{CharacterID: character, Page: int64(page)})
	if err != nil {
		return false, err
	}
	if held >= int64(limit) {
		return false, nil
	}
	if err = queries.AddSkinFavorite(ctx, sqlcgen.AddSkinFavoriteParams{CharacterID: character, Page: int64(page), SkinKey: int64(key)}); err != nil {
		return false, err
	}
	return true, tx.Commit(ctx)
}

// SkinFavorites returns the character's favourites as exactly groups page groups, each
// ordered by skin id. Every group is present even when empty so the caller can encode the
// absolute frame without padding it by hand.
func (s *Store) SkinFavorites(ctx context.Context, character int64, groups int) ([][]uint32, error) {
	if character == 0 {
		return nil, fmt.Errorf("invalid character")
	}
	if groups <= 0 {
		return nil, fmt.Errorf("invalid favourite group count")
	}
	rows, err := s.queries.SkinFavorites(ctx, sqlcgen.SkinFavoritesParams{CharacterID: character, Groups: int64(groups)})
	if err != nil {
		return nil, err
	}
	pages := make([][]uint32, groups)
	for _, row := range rows {
		if row.Page < 0 || row.SkinKey < 0 {
			return nil, fmt.Errorf("negative stored skin page or key")
		}
		pages[row.Page] = append(pages[row.Page], uint32(row.SkinKey))
	}
	return pages, nil
}
