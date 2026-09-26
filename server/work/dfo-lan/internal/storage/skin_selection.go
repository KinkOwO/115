package storage

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
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
	_, e := s.DB.Exec(ctx, `CREATE TABLE IF NOT EXISTS character_skin_selection (
 character_id bigint NOT NULL REFERENCES characters(id),
 page bigint NOT NULL,
 skin_key bigint NOT NULL,
 updated_at timestamptz NOT NULL DEFAULT now(),
 PRIMARY KEY(character_id,page));`)
	return e
}

// SelectSkin applies one skin for the character's slot that the category names.
// The caller has to have checked that the account owns the skin, because the
// client resets the feature to its built-in default for any id its owned page does
// not hold. skinKey 0 records an unequip (the 解除 button).
func (s *Store) SelectSkin(ctx context.Context, character int64, category, skinKey uint32) error {
	if character == 0 {
		return errors.New("invalid skin selection")
	}
	_, e := s.DB.Exec(ctx, `INSERT INTO character_skin_selection(character_id,page,skin_key)
 VALUES($1,$2,$3) ON CONFLICT(character_id,page) DO UPDATE SET skin_key=EXCLUDED.skin_key,updated_at=now()`,
		character, category, skinKey)
	return e
}

// SelectedSkin returns the skin the character applied for one category, or 0 when
// it never applied one or cleared it through 解除.
func (s *Store) SelectedSkin(ctx context.Context, character int64, category uint32) (uint32, error) {
	if character == 0 {
		return 0, fmt.Errorf("invalid character")
	}
	var skinKey uint32
	e := s.DB.QueryRow(ctx, `SELECT skin_key FROM character_skin_selection
 WHERE character_id=$1 AND page=$2`, character, category).Scan(&skinKey)
	if errors.Is(e, pgx.ErrNoRows) {
		return 0, nil
	}
	return skinKey, e
}
