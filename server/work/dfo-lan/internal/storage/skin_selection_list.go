package storage

import (
	"context"
	"errors"
	"fmt"
)

// MigrateSkinSelectionList stores the selections a category holds as a **set**
// rather than one id: the two list categories fill their selection vector from
// several id slots at once (CMD1565 body `u32 category, u32 result, u32 ids[20]`,
// walked slot by slot by sub_1444EE820), and the 觉醒插图 renderer then draws a
// random member of that vector (df26_ebc10caller_sub_1444EA8A0.c), so "which skins
// are applied" genuinely has more than one answer per character.
//
// character_skin_selection above stays untouched — the damage-font tabs are
// one-id-per-category and already round-trip through it. Adding a table keeps both
// the existing rows and the existing code valid, which is what the save-compat
// constraint asks for.
//
// The two further tables live in the same migration because the gateway calls exactly
// one entry point here:
//   - character_skin_selection_slot keeps a position, because the 表情 selection is
//     positional rather than a set (see SetSkinSelectionSlots);
//   - character_skin_favorite keeps the per-page star list the 概要 tab renders, which
//     nothing else on the client stores (see skin_favorite.go).
func (s *Store) MigrateSkinSelectionList(ctx context.Context) error {
	_, e := s.DB.Exec(ctx, `CREATE TABLE IF NOT EXISTS character_skin_selection_list(
 character_id bigint NOT NULL REFERENCES characters(id),
 category bigint NOT NULL,
 skin_key bigint NOT NULL,
 updated_at timestamptz NOT NULL DEFAULT now(),
 PRIMARY KEY(character_id,category,skin_key));`)
	if e != nil {
		return e
	}
	_, e = s.DB.Exec(ctx, `CREATE TABLE IF NOT EXISTS character_skin_selection_slot(
 character_id bigint NOT NULL REFERENCES characters(id),
 category bigint NOT NULL,
 slot bigint NOT NULL,
 skin_key bigint NOT NULL,
 updated_at timestamptz NOT NULL DEFAULT now(),
 PRIMARY KEY(character_id,category,slot));`)
	if e != nil {
		return e
	}
	_, e = s.DB.Exec(ctx, `CREATE TABLE IF NOT EXISTS character_skin_favorite(
 character_id bigint NOT NULL REFERENCES characters(id),
 page bigint NOT NULL,
 skin_key bigint NOT NULL,
 updated_at timestamptz NOT NULL DEFAULT now(),
 PRIMARY KEY(character_id,page,skin_key));`)
	return e
}

// SetSkinSelectionList replaces one category's whole selection. The client sends the
// slots it wants applied as one list and its own reader replaces the vector from
// scratch, so an incremental insert here would keep ids the player just deselected.
// An empty list therefore clears the category, which is what 解除 means.
func (s *Store) SetSkinSelectionList(ctx context.Context, character int64, category uint32, keys []uint32) error {
	if character == 0 {
		return errors.New("invalid skin selection")
	}
	tx, e := s.DB.Begin(ctx)
	if e != nil {
		return e
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, e = tx.Exec(ctx, `DELETE FROM character_skin_selection_list
 WHERE character_id=$1 AND category=$2`, character, category); e != nil {
		return e
	}
	for _, key := range keys {
		if _, e = tx.Exec(ctx, `INSERT INTO character_skin_selection_list(character_id,category,skin_key)
 VALUES($1,$2,$3) ON CONFLICT(character_id,category,skin_key) DO NOTHING`, character, category, key); e != nil {
			return e
		}
	}
	return tx.Commit(ctx)
}

// SkinSelectionList returns the skins the character has applied for one category,
// ordered by id. The client neither reads nor renders an order — the 边框 vector's
// three slots merge in read order and each consumer filters that vector by its own
// registry subtype, while 觉醒插图 picks at random — so any stable order serves.
func (s *Store) SkinSelectionList(ctx context.Context, character int64, category uint32) ([]uint32, error) {
	if character == 0 {
		return nil, fmt.Errorf("invalid character")
	}
	rows, e := s.DB.Query(ctx, `SELECT skin_key FROM character_skin_selection_list
 WHERE character_id=$1 AND category=$2 ORDER BY skin_key`, character, category)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	var keys []uint32
	for rows.Next() {
		var key uint64
		if e = rows.Scan(&key); e != nil {
			return nil, e
		}
		keys = append(keys, uint32(key))
	}
	return keys, rows.Err()
}

// SetSkinSelectionSlots replaces one category's positional selection.
//
// Unlike the set above, 表情 is stored by cell: NOTI1546's category-3 reader consumes a
// fixed four words with no count prefix and appends each survivor to one vector in read
// order, and its only consumer forwards that vector to the chat channel unchanged, so the
// index in the vector IS the quick-bar cell. An empty cell is sent as a zero word — the
// reader appends the zero and thereby keeps the positions after it — which is why this
// table has (category, slot) rather than (category, skin_key) as its key: the same skin
// legitimately occupies two cells, and dropping a duplicate would shift the bar.
//
// A zero slot is not stored; read back, it becomes the zero word again.
func (s *Store) SetSkinSelectionSlots(ctx context.Context, character int64, category uint32, keys []uint32) error {
	if character == 0 {
		return errors.New("invalid skin selection")
	}
	tx, e := s.DB.Begin(ctx)
	if e != nil {
		return e
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, e = tx.Exec(ctx, `DELETE FROM character_skin_selection_slot
 WHERE character_id=$1 AND category=$2`, character, category); e != nil {
		return e
	}
	for slot, key := range keys {
		if key == 0 {
			continue
		}
		if _, e = tx.Exec(ctx, `INSERT INTO character_skin_selection_slot(character_id,category,slot,skin_key)
 VALUES($1,$2,$3,$4)`, character, category, slot, key); e != nil {
			return e
		}
	}
	return tx.Commit(ctx)
}

// SkinSelectionSlots returns the category's selection as a slice of exactly width
// elements, ordered by cell and zero-filled where the cell is empty. Callers pass the
// width the client's reader consumes (protocol.SkinSelectionInstantEmoticonSlots).
func (s *Store) SkinSelectionSlots(ctx context.Context, character int64, category uint32, width int) ([]uint32, error) {
	if character == 0 {
		return nil, fmt.Errorf("invalid character")
	}
	if width <= 0 {
		return nil, fmt.Errorf("invalid skin slot width")
	}
	rows, e := s.DB.Query(ctx, `SELECT slot, skin_key FROM character_skin_selection_slot
 WHERE character_id=$1 AND category=$2`, character, category)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	keys := make([]uint32, width)
	for rows.Next() {
		var slot, key uint64
		if e = rows.Scan(&slot, &key); e != nil {
			return nil, e
		}
		// A row beyond the width the current reader consumes is stale state from a wider
		// layout; it is dropped rather than growing the frame past its proven length.
		if slot >= uint64(width) {
			continue
		}
		keys[slot] = uint32(key)
	}
	return keys, rows.Err()
}
