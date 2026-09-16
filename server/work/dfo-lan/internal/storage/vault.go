package storage

import (
	"context"
	"encoding/json"
	"errors"
)

type VaultState struct {
	Slots         uint16
	Items         json.RawMessage
	ConfigVersion string
}

func (s *Store) MigrateVault(ctx context.Context) error {
	_, e := s.DB.Exec(ctx, `CREATE TABLE IF NOT EXISTS character_vaults (
      character_id bigint PRIMARY KEY REFERENCES characters(id),
      slots integer NOT NULL CHECK(slots BETWEEN 1 AND 65535),
      items jsonb NOT NULL DEFAULT '[]' CHECK(jsonb_typeof(items)='array'),
      config_version text NOT NULL, updated_at timestamptz NOT NULL DEFAULT now());`)
	return e
}

// Initialization inserts only absent state; reconnect never resets stored items.
func (s *Store) LoadVault(ctx context.Context, account, id int64, initial uint16, version string) (VaultState, error) {
	var v VaultState
	if initial == 0 || len(version) != 64 {
		return v, errors.New("invalid vault source")
	}
	_, e := s.DB.Exec(ctx, `INSERT INTO character_vaults(character_id,slots,config_version)
      SELECT id,$3,$4 FROM characters WHERE id=$2 AND account_id=$1 ON CONFLICT DO NOTHING`, account, id, initial, version)
	if e != nil {
		return v, e
	}
	e = s.DB.QueryRow(ctx, `SELECT v.slots,v.items,v.config_version FROM character_vaults v JOIN characters c ON c.id=v.character_id WHERE c.id=$2 AND c.account_id=$1`, account, id).Scan(&v.Slots, &v.Items, &v.ConfigVersion)
	return v, e
}
