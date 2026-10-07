package database

import (
	"context"
	"dfolan/internal/database/sqlcgen"
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
	return s.execMigration(ctx, "0014_skin_lists.sql")
}

// SetSkinSelectionList replaces one category's whole selection. The client sends the
// slots it wants applied as one list and its own reader replaces the vector from
// scratch, so an incremental insert here would keep ids the player just deselected.
// An empty list therefore clears the category, which is what 解除 means.
func (s *Store) SetSkinSelectionList(ctx context.Context, character int64, category uint32, keys []uint32) error {
	if character == 0 {
		return errors.New("invalid skin selection")
	}
	tx, err := s.engine.begin(ctx)
	if err != nil {
		return err
	}
	defer tx.rollback(ctx)
	queries := tx.queries()
	if err = queries.ClearSkinSelectionList(ctx, sqlcgen.ClearSkinSelectionListParams{CharacterID: character, Category: int64(category)}); err != nil {
		return err
	}
	for _, key := range keys {
		if err = queries.AddSkinSelectionList(ctx, sqlcgen.AddSkinSelectionListParams{CharacterID: character, Category: int64(category), SkinKey: int64(key)}); err != nil {
			return err
		}
	}
	return tx.commit(ctx)
}

// SkinSelectionList returns the skins the character has applied for one category,
// ordered by id. The client neither reads nor renders an order — the 边框 vector's
// three slots merge in read order and each consumer filters that vector by its own
// registry subtype, while 觉醒插图 picks at random — so any stable order serves.
func (s *Store) SkinSelectionList(ctx context.Context, character int64, category uint32) ([]uint32, error) {
	if character == 0 {
		return nil, fmt.Errorf("invalid character")
	}
	rows, err := s.queries.SkinSelectionList(ctx, sqlcgen.SkinSelectionListParams{CharacterID: character, Category: int64(category)})
	if err != nil {
		return nil, err
	}
	var keys []uint32
	for _, key := range rows {
		if key < 0 {
			return nil, fmt.Errorf("negative stored skin key")
		}
		keys = append(keys, uint32(key))
	}
	return keys, nil
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
	tx, err := s.engine.begin(ctx)
	if err != nil {
		return err
	}
	defer tx.rollback(ctx)
	queries := tx.queries()
	if err = queries.ClearSkinSelectionSlots(ctx, sqlcgen.ClearSkinSelectionSlotsParams{CharacterID: character, Category: int64(category)}); err != nil {
		return err
	}
	for slot, key := range keys {
		if key == 0 {
			continue
		}
		if err = queries.AddSkinSelectionSlot(ctx, sqlcgen.AddSkinSelectionSlotParams{CharacterID: character, Category: int64(category), Slot: int64(slot), SkinKey: int64(key)}); err != nil {
			return err
		}
	}
	return tx.commit(ctx)
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
	rows, err := s.queries.SkinSelectionSlots(ctx, sqlcgen.SkinSelectionSlotsParams{CharacterID: character, Category: int64(category)})
	if err != nil {
		return nil, err
	}
	keys := make([]uint32, width)
	for _, row := range rows {
		if row.Slot < 0 || row.SkinKey < 0 {
			return nil, fmt.Errorf("negative stored skin slot or key")
		}
		if row.Slot >= int64(width) {
			continue
		}
		keys[row.Slot] = uint32(row.SkinKey)
	}
	return keys, nil
}
