package workflow

import (
	"context"
	"dfolan/internal/database"
	"dfolan/internal/loot"
	"encoding/json"
	"fmt"
)

// OpenBoxes settles one radiant box request: it spends the material and hands out
// every rolled prize inside the same character transaction, so a retried request
// cannot spend a second stack or advance pity twice.
func (s *LootService) OpenBoxes(ctx context.Context, role database.Character, box, count uint32) (database.Character, loot.BoxOpenReceipt, bool, error) {
	var out loot.BoxOpenReceipt
	fail := func(e error) (database.Character, loot.BoxOpenReceipt, bool, error) {
		return role, out, false, e
	}
	plan, e := s.Loot.PlanBoxOpen(LootRole(role), box, count)
	if e != nil {
		return fail(e)
	}
	key := fmt.Sprintf("boxopen:%d:%d:%d", box, count, plan.Opens)
	saved, applied, e := s.Store.CommitCharacterPremiumEvent(ctx, role.AccountID, role.ID,
		s.Loot.Catalog.Source.SaveIdentity(), key, s.Loot.Rules.Model,
		func(current database.Character) (json.RawMessage, json.RawMessage, []database.CashPremiumActivation, error) {
			state, receipt, premiums, err := s.Loot.PrepareBoxOpen(LootRole(current), box, count, plan, resolveLootContract)
			return state, receipt, lootPremiumActivations(premiums), err
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
	if out.Box != box || out.Source != s.Loot.Catalog.Source.SaveIdentity() {
		return fail(fmt.Errorf("box open receipt conflict"))
	}
	saved.WireID = role.WireID
	return saved, out, applied, nil
}

// RepairBoxRewards 在登录背包还原前修复遗留奖励，重复登录不重复续期。
func (s *LootService) RepairBoxRewards(ctx context.Context, role database.Character) (database.Character, bool, error) {
	needed, err := s.Loot.NeedsBoxRewardRepair(LootRole(role), resolveLootContract)
	if err != nil || !needed {
		return role, false, err
	}
	saved, applied, err := s.Store.CommitCharacterPremiumEvent(ctx, role.AccountID, role.ID, s.Loot.Catalog.Source.SaveIdentity(), "box-reward-repair-v1", s.Loot.Rules.Model,
		func(current database.Character) (json.RawMessage, json.RawMessage, []database.CashPremiumActivation, error) {
			state, receipt, premiums, err := s.Loot.PrepareBoxRewardRepair(LootRole(current), resolveLootContract)
			return state, receipt, lootPremiumActivations(premiums), err
		})
	saved.WireID = role.WireID
	return saved, applied, err
}
