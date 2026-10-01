package workflow

import (
	"context"
	"dfolan/internal/loot"
	"dfolan/internal/storage"
	"encoding/json"
)

func (s *LootService) CreateEquipment(
	ctx context.Context,
	role storage.Character,
	template uint32,
	slot uint32,
	payOption int,
) (storage.Character, loot.EquipmentCraftReceipt, bool, error) {
	var result loot.EquipmentCraftReceipt
	fail := func(e error) (storage.Character, loot.EquipmentCraftReceipt, bool, error) {
		return role, result, false, e
	}
	plan, e := s.Loot.PlanEquipmentCraft(LootRole(role), template, slot, payOption)
	if e != nil {
		return fail(e)
	}
	key := plan.Key
	saved, _, applied, e := s.Store.CommitAccountMaterialEvent(ctx, role.AccountID, role.ID,
		s.Loot.Catalog.Source.SaveIdentity(), key, s.Loot.Rules.Model,
		func(current storage.Character, accountRaw json.RawMessage) (json.RawMessage, json.RawMessage, error) {
			state, account, receipt, err := s.Loot.PrepareEquipmentCraft(LootRole(current), accountRaw, template, payOption, plan)
			result = receipt
			return state, account, err
		})
	if e != nil {
		return fail(e)
	}
	return saved, result, applied, nil
}
