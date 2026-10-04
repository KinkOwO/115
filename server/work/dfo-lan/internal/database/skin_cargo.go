package database

import (
	"context"
	"dfolan/internal/database/sqlcgen"
	"fmt"
	"math"
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
	return s.execMigration(ctx, "0015_skin_cargo.sql")
}

// UnlockSkin registers one skin for the account. The insert is idempotent on
// (account, template), so a replayed hotkey press cannot register twice, and a
// use whose registration failed after the item was already spent still lands on
// the next press.
func (s *Store) UnlockSkin(ctx context.Context, account int64, template, skinKey uint32) error {
	if account == 0 || template == 0 || skinKey == 0 {
		return fmt.Errorf("invalid skin unlock")
	}
	return s.queries.UnlockSkin(ctx, sqlcgen.UnlockSkinParams{AccountID: account, SourceTemplate: int64(template), SkinKey: int64(skinKey)})
}

// ListSkins returns every skin registered to the account, oldest unlock first. A
// row whose key is still unknown is left out rather than sent: skin id 0 is not a
// skin, and the page frame rebuilds the whole cargo from the ids it carries.
func (s *Store) ListSkins(ctx context.Context, account int64) ([]AccountSkin, error) {
	rows, err := s.queries.ListSkins(ctx, account)
	if err != nil {
		return nil, err
	}
	var out []AccountSkin
	for _, row := range rows {
		if row.SourceTemplate < 0 || row.SourceTemplate > math.MaxUint32 || row.SkinKey < 0 || row.SkinKey > math.MaxUint32 {
			return nil, fmt.Errorf("stored skin cargo id out of uint32 range")
		}
		out = append(out, AccountSkin{SourceTemplate: uint32(row.SourceTemplate), SkinKey: uint32(row.SkinKey), UnlockedAt: row.UnlockedAt})
	}
	return out, nil
}
