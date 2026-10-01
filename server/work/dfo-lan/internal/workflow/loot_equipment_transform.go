package workflow

import (
	"context"
	"dfolan/internal/loot"
	"dfolan/internal/storage"
	"encoding/json"
)

func (s *LootService) TransformEquipment(
	ctx context.Context,
	role storage.Character,
	slots []uint32,
	templates []uint32,
	payOption int,
) (storage.Character, loot.EquipmentTransformReceipt, bool, error) {
	var result loot.EquipmentTransformReceipt
	fail := func(e error) (storage.Character, loot.EquipmentTransformReceipt, bool, error) {
		return role, result, false, e
	}
	plan, e := s.Loot.PlanEquipmentTransform(LootRole(role), slots, templates, payOption)
	result = plan.Receipt
	if e != nil {
		return fail(e)
	}
	key := plan.Key
	saved, _, applied, e := s.Store.CommitAccountMaterialEvent(ctx, role.AccountID, role.ID,
		s.Loot.Catalog.Source.SaveIdentity(), key, s.Loot.Rules.Model,
		func(current storage.Character, accountRaw json.RawMessage) (json.RawMessage, json.RawMessage, error) {
			state, account, receipt, err := s.Loot.PrepareEquipmentTransform(LootRole(current), accountRaw, plan)
			result = receipt
			return state, account, err
		})
	if e != nil {
		return fail(e)
	}
	return saved, result, applied, nil
}
