package workflow

import (
	"dfolan/internal/loot"
	"dfolan/internal/storage"
)

func LootRole(role storage.Character) loot.Role {
	return loot.Role{AccountID: role.AccountID, ID: role.ID, WireID: role.WireID, Profession: role.Profession, ConfigVersion: role.ConfigVersion, State: role.State}
}
