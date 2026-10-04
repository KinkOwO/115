package workflow

import (
	"dfolan/internal/database"
	"dfolan/internal/loot"
)

func LootRole(role database.Character) loot.Role {
	return loot.Role{AccountID: role.AccountID, ID: role.ID, WireID: role.WireID, Profession: role.Profession, ConfigVersion: role.ConfigVersion, State: role.State}
}

func lootPremiumActivations(values []loot.PremiumActivation) []database.CashPremiumActivation {
	if values == nil {
		return nil
	}
	out := make([]database.CashPremiumActivation, len(values))
	for i, v := range values {
		out[i] = database.CashPremiumActivation{Type: v.Type, DurationSecond: v.DurationSecond}
	}
	return out
}
