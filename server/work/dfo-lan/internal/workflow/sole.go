package workflow

import (
	"context"
	"dfolan/internal/game/protocol"
	"dfolan/internal/inventory"
	"dfolan/internal/storage"
	"encoding/json"
	"fmt"
)

// ApplySoleQuality owns the account-material transaction around the
// inventory preparation rules for CMD2288 sole quality.
func (s *WearService) ApplySoleQuality(ctx context.Context, role storage.Character, key string, r protocol.SoleQualityRequest) (storage.Character, inventory.SoleQualityReceipt, error) {
	var out inventory.SoleQualityReceipt
	if s == nil || s.Store == nil || s.Catalog == nil {
		return role, out, fmt.Errorf("秘宝精度需要有效装备目录及角色存档")
	}
	if err := s.rules().ValidateSoleQuality(InventoryRole(role)); err != nil {
		return role, out, err
	}
	applied := false
	saved, _, _, err := s.Store.CommitAccountMaterialEvent(ctx, role.AccountID, role.ID, role.ConfigVersion, key, "sole-quality-v1",
		func(current storage.Character, counts json.RawMessage) (json.RawMessage, json.RawMessage, error) {
			state, nextCounts, receipt, err := s.rules().PrepareSoleQuality(InventoryRole(current), counts, key, r)
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
		out, err = inventory.ReadSoleQualityReceipt(saved.State, key)
		if err != nil {
			return role, out, err
		}
	}
	saved.WireID = role.WireID
	return saved, out, nil
}
