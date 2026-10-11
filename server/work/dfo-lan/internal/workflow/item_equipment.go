package workflow

import (
	"context"
	"dfolan/internal/database"
	"dfolan/internal/game/protocol"
	"dfolan/internal/inventory"
	"encoding/json"
	"fmt"
)

func (s *ItemService) CreateEquipment(
	ctx context.Context,
	role database.Character,
	template uint32,
	slot uint32,
	payOption int,
) (database.Character, inventory.EquipmentCraftReceipt, bool, error) {
	var result inventory.EquipmentCraftReceipt
	fail := func(e error) (database.Character, inventory.EquipmentCraftReceipt, bool, error) {
		return role, result, false, e
	}
	plan, e := s.Items.PlanEquipmentCraft(InventoryRole(role), template, slot, payOption)
	if e != nil {
		return fail(e)
	}
	key := plan.Key
	// 金币从**账号金库**同步调取（业主确认的官方「取用金库材料」语义）：
	// 提交覆盖 角色 + 账号材料(灵魂) + 金库金币 三者，一个事务原子落账。
	saved, _, _, applied, e := s.Store.CommitAccountVault(ctx, role.AccountID, role.ID,
		s.Items.Catalog.Source.SaveIdentity(), key, 2259,
		func(current database.Character, accountRaw json.RawMessage, vault database.AccountVaultState) (json.RawMessage, json.RawMessage, database.AccountVaultState, error) {
			state, account, vaultGold, receipt, err := s.Items.PrepareEquipmentCraft(InventoryRole(current), accountRaw, vault.Gold, template, payOption, plan)
			result = receipt
			vault.Gold = vaultGold
			return state, account, vault, err
		})
	if e != nil {
		return fail(e)
	}
	return saved, result, applied, nil
}
func (s *ItemService) TransformEquipment(
	ctx context.Context,
	role database.Character,
	slots []uint32,
	templates []uint32,
	payOption int,
) (database.Character, inventory.EquipmentTransformReceipt, bool, error) {
	var result inventory.EquipmentTransformReceipt
	fail := func(e error) (database.Character, inventory.EquipmentTransformReceipt, bool, error) {
		return role, result, false, e
	}
	plan, e := s.Items.PlanEquipmentTransform(InventoryRole(role), slots, templates, payOption)
	result = plan.Receipt
	if e != nil {
		return fail(e)
	}
	key := plan.Key
	// 金币从**账号金库**同步调取（同 CreateEquipment 的说明）。
	saved, _, _, applied, e := s.Store.CommitAccountVault(ctx, role.AccountID, role.ID,
		s.Items.Catalog.Source.SaveIdentity(), key, 2259,
		func(current database.Character, accountRaw json.RawMessage, vault database.AccountVaultState) (json.RawMessage, json.RawMessage, database.AccountVaultState, error) {
			state, account, vaultGold, receipt, err := s.Items.PrepareEquipmentTransform(InventoryRole(current), accountRaw, vault.Gold, plan)
			result = receipt
			vault.Gold = vaultGold
			return state, account, vault, err
		})
	if e != nil {
		return fail(e)
	}
	return saved, result, applied, nil
}

// TransformPrimers 实现装备库「誓约 / 晶体变换」（CMD2381 ENUM_CMDPACKET_PRIMER_TRANSFORM）。
//
// 与 TransformEquipment 同构：成本可能落在**账号共享材料**（灵魂仓库/巡礼之印按源表所在仓）
// 与背包两处，所以走 CommitAccountMaterialEvent，让角色状态与账号仓同生共死。
func (s *ItemService) TransformPrimers(
	ctx context.Context,
	role database.Character,
	r protocol.PrimerTransformRequest,
	payOption int,
) (database.Character, inventory.PrimerTransformReceipt, bool, error) {
	var result inventory.PrimerTransformReceipt
	fail := func(e error) (database.Character, inventory.PrimerTransformReceipt, bool, error) {
		return role, result, false, e
	}
	plan, e := s.Items.PlanPrimerTransform(InventoryRole(role), r, payOption)
	result = plan.Receipt
	if e != nil {
		return fail(e)
	}
	// 变换费用同样**背包优先、不足部分从账号金库同步调取**（业主确认的官方语义）：
	// 提交覆盖 角色 + 账号材料(灵魂/返还) + 金库金币，一个事务原子落账。
	saved, _, _, applied, e := s.Store.CommitAccountVault(ctx, role.AccountID, role.ID,
		s.Items.Catalog.Source.SaveIdentity(), plan.Key, 2381,
		func(current database.Character, accountRaw json.RawMessage, vault database.AccountVaultState) (json.RawMessage, json.RawMessage, database.AccountVaultState, error) {
			state, account, vaultGold, receipt, err := s.Items.PreparePrimerTransform(InventoryRole(current), accountRaw, vault.Gold, plan)
			result = receipt
			vault.Gold = vaultGold
			return state, account, vault, err
		})
	if e != nil {
		return fail(e)
	}
	saved.WireID = role.WireID
	return saved, result, applied, nil
}

func (s *ItemService) Disjoint(
	ctx context.Context,
	role database.Character,
	r protocol.DisjointItemRequest,
) (database.Character, inventory.DisjointReceipt, bool, error) {
	var result inventory.DisjointReceipt
	fail := func(e error) (database.Character, inventory.DisjointReceipt, bool, error) {
		return role, result, false, e
	}
	plan, e := s.Items.PlanDisjoint(InventoryRole(role), r)
	if e != nil {
		return fail(e)
	}
	key, slots := plan.Key, plan.Slots
	saved, applied, e := s.Store.CommitCharacterEvent(ctx, role.AccountID, role.ID,
		s.Items.Catalog.Source.SaveIdentity(), key, s.Items.Model,
		func(current database.Character) (json.RawMessage, json.RawMessage, error) {
			return s.Items.PrepareDisjoint(InventoryRole(current), r, plan)
		})
	if e != nil {
		return fail(e)
	}
	receipt, e := s.Store.CharacterEventReceipt(ctx, role.AccountID, role.ID, key)
	if e != nil {
		return fail(e)
	}
	if e = json.Unmarshal(receipt, &result); e != nil {
		return fail(e)
	}
	if result.Source != s.Items.Catalog.Source.SaveIdentity() || len(result.DeletedSlots) != len(slots) {
		return fail(fmt.Errorf("disjoint receipt conflict"))
	}
	saved.WireID = role.WireID
	return saved, result, applied, nil
}

// ItemService coordinates inventory item transitions with durable event commits.
type ItemService struct {
	Store *database.Store
	Items *inventory.ItemService
}
