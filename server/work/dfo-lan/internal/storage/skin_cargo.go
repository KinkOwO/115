package storage

import (
	"context"
	"fmt"
	"time"
)

// AccountSkin is one skin a player registered by using an
// `[action type] [add skin storage]` stackable (CMD507 action 169).
type AccountSkin struct {
	SourceTemplate uint32    `json:"source_template"`
	SkinKey        uint32    `json:"skin_key"`
	UnlockedAt     time.Time `json:"unlocked_at"`
}

// MigrateSkinCargo creates the account-shared skin cargo, upgrading the shape an
// earlier version of this table left behind. The upgrade only touches this table,
// so existing character and item saves are untouched.
func (s *Store) MigrateSkinCargo(ctx context.Context) error {
	_, e := s.DB.Exec(ctx, `CREATE TABLE IF NOT EXISTS account_skin_cargo (
 account_id bigint NOT NULL REFERENCES accounts(id),
 source_template bigint NOT NULL,
 skin_key bigint NOT NULL,
 unlocked_at timestamptz NOT NULL DEFAULT now(),
 PRIMARY KEY(account_id, source_template));`)
	if e != nil {
		return e
	}
	// On a database that already ran the first version the CREATE is a no-op and
	// the key column does not exist at all: that version keyed the cargo with
	// `skin_index`, a per-file index the client never reads. Adding the column is
	// only half the upgrade, because every later write is `ON CONFLICT DO NOTHING`
	// and therefore cannot repair the rows the old code stored.
	if _, e = s.DB.Exec(ctx, `ALTER TABLE account_skin_cargo ADD COLUMN IF NOT EXISTS skin_key bigint NOT NULL DEFAULT 0`); e != nil {
		return e
	}
	var legacy bool
	if e = s.DB.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM information_schema.columns
 WHERE table_schema=current_schema() AND table_name='account_skin_cargo' AND column_name='action_param')`).Scan(&legacy); e != nil {
		return e
	}
	if !legacy {
		return nil
	}
	// `action_param` is the item's `[action type]` parameter, which is the
	// `list/skin.lst` identifier and so the correct key. The legacy columns stay:
	// dropping them cannot be undone, and they only cost one unused integer.
	// `skin_index` is NOT NULL without a default, so it needs a default before the
	// current INSERT (which writes only the v2 columns) can land.
	_, e = s.DB.Exec(ctx, `UPDATE account_skin_cargo SET skin_key=action_param
 WHERE skin_key=0 AND action_param<>0;
 ALTER TABLE account_skin_cargo ALTER COLUMN skin_index SET DEFAULT 0;`)
	return e
}

// UnlockSkin registers one skin for the account. The insert is idempotent on
// (account, template), so a replayed hotkey press cannot register twice, and a
// use whose registration failed after the item was already spent still lands on
// the next press.
func (s *Store) UnlockSkin(ctx context.Context, account int64, template, skinKey uint32) error {
	if account == 0 || template == 0 || skinKey == 0 {
		return fmt.Errorf("invalid skin unlock")
	}
	_, e := s.DB.Exec(ctx, `INSERT INTO account_skin_cargo(account_id,source_template,skin_key)
 VALUES($1,$2,$3) ON CONFLICT(account_id,source_template) DO NOTHING`, account, template, skinKey)
	return e
}

// ListSkins returns every skin registered to the account, oldest unlock first. A
// row whose key is still unknown is left out rather than sent: skin id 0 is not a
// skin, and the page frame rebuilds the whole cargo from the ids it carries.
func (s *Store) ListSkins(ctx context.Context, account int64) ([]AccountSkin, error) {
	rows, e := s.DB.Query(ctx, `SELECT source_template,skin_key,unlocked_at FROM account_skin_cargo
 WHERE account_id=$1 AND skin_key<>0 ORDER BY source_template`, account)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	var out []AccountSkin
	for rows.Next() {
		var skin AccountSkin
		if e = rows.Scan(&skin.SourceTemplate, &skin.SkinKey, &skin.UnlockedAt); e != nil {
			return nil, e
		}
		out = append(out, skin)
	}
	return out, rows.Err()
}
