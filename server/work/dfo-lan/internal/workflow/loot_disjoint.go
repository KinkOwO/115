package workflow

import (
	"context"
	"dfolan/internal/game/protocol"
	"dfolan/internal/loot"
	"dfolan/internal/storage"
	"encoding/json"
	"fmt"
)

func (s *LootService) Disjoint(
	ctx context.Context,
	role storage.Character,
	r protocol.DisjointItemRequest,
) (storage.Character, loot.DisjointReceipt, bool, error) {
	var result loot.DisjointReceipt
	fail := func(e error) (storage.Character, loot.DisjointReceipt, bool, error) {
		return role, result, false, e
	}
	plan, e := s.Loot.PlanDisjoint(LootRole(role), r)
	if e != nil {
		return fail(e)
	}
	key, slots := plan.Key, plan.Slots
	saved, applied, e := s.Store.CommitCharacterEvent(ctx, role.AccountID, role.ID,
		s.Loot.Catalog.Source.SaveIdentity(), key, s.Loot.Rules.Model,
		func(current storage.Character) (json.RawMessage, json.RawMessage, error) {
			return s.Loot.PrepareDisjoint(LootRole(current), r, plan)
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
	if result.Source != s.Loot.Catalog.Source.SaveIdentity() || len(result.DeletedSlots) != len(slots) {
		return fail(fmt.Errorf("disjoint receipt conflict"))
	}
	saved.WireID = role.WireID
	return saved, result, applied, nil
}
