package workflow

import (
	"context"
	"dfolan/internal/adventure"
	"dfolan/internal/database"
	"dfolan/internal/game/protocol"
	"dfolan/internal/loot"
	"encoding/json"
	"fmt"
)

// Consume settles one stackable use durably. The decrement and its receipt
// commit in the same character transaction the pickup path uses, so a replayed
// or retried request returns the original receipt instead of spending a second
// unit — the native client sends this on a hotkey press, which is exactly the
// kind of input that repeats.
//
// The recovery effect is the client's own: the native success acknowledgement
// carries no restored amount, so nothing is invented here.
func (s *LootService) Consume(ctx context.Context, role database.Character, r protocol.UseStackableRequest) (database.Character, loot.ConsumeReceipt, bool, error) {
	var out loot.ConsumeReceipt
	fail := func(e error) (database.Character, loot.ConsumeReceipt, bool, error) {
		return role, out, false, e
	}
	key, e := s.Loot.ConsumeKey(LootRole(role), r)
	if e != nil {
		return fail(e)
	}
	seasonRules, e := adventure.CurrentSeason()
	if e != nil {
		return fail(e)
	}
	_, seasonCapsule := seasonRules.Capsules[r.Template]
	saved, applied, e := s.Store.CommitCharacterPremiumEvent(ctx, role.AccountID, role.ID,
		s.Loot.Catalog.Source.SaveIdentity(), key, s.Loot.Rules.Model,
		func(current database.Character) (json.RawMessage, json.RawMessage, []database.CashPremiumActivation, error) {
			state, receipt, premiums, err := s.Loot.PrepareConsume(LootRole(current), r, seasonCapsule, adventure.ApplySeasonCapsule, resolveLootContract)
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
	if out.Source != s.Loot.Catalog.Source.SaveIdentity() || out.Template != r.Template || out.Slot != r.Slot {
		return fail(fmt.Errorf("consume receipt conflict"))
	}
	out.EventKey = key
	saved.WireID = role.WireID
	return saved, out, applied, nil
}
