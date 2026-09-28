package storage

import (
	"context"
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
	tx, e := s.DB.Begin(ctx)
	if e != nil {
		return false, e
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if !starred {
		if _, e = tx.Exec(ctx, `DELETE FROM character_skin_favorite
 WHERE character_id=$1 AND page=$2 AND skin_key=$3`, character, page, key); e != nil {
			return false, e
		}
		return true, tx.Commit(ctx)
	}
	var held int
	if e = tx.QueryRow(ctx, `SELECT count(*) FROM character_skin_favorite
 WHERE character_id=$1 AND page=$2`, character, page).Scan(&held); e != nil {
		return false, e
	}
	if held >= limit {
		return false, nil
	}
	if _, e = tx.Exec(ctx, `INSERT INTO character_skin_favorite(character_id,page,skin_key)
 VALUES($1,$2,$3) ON CONFLICT(character_id,page,skin_key) DO NOTHING`, character, page, key); e != nil {
		return false, e
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
	rows, e := s.DB.Query(ctx, `SELECT page, skin_key FROM character_skin_favorite
 WHERE character_id=$1 AND page < $2 ORDER BY page, skin_key`, character, groups)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	pages := make([][]uint32, groups)
	for rows.Next() {
		var page, key uint64
		if e = rows.Scan(&page, &key); e != nil {
			return nil, e
		}
		pages[page] = append(pages[page], uint32(key))
	}
	return pages, rows.Err()
}
