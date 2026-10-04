package workflow

import (
	"context"
	"dfolan/internal/database"
	"dfolan/internal/game/protocol"
	"dfolan/internal/inventory"
	"encoding/json"
	"fmt"
)

// ApplyEquipmentAwakening owns the account-material transaction around the
// inventory preparation rules.
func (s *WearService) ApplyEquipmentAwakening(ctx context.Context, role database.Character, key string, r protocol.EquipmentAwakeningRequest) (database.Character, inventory.AwakeningReceipt, error) {
	var out inventory.AwakeningReceipt
	if s == nil || s.Store == nil || s.Catalog == nil {
		return role, out, fmt.Errorf("装备调适需要有效装备目录及角色存档")
	}
	if err := s.rules().ValidateEquipmentAwakening(InventoryRole(role)); err != nil {
		return role, out, err
	}
	applied := false
	saved, _, _, err := s.Store.CommitAccountMaterialEvent(ctx, role.AccountID, role.ID, role.ConfigVersion, key, "equipment-awakening-v1",
		func(current database.Character, counts json.RawMessage) (json.RawMessage, json.RawMessage, error) {
			state, nextCounts, receipt, err := s.rules().PrepareEquipmentAwakening(InventoryRole(current), counts, key, r)
			if err != nil {
				return nil, nil, err
			}
			out = receipt
			applied = true
			return state, nextCounts, nil
		})
	if err != nil {
		return role, out, err
	}
	if !applied {
		out, err = inventory.ReadAwakeningReceipt(saved.State, key)
		if err != nil {
			return role, out, err
		}
	}
	saved.WireID = role.WireID
	return saved, out, nil
}
