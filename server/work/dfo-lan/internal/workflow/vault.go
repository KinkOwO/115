package workflow

import (
	"context"
	"dfolan/internal/database"
	"dfolan/internal/game/protocol"
	"dfolan/internal/inventory"
	"encoding/json"
	"fmt"
)

// VaultService owns initialization and atomic character/vault transfers.
type VaultService struct {
	inventory.VaultService
	Store *database.Store
}

func (s *VaultService) Bootstrap(ctx context.Context, role database.Character) ([]byte, error) {
	return s.BootstrapSpace(ctx, role, 2)
}

func (s *VaultService) BootstrapSpace(ctx context.Context, role database.Character, space byte) ([]byte, error) {
	initial := s.Rules.InitialSlots
	if space == 45 && s.Rules.InitialSecondarySlots != 0 {
		initial = s.Rules.InitialSecondarySlots
	}
	v, e := s.Store.LoadVault(ctx, role.AccountID, role.ID, initial, s.Rules.SourceSHA256, space)
	if e != nil {
		return nil, e
	}
	if v.ConfigVersion != s.Rules.SourceSHA256 || !s.Rules.Allows(v.Slots) {
		return nil, fmt.Errorf("vault requires configuration migration")
	}
	vault, e := inventory.ReadExtendedVault(v)
	if e != nil {
		return nil, e
	}
	return protocol.PersonalVaultSpace(space, vault.Slots, vault.Rows())
}

func (s *VaultService) Move(ctx context.Context, role database.Character, key string, r protocol.ItemMoveRequest) (database.Character, database.VaultState, bool, error) {
	if s == nil || s.Store == nil {
		return role, database.VaultState{}, false, fmt.Errorf("vault service unavailable")
	}
	request, e := json.Marshal(r)
	if e != nil {
		return role, database.VaultState{}, false, e
	}
	saved, v, applied, e := s.Store.CommitVaultTransfer(ctx, role.AccountID, role.ID, role.ConfigVersion, s.Rules.SourceSHA256, key, request, func(current database.Character, v database.VaultState) (json.RawMessage, json.RawMessage, error) {
		return s.TransferCombined(InventoryRole(current), v, r)
	})
	saved.WireID = role.WireID
	return saved, v, applied, e
}
