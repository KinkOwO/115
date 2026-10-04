package workflow

import (
	"context"
	"dfolan/internal/adventure"
	"dfolan/internal/cashshop"
	"dfolan/internal/database"
	"dfolan/internal/game/protocol"
	"dfolan/internal/inventory"
	"encoding/json"
	"fmt"
)

func resolveItemContract(template uint32) (inventory.PremiumActivation, bool) {
	contract, ok := cashshop.ResolveContractItem(template)
	return inventory.PremiumActivation{Type: contract.Type, DurationSecond: contract.DurationSecond}, ok
}

func itemPremiumActivations(values []inventory.PremiumActivation) []database.CashPremiumActivation {
	if values == nil {
		return nil
	}
	out := make([]database.CashPremiumActivation, len(values))
	for i, v := range values {
		out[i] = database.CashPremiumActivation{Type: v.Type, DurationSecond: v.DurationSecond}
	}
	return out
}

// Consume settles one stackable use durably. The decrement and its receipt
// commit in the same character transaction the pickup path uses, so a replayed
// or retried request returns the original receipt instead of spending a second
// unit — the native client sends this on a hotkey press, which is exactly the
// kind of input that repeats.
//
// The recovery effect is the client's own: the native success acknowledgement
// carries no restored amount, so nothing is invented here.
func (s *ItemService) Consume(ctx context.Context, role database.Character, r protocol.UseStackableRequest) (database.Character, inventory.ConsumeReceipt, bool, error) {
	var out inventory.ConsumeReceipt
	fail := func(e error) (database.Character, inventory.ConsumeReceipt, bool, error) {
		return role, out, false, e
	}
	key, e := s.Items.ConsumeKey(InventoryRole(role), r)
	if e != nil {
		return fail(e)
	}
	seasonRules, e := adventure.CurrentSeason()
	if e != nil {
		return fail(e)
	}
	_, seasonCapsule := seasonRules.Capsules[r.Template]
	saved, applied, e := s.Store.CommitCharacterPremiumEvent(ctx, role.AccountID, role.ID,
		s.Items.Catalog.Source.SaveIdentity(), key, s.Items.Model,
		func(current database.Character) (json.RawMessage, json.RawMessage, []database.CashPremiumActivation, error) {
			state, receipt, premiums, err := s.Items.PrepareConsume(InventoryRole(current), r, seasonCapsule, adventure.ApplySeasonCapsule, resolveItemContract)
			return state, receipt, itemPremiumActivations(premiums), err
		})
	if e != nil {
		return fail(e)
	}
	receipt, e := s.Store.CharacterEventReceipt(ctx, role.AccountID, role.ID, key)
	if e != nil {
		return fail(e)
	}
	if e = json.Unmarshal(receipt, &out); e != nil {
		return fail(e)
	}
	if out.Source != s.Items.Catalog.Source.SaveIdentity() || out.Template != r.Template || out.Slot != r.Slot {
		return fail(fmt.Errorf("consume receipt conflict"))
	}
	out.EventKey = key
	saved.WireID = role.WireID
	return saved, out, applied, nil
}

// OpenBoxes settles one radiant box request: it spends the material and hands out
// every rolled prize inside the same character transaction, so a retried request
// cannot spend a second stack or advance pity twice.
func (s *ItemService) OpenBoxes(ctx context.Context, role database.Character, box, count uint32) (database.Character, inventory.BoxOpenReceipt, bool, error) {
	var out inventory.BoxOpenReceipt
	fail := func(e error) (database.Character, inventory.BoxOpenReceipt, bool, error) {
		return role, out, false, e
	}
	plan, e := s.Items.PlanBoxOpen(InventoryRole(role), box, count)
	if e != nil {
		return fail(e)
	}
	key := fmt.Sprintf("boxopen:%d:%d:%d", box, count, plan.Opens)
	saved, applied, e := s.Store.CommitCharacterPremiumEvent(ctx, role.AccountID, role.ID,
		s.Items.Catalog.Source.SaveIdentity(), key, s.Items.Model,
		func(current database.Character) (json.RawMessage, json.RawMessage, []database.CashPremiumActivation, error) {
			state, receipt, premiums, err := s.Items.PrepareBoxOpen(InventoryRole(current), box, count, plan, resolveItemContract)
			return state, receipt, itemPremiumActivations(premiums), err
		})
	if e != nil {
		return fail(e)
	}
	receipt, e := s.Store.CharacterEventReceipt(ctx, role.AccountID, role.ID, key)
	if e != nil {
		return fail(e)
	}
	if e = json.Unmarshal(receipt, &out); e != nil {
		return fail(e)
	}
	if out.Box != box || out.Source != s.Items.Catalog.Source.SaveIdentity() {
		return fail(fmt.Errorf("box open receipt conflict"))
	}
	saved.WireID = role.WireID
	return saved, out, applied, nil
}

// RepairBoxRewards 在登录背包还原前修复遗留奖励，重复登录不重复续期。
func (s *ItemService) RepairBoxRewards(ctx context.Context, role database.Character) (database.Character, bool, error) {
	needed, err := s.Items.NeedsBoxRewardRepair(InventoryRole(role), resolveItemContract)
	if err != nil || !needed {
		return role, false, err
	}
	saved, applied, err := s.Store.CommitCharacterPremiumEvent(ctx, role.AccountID, role.ID, s.Items.Catalog.Source.SaveIdentity(), "box-reward-repair-v1", s.Items.Model,
		func(current database.Character) (json.RawMessage, json.RawMessage, []database.CashPremiumActivation, error) {
			state, receipt, premiums, err := s.Items.PrepareBoxRewardRepair(InventoryRole(current), resolveItemContract)
			return state, receipt, itemPremiumActivations(premiums), err
		})
	saved.WireID = role.WireID
	return saved, applied, err
}
