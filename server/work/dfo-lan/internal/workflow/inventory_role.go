package workflow

import (
	"context"
	"dfolan/internal/inventory"
	"dfolan/internal/storage"
	"time"
)

// InventoryRole selects the character fields consumed by inventory rules.
func InventoryRole(role storage.Character) inventory.Role {
	return inventory.Role{AccountID: role.AccountID, Profession: role.Profession, ConfigVersion: role.ConfigVersion, State: role.State}
}

// PremiumReader adapts the premium ledger to the inventory's conqueror query.
type PremiumReader struct{ Store *storage.Store }

func (p PremiumReader) HasConquerorPremium(ctx context.Context, account int64, now time.Time) (bool, error) {
	return p.Store.HasActivePremium(ctx, account, storage.PremiumConqueror, now)
}
