package database

import (
	"context"
	"dfolan/internal/database/sqlcgen"
	"errors"
	"fmt"
	"math"

)

// MigrateSkinSelection stores which skin a character has applied. It only adds a
// table, so existing saves upgrade without any data change.
//
// The selection is per character while the cargo that authorises it is per
// account: NOTI1546 is an entry frame this server replays for one character at a
// time, and the client validates the id against the account's owned page.
//
// The `page` column holds the CMD1565 selection category, not a cargo page: the
// damage-font panel has two tabs (categories 2 and 6) that both enumerate cargo
// page 2, so one row per tab is what the client restores. The column keeps its
// original name so existing rows stay valid without a migration.
func (s *Store) MigrateSkinSelection(ctx context.Context) error {
	return s.execMigration(ctx, "0013_skin_selection.sql")
}

// SelectSkin applies one skin for the character's slot that the category names.
// The caller has to have checked that the account owns the skin, because the
// client resets the feature to its built-in default for any id its owned page does
// not hold. skinKey 0 records an unequip (the 解除 button).
func (s *Store) SelectSkin(ctx context.Context, character int64, category, skinKey uint32) error {
	if character == 0 {
		return errors.New("invalid skin selection")
	}
	return s.queries.SelectSkin(ctx, sqlcgen.SelectSkinParams{CharacterID: character, Page: int64(category), SkinKey: int64(skinKey)})
}

// SelectedSkin returns the skin the character applied for one category, or 0 when
// it never applied one or cleared it through 解除.
func (s *Store) SelectedSkin(ctx context.Context, character int64, category uint32) (uint32, error) {
	if character == 0 {
		return 0, fmt.Errorf("invalid character")
	}
	key, err := s.queries.SelectedSkin(ctx, sqlcgen.SelectedSkinParams{CharacterID: character, Page: int64(category)})
	if isNoRows(err) {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	if key < 0 || key > math.MaxUint32 {
		return 0, fmt.Errorf("stored skin key out of uint32 range")
	}
	return uint32(key), nil
}
