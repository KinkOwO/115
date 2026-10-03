package workflow

import (
	"dfolan/internal/loot"
	"dfolan/internal/storage"
)

func LootRole(role storage.Character) loot.Role {
	return loot.Role{AccountID: role.AccountID, ID: role.ID, WireID: role.WireID, Profession: role.Profession, ConfigVersion: role.ConfigVersion, State: role.State}
}

func lootPremiumActivations(values []loot.PremiumActivation) []storage.CashPremiumActivation {
	if values == nil {
		return nil
	}
	out := make([]storage.CashPremiumActivation, len(values))
	for i, v := range values {
		out[i] = storage.CashPremiumActivation{Type: v.Type, DurationSecond: v.DurationSecond}
	}
	return out
}
