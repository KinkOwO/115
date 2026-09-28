package storage

import (
	"context"
	"errors"
	"fmt"
	"github.com/jackc/pgx/v5"
)

// Unified options: CMD2377 subtype 0x01 carries the account-scoped option
// block (restored through NOTI2826), subtype 0x05 the character-scoped system
// settings block. Both are durable here. The account block has a known restore
// channel; the character settings block's offset inside NOTI2827 is not yet
// reversed, so it is stored for a later restore path.
const maxUnifiedOption = 286

type UnifiedOptionEntry struct {
	Position uint16
	Value    uint16
}

func (s *Store) MigrateUnifiedOptions(ctx context.Context) error {
	if _, e := s.DB.Exec(ctx, `CREATE TABLE IF NOT EXISTS account_unified_options(
 account_id bigint NOT NULL REFERENCES accounts(id),
 opt_index integer NOT NULL CHECK(opt_index BETWEEN 0 AND 285),
 value integer NOT NULL,
 updated_at timestamptz NOT NULL DEFAULT now(),
 PRIMARY KEY(account_id,opt_index));`); e != nil {
		return e
	}
	if _, e := s.DB.Exec(ctx, `CREATE TABLE IF NOT EXISTS character_unified_options(
 character_id bigint NOT NULL REFERENCES characters(id),
 opt_index integer NOT NULL CHECK(opt_index BETWEEN 0 AND 285),
 value integer NOT NULL,
 updated_at timestamptz NOT NULL DEFAULT now(),
 PRIMARY KEY(character_id,opt_index));`); e != nil {
		return e
	}
	if _, e := s.DB.Exec(ctx, `CREATE TABLE IF NOT EXISTS account_hotkeys(
 account_id bigint NOT NULL REFERENCES accounts(id),
 subtype smallint NOT NULL CHECK(subtype IN (3, 4)),
 slot_index smallint NOT NULL CHECK(slot_index BETWEEN 0 AND 156),
 keycode integer NOT NULL,
 updated_at timestamptz NOT NULL DEFAULT now(),
 PRIMARY KEY(account_id,subtype,slot_index));`); e != nil {
		return e
	}
	_, e := s.DB.Exec(ctx, `CREATE TABLE IF NOT EXISTS character_hotkeys(
 character_id bigint NOT NULL REFERENCES characters(id),
 subtype smallint NOT NULL CHECK(subtype IN (3, 4)),
 slot_index smallint NOT NULL CHECK(slot_index BETWEEN 0 AND 156),
 keycode integer NOT NULL,
 updated_at timestamptz NOT NULL DEFAULT now(),
 PRIMARY KEY(character_id,subtype,slot_index));`)
	return e
}

func validateUnifiedEntries(entries []UnifiedOptionEntry) error {
	for _, e := range entries {
		if int(e.Position) >= maxUnifiedOption {
			return fmt.Errorf("unified option index out of range")
		}
	}
	return nil
}

// SaveAccountUnifiedOptions upserts the account option block under the owning
// account's row lock, so a replayed frame is a no-op instead of a double apply.
func (s *Store) SaveAccountUnifiedOptions(ctx context.Context, account int64, entries []UnifiedOptionEntry) error {
	if account == 0 {
		return fmt.Errorf("unified options require an account")
	}
	if len(entries) == 0 {
		return nil
	}
	if e := validateUnifiedEntries(entries); e != nil {
		return e
	}
	tx, e := s.DB.Begin(ctx)
	if e != nil {
		return e
	}
	defer tx.Rollback(ctx)
	if e = tx.QueryRow(ctx, `SELECT 1 FROM accounts WHERE id=$1 FOR UPDATE`, account).Scan(new(int)); e != nil {
		return e
	}
	for _, en := range entries {
		if _, e = tx.Exec(ctx, `INSERT INTO account_unified_options(account_id,opt_index,value) VALUES($1,$2,$3)
 ON CONFLICT(account_id,opt_index) DO UPDATE SET value=EXCLUDED.value,updated_at=now()`, account, int(en.Position), int(en.Value)); e != nil {
			return e
		}
	}
	return tx.Commit(ctx)
}

// SaveCharacterUnifiedOptions upserts the character settings block, scoped to a
// character owned by the account.
func (s *Store) SaveCharacterUnifiedOptions(ctx context.Context, account, id int64, entries []UnifiedOptionEntry) error {
	if len(entries) == 0 {
		return nil
	}
	if e := validateUnifiedEntries(entries); e != nil {
		return e
	}
	tx, e := s.DB.Begin(ctx)
	if e != nil {
		return e
	}
	defer tx.Rollback(ctx)
	var one int
	if e = tx.QueryRow(ctx, `SELECT 1 FROM characters WHERE account_id=$1 AND id=$2 AND deleted_at IS NULL FOR UPDATE`, account, id).Scan(&one); e != nil {
		if errors.Is(e, pgx.ErrNoRows) {
			return fmt.Errorf("unified options character is not owned")
		}
		return e
	}
	for _, en := range entries {
		if _, e = tx.Exec(ctx, `INSERT INTO character_unified_options(character_id,opt_index,value) VALUES($1,$2,$3)
 ON CONFLICT(character_id,opt_index) DO UPDATE SET value=EXCLUDED.value,updated_at=now()`, id, int(en.Position), int(en.Value)); e != nil {
			return e
		}
	}
	return tx.Commit(ctx)
}

// AccountUnifiedOptions returns the stored account option overrides for the
// NOTI2826 restore payload. The native missing-value sentinel is not an
// override: leave that position at the client's default instead of rejecting
// the whole account block and losing unrelated settings such as guide flags.
func (s *Store) AccountUnifiedOptions(ctx context.Context, account int64) (map[uint16]uint16, error) {
	out := map[uint16]uint16{}
	if account == 0 {
		return out, nil
	}
	rows, e := s.DB.Query(ctx, `SELECT opt_index,value FROM account_unified_options WHERE account_id=$1`, account)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	for rows.Next() {
		var idx, value int
		if e = rows.Scan(&idx, &value); e != nil {
			return nil, e
		}
		if value == 65535 {
			continue
		}
		out[uint16(idx)] = uint16(value)
	}
	return out, rows.Err()
}

// CharacterUnifiedOptions returns the stored per-character setting overrides
// (CMD2377 subtype 0x05) for the NOTI2827 restore payload.
func (s *Store) CharacterUnifiedOptions(ctx context.Context, characterID int64) (map[uint16]uint16, error) {
	out := map[uint16]uint16{}
	if characterID == 0 {
		return out, nil
	}
	rows, e := s.DB.Query(ctx, `SELECT opt_index,value FROM character_unified_options WHERE character_id=$1`, characterID)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	for rows.Next() {
		var idx, value int
		if e = rows.Scan(&idx, &value); e != nil {
			return nil, e
		}
		out[uint16(idx)] = uint16(value)
	}
	return out, rows.Err()
}

// SaveAccountHotkeys upserts account-scoped keyboard hotkeys (Subtype 3 Scheme A, Subtype 4 Scheme B).
func (s *Store) SaveAccountHotkeys(ctx context.Context, account int64, subtype byte, entries []UnifiedOptionEntry) error {
	if account == 0 {
		return fmt.Errorf("account hotkeys require an account")
	}
	if subtype != 3 && subtype != 4 {
		return fmt.Errorf("unsupported hotkey subtype %d", subtype)
	}
	if len(entries) == 0 {
		return nil
	}
	tx, err := s.DB.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if err = tx.QueryRow(ctx, `SELECT 1 FROM accounts WHERE id=$1 FOR UPDATE`, account).Scan(new(int)); err != nil {
		return err
	}
	for _, en := range entries {
		if en.Position > 156 {
			continue
		}
		if _, err = tx.Exec(ctx, `INSERT INTO account_hotkeys(account_id, subtype, slot_index, keycode)
VALUES($1, $2, $3, $4)
ON CONFLICT(account_id, subtype, slot_index) DO UPDATE SET keycode=EXCLUDED.keycode, updated_at=now()`,
			account, int(subtype), int(en.Position), int(en.Value)); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

// SaveCharacterHotkeys upserts character-scoped keyboard hotkeys (Subtype 3 Scheme A, Subtype 4 Scheme B).
func (s *Store) SaveCharacterHotkeys(ctx context.Context, account, characterID int64, subtype byte, entries []UnifiedOptionEntry) error {
	if subtype != 3 && subtype != 4 {
		return fmt.Errorf("unsupported hotkey subtype %d", subtype)
	}
	if len(entries) == 0 {
		return nil
	}
	tx, err := s.DB.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	var one int
	if err = tx.QueryRow(ctx, `SELECT 1 FROM characters WHERE account_id=$1 AND id=$2 AND deleted_at IS NULL FOR UPDATE`, account, characterID).Scan(&one); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return fmt.Errorf("character hotkeys: character is not owned")
		}
		return err
	}
	for _, en := range entries {
		if en.Position > 156 {
			continue
		}
		if _, err = tx.Exec(ctx, `INSERT INTO character_hotkeys(character_id, subtype, slot_index, keycode)
VALUES($1, $2, $3, $4)
ON CONFLICT(character_id, subtype, slot_index) DO UPDATE SET keycode=EXCLUDED.keycode, updated_at=now()`,
			characterID, int(subtype), int(en.Position), int(en.Value)); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

// AccountHotkeys returns stored account-wide keyboard hotkeys for the requested subtype (3 or 4).
func (s *Store) AccountHotkeys(ctx context.Context, account int64, subtype byte) (map[uint16]uint16, error) {
	out := map[uint16]uint16{}
	if account == 0 {
		return out, nil
	}
	rows, err := s.DB.Query(ctx, `SELECT slot_index, keycode FROM account_hotkeys WHERE account_id=$1 AND subtype=$2`, account, int(subtype))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var slot, keycode int
		if err = rows.Scan(&slot, &keycode); err != nil {
			return nil, err
		}
		out[uint16(slot)] = uint16(keycode)
	}
	return out, rows.Err()
}

// CharacterHotkeys returns stored character-specific keyboard hotkeys for the requested subtype (3 or 4).
func (s *Store) CharacterHotkeys(ctx context.Context, characterID int64, subtype byte) (map[uint16]uint16, error) {
	out := map[uint16]uint16{}
	if characterID == 0 {
		return out, nil
	}
	rows, err := s.DB.Query(ctx, `SELECT slot_index, keycode FROM character_hotkeys WHERE character_id=$1 AND subtype=$2`, characterID, int(subtype))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var slot, keycode int
		if err = rows.Scan(&slot, &keycode); err != nil {
			return nil, err
		}
		out[uint16(slot)] = uint16(keycode)
	}
	return out, rows.Err()
}

// PromoteCharacterHotkeysToAccount copies a character's custom hotkeys to the account level.
// This is used when saving with account scope from a character that has custom hotkeys,
// ensuring the active key mapping becomes the account baseline.
func (s *Store) PromoteCharacterHotkeysToAccount(ctx context.Context, account, characterID int64, subtype byte) error {
	if account == 0 || characterID == 0 {
		return nil
	}
	if subtype != 3 && subtype != 4 {
		return fmt.Errorf("unsupported hotkey subtype %d", subtype)
	}
	_, err := s.DB.Exec(ctx, `
INSERT INTO account_hotkeys (account_id, subtype, slot_index, keycode, updated_at)
SELECT $1, subtype, slot_index, keycode, now()
FROM character_hotkeys
WHERE character_id = $2 AND subtype = $3
ON CONFLICT (account_id, subtype, slot_index)
DO UPDATE SET keycode = EXCLUDED.keycode, updated_at = now()`,
		account, characterID, int(subtype))
	return err
}

// ClearAccountCharacterHotkeys removes character-specific keyboard hotkeys for all characters of the account
// (for the given subtype) so all characters fall back to account hotkeys.
func (s *Store) ClearAccountCharacterHotkeys(ctx context.Context, accountID int64, subtype byte) error {
	if accountID == 0 {
		return nil
	}
	if subtype != 3 && subtype != 4 {
		return fmt.Errorf("unsupported hotkey subtype %d", subtype)
	}
	_, err := s.DB.Exec(ctx, `
DELETE FROM character_hotkeys
WHERE character_id IN (SELECT id FROM characters WHERE account_id = $1)
  AND subtype = $2`,
		accountID, int(subtype))
	return err
}

// CopyAccountHotkeysToCharacter copies account-wide hotkeys to a character as a baseline
// if the character does not already have custom hotkeys for those slots.
func (s *Store) CopyAccountHotkeysToCharacter(ctx context.Context, account, characterID int64, subtype byte) error {
	if account == 0 || characterID == 0 {
		return nil
	}
	if subtype != 3 && subtype != 4 {
		return fmt.Errorf("unsupported hotkey subtype %d", subtype)
	}
	_, err := s.DB.Exec(ctx, `
INSERT INTO character_hotkeys (character_id, subtype, slot_index, keycode, updated_at)
SELECT $2, subtype, slot_index, keycode, now()
FROM account_hotkeys
WHERE account_id = $1 AND subtype = $3
ON CONFLICT (character_id, subtype, slot_index) DO NOTHING`,
		account, characterID, int(subtype))
	return err
}

