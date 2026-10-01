package workflow

import (
	"context"
	"dfolan/internal/storage"
	"encoding/json"
)

// RepairBoxRewards 在登录背包还原前修复遗留奖励，重复登录不重复续期。
func (s *LootService) RepairBoxRewards(ctx context.Context, role storage.Character) (storage.Character, bool, error) {
	needed, err := s.Loot.NeedsBoxRewardRepair(LootRole(role), resolveLootContract)
	if err != nil || !needed {
		return role, false, err
	}
	saved, applied, err := s.Store.CommitCharacterPremiumEvent(ctx, role.AccountID, role.ID, s.Loot.Catalog.Source.SaveIdentity(), "box-reward-repair-v1", s.Loot.Rules.Model,
		func(current storage.Character) (json.RawMessage, json.RawMessage, []storage.CashPremiumActivation, error) {
			state, receipt, premiums, err := s.Loot.PrepareBoxRewardRepair(LootRole(current), resolveLootContract)
			return state, receipt, lootPremiumActivations(premiums), err
		})
	saved.WireID = role.WireID
	return saved, applied, err
}
