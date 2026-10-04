package database

import (
	"bytes"
	"context"
	"dfolan/internal/database/sqlcgen"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
)

// GamepadPayloadSize is the fixed size of NOTI 2128 packet body (0x583).
const GamepadPayloadSize = 1411

// DefaultGamepadOptions represents the 5 uint16 LE options when not configured
// (values: 0, 20, 40, 50, 0 => 00 00 14 00 28 00 32 00 00 00).
var DefaultGamepadOptions = []byte{0x00, 0x00, 0x14, 0x00, 0x28, 0x00, 0x32, 0x00, 0x00, 0x00}

// MigrateGamepad creates the account_gamepad_settings and character_gamepad_settings tables if not exists.
func (s *Store) MigrateGamepad(ctx context.Context) error {
	return s.execMigration(ctx, "0010_gamepad.sql")
}

// BuildGamepadPayload constructs the 1411-byte NOTI 2128 payload.
// Offset 0: 0x00 enables custom gamepad mapping (0x01 resets to default).
// Offset 1..1400: zero-padded ASCII TSV mapping.
// Offset 1401..1410: 5 uint16 LE options (defaults to DefaultGamepadOptions if len != 10).
func BuildGamepadPayload(mappingTSV []byte, options []byte) []byte {
	payload := make([]byte, GamepadPayloadSize)
	payload[0] = 0x00

	if len(mappingTSV) > 0 {
		copyLen := len(mappingTSV)
		if copyLen > 1400 {
			copyLen = 1400
		}
		copy(payload[1:1+copyLen], mappingTSV[:copyLen])
	}

	if len(options) == 10 {
		copy(payload[1401:1411], options)
	} else {
		copy(payload[1401:1411], DefaultGamepadOptions)
	}

	return payload
}

// SaveAccountGamepadKeys upserts the gamepad TSV keys for an account.
func (s *Store) SaveAccountGamepadKeys(ctx context.Context, accountID int64, tsv []byte) error {
	if accountID == 0 {
		return fmt.Errorf("gamepad settings require an account")
	}
	cleanTSV := bytes.TrimRight(tsv, "\x00")
	return s.queries.SaveAccountGamepadKeys(ctx, sqlcgen.SaveAccountGamepadKeysParams{AccountID: accountID, MappingTsv: cleanTSV})
}

// SaveAccountGamepadOptions upserts the 10-byte gamepad options for an account.
func (s *Store) SaveAccountGamepadOptions(ctx context.Context, accountID int64, opts []byte) error {
	if accountID == 0 {
		return fmt.Errorf("gamepad settings require an account")
	}
	return s.queries.SaveAccountGamepadOptions(ctx, sqlcgen.SaveAccountGamepadOptionsParams{AccountID: accountID, Options: opts})
}

// AccountGamepadPayload queries the account gamepad settings and constructs the 1411-byte payload.
// Returns nil if no custom gamepad settings are saved.
func (s *Store) AccountGamepadPayload(ctx context.Context, accountID int64) ([]byte, error) {
	if accountID == 0 {
		return nil, nil
	}
	settings, err := s.queries.AccountGamepadSettings(ctx, accountID)
	mappingTSV, options := settings.MappingTsv, settings.Options
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if len(mappingTSV) == 0 && len(options) == 0 {
		return nil, nil
	}
	return BuildGamepadPayload(mappingTSV, options), nil
}

// SaveCharacterGamepadKeys upserts the gamepad TSV keys for a specific character.
func (s *Store) SaveCharacterGamepadKeys(ctx context.Context, accountID, characterID int64, tsv []byte) error {
	if characterID == 0 {
		return fmt.Errorf("character gamepad settings require a character")
	}
	cleanTSV := bytes.TrimRight(tsv, "\x00")
	return s.queries.SaveCharacterGamepadKeys(ctx, sqlcgen.SaveCharacterGamepadKeysParams{CharacterID: characterID, AccountID: accountID, MappingTsv: cleanTSV})
}

// SaveCharacterGamepadOptions upserts the 10-byte gamepad options for a specific character.
func (s *Store) SaveCharacterGamepadOptions(ctx context.Context, accountID, characterID int64, opts []byte) error {
	if characterID == 0 {
		return fmt.Errorf("character gamepad settings require a character")
	}
	return s.queries.SaveCharacterGamepadOptions(ctx, sqlcgen.SaveCharacterGamepadOptionsParams{CharacterID: characterID, AccountID: accountID, Options: opts})
}

// ClearCharacterGamepadSettings removes character-specific gamepad settings so it falls back to account settings.
func (s *Store) ClearCharacterGamepadSettings(ctx context.Context, characterID int64) error {
	if characterID == 0 {
		return nil
	}
	return s.queries.ClearCharacterGamepadSettings(ctx, characterID)
}

// ClearAccountCharacterGamepadSettings removes character-specific gamepad settings for all characters of the account
// so all characters fall back to account settings.
func (s *Store) ClearAccountCharacterGamepadSettings(ctx context.Context, accountID int64) error {
	if accountID == 0 {
		return nil
	}
	return s.queries.ClearAccountCharacterGamepadSettings(ctx, accountID)
}

// CharacterGamepadPayload queries the character gamepad settings and constructs the 1411-byte payload.
// Returns nil if no custom gamepad settings are saved for the character.
func (s *Store) CharacterGamepadPayload(ctx context.Context, characterID int64) ([]byte, error) {
	if characterID == 0 {
		return nil, nil
	}
	settings, err := s.queries.CharacterGamepadSettings(ctx, characterID)
	mappingTSV, options := settings.MappingTsv, settings.Options
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if len(mappingTSV) == 0 && len(options) == 0 {
		return nil, nil
	}
	return BuildGamepadPayload(mappingTSV, options), nil
}

// ResolveGamepadPayload resolves the gamepad payload for a character with fallback to account settings.
// If the character has custom settings, they take precedence (with partial fallback to account settings for missing parts).
// If the character has no custom settings, it falls back to account settings.
func (s *Store) ResolveGamepadPayload(ctx context.Context, accountID, characterID int64) ([]byte, error) {
	var charTSV, charOpts []byte
	if characterID != 0 {
		settings, err := s.queries.CharacterGamepadSettings(ctx, characterID)
		charTSV, charOpts = settings.MappingTsv, settings.Options
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return nil, err
		}
	}

	var accTSV, accOpts []byte
	if accountID != 0 {
		settings, err := s.queries.AccountGamepadSettings(ctx, accountID)
		accTSV, accOpts = settings.MappingTsv, settings.Options
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return nil, err
		}
	}

	var finalTSV []byte
	if len(charTSV) > 0 {
		finalTSV = charTSV
	} else if len(accTSV) > 0 {
		finalTSV = accTSV
	}

	var finalOpts []byte
	if len(charOpts) == 10 {
		finalOpts = charOpts
	} else if len(accOpts) == 10 {
		finalOpts = accOpts
	}

	if len(finalTSV) == 0 && len(finalOpts) == 0 {
		return nil, nil
	}
	return BuildGamepadPayload(finalTSV, finalOpts), nil
}
