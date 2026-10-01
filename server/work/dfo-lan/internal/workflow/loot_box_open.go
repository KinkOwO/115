package workflow

import (
	"context"
	"dfolan/internal/loot"
	"dfolan/internal/storage"
	"encoding/json"
	"fmt"
)

// OpenBoxes settles one radiant box request: it spends the material and hands out
// every rolled prize inside the same character transaction, so a retried request
// cannot spend a second stack or advance pity twice.
func (s *LootService) OpenBoxes(ctx context.Context, role storage.Character, box, count uint32) (storage.Character, loot.BoxOpenReceipt, bool, error) {
	var out loot.BoxOpenReceipt
	fail := func(e error) (storage.Character, loot.BoxOpenReceipt, bool, error) {
		return role, out, false, e
	}
	plan, e := s.Loot.PlanBoxOpen(LootRole(role), box, count)
	if e != nil {
		return fail(e)
	}
	key := fmt.Sprintf("boxopen:%d:%d:%d", box, count, plan.Opens)
	saved, applied, e := s.Store.CommitCharacterPremiumEvent(ctx, role.AccountID, role.ID,
		s.Loot.Catalog.Source.SaveIdentity(), key, s.Loot.Rules.Model,
		func(current storage.Character) (json.RawMessage, json.RawMessage, []storage.CashPremiumActivation, error) {
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
