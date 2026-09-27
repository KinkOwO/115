package storage

import (
	"bytes"
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
)

// GamepadPayloadSize is the fixed size of NOTI 2128 packet body (0x583).
const GamepadPayloadSize = 1411

// DefaultGamepadOptions represents the 5 uint16 LE options when not configured
// (values: 0, 20, 40, 50, 0 => 00 00 14 00 28 00 32 00 00 00).
var DefaultGamepadOptions = []byte{0x00, 0x00, 0x14, 0x00, 0x28, 0x00, 0x32, 0x00, 0x00, 0x00}

// MigrateGamepad creates the account_gamepad_settings table if not exists.
func (s *Store) MigrateGamepad(ctx context.Context) error {
	_, err := s.DB.Exec(ctx, `CREATE TABLE IF NOT EXISTS account_gamepad_settings (
    account_id bigint NOT NULL REFERENCES accounts(id) PRIMARY KEY,
    mapping_tsv bytea NOT NULL DEFAULT ''::bytea,
    options bytea NOT NULL DEFAULT ''::bytea,
    updated_at timestamptz NOT NULL DEFAULT now()
);`)
	return err
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
	_, err := s.DB.Exec(ctx, `INSERT INTO account_gamepad_settings(account_id, mapping_tsv, updated_at)
VALUES($1, $2, now())
ON CONFLICT (account_id)
DO UPDATE SET mapping_tsv = EXCLUDED.mapping_tsv, updated_at = now()`, accountID, cleanTSV)
	return err
}

// SaveAccountGamepadOptions upserts the 10-byte gamepad options for an account.
func (s *Store) SaveAccountGamepadOptions(ctx context.Context, accountID int64, opts []byte) error {
	if accountID == 0 {
		return fmt.Errorf("gamepad settings require an account")
	}
	_, err := s.DB.Exec(ctx, `INSERT INTO account_gamepad_settings(account_id, options, updated_at)
VALUES($1, $2, now())
ON CONFLICT (account_id)
DO UPDATE SET options = EXCLUDED.options, updated_at = now()`, accountID, opts)
	return err
}

// AccountGamepadPayload queries the account gamepad settings and constructs the 1411-byte payload.
// Returns nil if no custom gamepad settings are saved.
func (s *Store) AccountGamepadPayload(ctx context.Context, accountID int64) ([]byte, error) {
	if accountID == 0 {
		return nil, nil
	}
	var mappingTSV, options []byte
	err := s.DB.QueryRow(ctx, `SELECT mapping_tsv, options FROM account_gamepad_settings WHERE account_id = $1`, accountID).Scan(&mappingTSV, &options)
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
