package database

import (
	"context"
	"dfolan/internal/database/sqlcgen"
	"fmt"
)

// Unified options: CMD2377 subtype 0x01 carries account options restored
// through NOTI2826, subtype 0x05 carries character settings, and subtype 0x12
// carries a separate six-slot character effect group in NOTI2827.
const maxUnifiedOption = 286

type UnifiedOptionEntry struct {
	Position uint16
	Value    uint16
}

func (s *Store) MigrateUnifiedOptions(ctx context.Context) error {
	if err := s.execMigration(ctx, "0016_unified_options.sql"); err != nil {
		return err
	}
	return s.migrateWarpFavorites(ctx)
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
	return inTx(ctx, s.engine, func(tx txHandle) error {
		queries := tx.queries()
		if _, e := queries.LockAccount(ctx, account); e != nil {
			return e
		}
		for _, en := range entries {
			if e := queries.SaveAccountUnifiedOption(ctx, sqlcgen.SaveAccountUnifiedOptionParams{AccountID: account, OptIndex: int32(en.Position), Value: int32(en.Value)}); e != nil {
				return e
			}
		}
		return nil
	})
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
	return inTx(ctx, s.engine, func(tx txHandle) error {
		queries := tx.queries()
		if _, e := queries.LockCharacterOwner(ctx, sqlcgen.LockCharacterOwnerParams{AccountID: account, CharacterID: id}); e != nil {
			if isNoRows(e) {
				return fmt.Errorf("unified options character is not owned")
			}
			return e
		}
		for _, en := range entries {
			if e := queries.SaveCharacterUnifiedOption(ctx, sqlcgen.SaveCharacterUnifiedOptionParams{CharacterID: id, OptIndex: int32(en.Position), Value: int32(en.Value)}); e != nil {
				return e
			}
		}
		return nil
	})
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
	rows, err := s.queries.AccountUnifiedOptions(ctx, account)
	if err != nil {
		return nil, err
	}
	for _, row := range rows {
		out[uint16(row.OptIndex)] = uint16(row.Value)
	}
	return out, nil
}

// CharacterUnifiedOptions returns the stored per-character setting overrides
// (CMD2377 subtype 0x05) for the NOTI2827 restore payload.
func (s *Store) CharacterUnifiedOptions(ctx context.Context, characterID int64) (map[uint16]uint16, error) {
	out := map[uint16]uint16{}
	if characterID == 0 {
		return out, nil
	}
	rows, err := s.queries.CharacterUnifiedOptions(ctx, characterID)
	if err != nil {
		return nil, err
	}
	for _, row := range rows {
		out[uint16(row.OptIndex)] = uint16(row.Value)
	}
	return out, nil
}

// SaveCharacterUnifiedOptionGroup persists a subtype-specific character
// option group without colliding with the ordinary subtype 5 settings indices.
func (s *Store) SaveCharacterUnifiedOptionGroup(ctx context.Context, account, characterID int64, subtype byte, entries []UnifiedOptionEntry) error {
	if subtype != 18 {
		return fmt.Errorf("unsupported character option group subtype %d", subtype)
	}
	if len(entries) == 0 {
		return nil
	}
	for _, entry := range entries {
		if entry.Position >= 6 {
			return fmt.Errorf("character option group index out of range")
		}
	}
	return inTx(ctx, s.engine, func(tx txHandle) error {
		queries := tx.queries()
		if _, err := queries.LockCharacterOwner(ctx, sqlcgen.LockCharacterOwnerParams{AccountID: account, CharacterID: characterID}); err != nil {
			if isNoRows(err) {
				return fmt.Errorf("character option group: character is not owned")
			}
			return err
		}
		for _, entry := range entries {
			if err := queries.SaveCharacterUnifiedOptionGroup(ctx, sqlcgen.SaveCharacterUnifiedOptionGroupParams{CharacterID: characterID, Subtype: int16(subtype), OptIndex: int32(entry.Position), Value: int32(entry.Value)}); err != nil {
				return err
			}
		}
		return nil
	})
}

// CharacterUnifiedOptionGroup returns the persisted values for one supported
// subtype-specific character option group.
func (s *Store) CharacterUnifiedOptionGroup(ctx context.Context, characterID int64, subtype byte) (map[uint16]uint16, error) {
	if subtype != 18 {
		return nil, fmt.Errorf("unsupported character option group subtype %d", subtype)
	}
	out := map[uint16]uint16{}
	if characterID == 0 {
		return out, nil
	}
	rows, err := s.queries.CharacterUnifiedOptionGroup(ctx, sqlcgen.CharacterUnifiedOptionGroupParams{CharacterID: characterID, Subtype: int16(subtype)})
	if err != nil {
		return nil, err
	}
	for _, row := range rows {
		out[uint16(row.OptIndex)] = uint16(row.Value)
	}
	return out, nil
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
	return inTx(ctx, s.engine, func(tx txHandle) error {
		queries := tx.queries()
		if _, err := queries.LockAccount(ctx, account); err != nil {
			return err
		}
		for _, en := range entries {
			if en.Position > 156 {
				continue
			}
			if err := queries.SaveAccountHotkey(ctx, sqlcgen.SaveAccountHotkeyParams{AccountID: account, Subtype: int16(subtype), SlotIndex: int16(en.Position), Keycode: int32(en.Value)}); err != nil {
				return err
			}
		}
		return nil
	})
}

// SaveCharacterHotkeys upserts character-scoped keyboard hotkeys (Subtype 3 Scheme A, Subtype 4 Scheme B).
func (s *Store) SaveCharacterHotkeys(ctx context.Context, account, characterID int64, subtype byte, entries []UnifiedOptionEntry) error {
	if subtype != 3 && subtype != 4 {
		return fmt.Errorf("unsupported hotkey subtype %d", subtype)
	}
	if len(entries) == 0 {
		return nil
	}
	return inTx(ctx, s.engine, func(tx txHandle) error {
		queries := tx.queries()
		if _, err := queries.LockCharacterOwner(ctx, sqlcgen.LockCharacterOwnerParams{AccountID: account, CharacterID: characterID}); err != nil {
			if isNoRows(err) {
				return fmt.Errorf("character hotkeys: character is not owned")
			}
			return err
		}
		for _, en := range entries {
			if en.Position > 156 {
				continue
			}
			if err := queries.SaveCharacterHotkey(ctx, sqlcgen.SaveCharacterHotkeyParams{CharacterID: characterID, Subtype: int16(subtype), SlotIndex: int16(en.Position), Keycode: int32(en.Value)}); err != nil {
				return err
			}
		}
		return nil
	})
}

// AccountHotkeys returns stored account-wide keyboard hotkeys for the requested subtype (3 or 4).
func (s *Store) AccountHotkeys(ctx context.Context, account int64, subtype byte) (map[uint16]uint16, error) {
	out := map[uint16]uint16{}
	if account == 0 {
		return out, nil
	}
	rows, err := s.queries.AccountHotkeys(ctx, sqlcgen.AccountHotkeysParams{AccountID: account, Subtype: int16(subtype)})
	if err != nil {
		return nil, err
	}
	for _, row := range rows {
		out[uint16(row.SlotIndex)] = uint16(row.Keycode)
	}
	return out, nil
}

// CharacterHotkeys returns stored character-specific keyboard hotkeys for the requested subtype (3 or 4).
func (s *Store) CharacterHotkeys(ctx context.Context, characterID int64, subtype byte) (map[uint16]uint16, error) {
	out := map[uint16]uint16{}
	if characterID == 0 {
		return out, nil
	}
	rows, err := s.queries.CharacterHotkeys(ctx, sqlcgen.CharacterHotkeysParams{CharacterID: characterID, Subtype: int16(subtype)})
	if err != nil {
		return nil, err
	}
	for _, row := range rows {
		out[uint16(row.SlotIndex)] = uint16(row.Keycode)
	}
	return out, nil
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
	err := s.queries.PromoteCharacterHotkeysToAccount(ctx, sqlcgen.PromoteCharacterHotkeysToAccountParams{AccountID: account, CharacterID: characterID, Subtype: int16(subtype)})
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
	err := s.queries.ClearAccountCharacterHotkeys(ctx, sqlcgen.ClearAccountCharacterHotkeysParams{AccountID: accountID, Subtype: int16(subtype)})
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
	err := s.queries.CopyAccountHotkeysToCharacter(ctx, sqlcgen.CopyAccountHotkeysToCharacterParams{AccountID: account, CharacterID: characterID, Subtype: int16(subtype)})
	return err
}
