package workflow

import (
	"context"
	"dfolan/internal/dungeon"
	"dfolan/internal/game/protocol"
	"dfolan/internal/loot"
	"dfolan/internal/storage"
	"encoding/json"
	"fmt"
)

func (s *LootService) Pickup(ctx context.Context, role storage.Character, session *loot.Session, d *dungeon.Session, r protocol.PickupRequest) (storage.Character, loot.PickupReceipt, bool, error) {
	plan, e := s.Loot.PlanPickup(LootRole(role), session, d, r)
	if e != nil {
		return role, loot.PickupReceipt{}, false, e
	}
	return s.pickupPlan(ctx, role, plan)
}

// AutoPickup 与手动拾取共用持久化事务、幂等回执及失败保留地面物品的语义。
func (s *LootService) AutoPickup(ctx context.Context, role storage.Character, session *loot.Session, d *dungeon.Session, object uint32) (storage.Character, loot.PickupReceipt, bool, error) {
	plan, err := s.Loot.PlanAutoPickup(LootRole(role), session, d, object)
	if err != nil {
		return role, loot.PickupReceipt{}, false, err
	}
	return s.pickupPlan(ctx, role, plan)
}

func (s *LootService) pickupPlan(ctx context.Context, role storage.Character, plan loot.PickupPlan) (storage.Character, loot.PickupReceipt, bool, error) {
	var result loot.PickupReceipt
	fail := func(e error) (storage.Character, loot.PickupReceipt, bool, error) { return role, result, false, e }
	drop := plan.Drop
	if drop.BlackPurgatoryIndex != 0 {
		saved, receipt, applied, err := s.pickBlackPurgatoryBoss(ctx, role, drop.Run, drop.BlackPurgatoryIndex, drop.Award)
		if err != nil {
			return fail(err)
		}
		return saved, loot.PickupReceipt{Run: drop.Run, Map: drop.Map, Object: drop.Object, Award: receipt.Award, Destination: receipt.Destination, Source: s.Loot.Catalog.Source.SaveIdentity()}, applied, nil
	}
	key := fmt.Sprintf("pickup:%s:%d", drop.Run, drop.Object)
	saved, applied, e := s.Store.CommitCharacterEvent(ctx, role.AccountID, role.ID, s.Loot.Catalog.Source.SaveIdentity(), key, s.Loot.Rules.Model, func(current storage.Character) (json.RawMessage, json.RawMessage, error) {
		return s.Loot.PreparePickup(LootRole(current), plan)
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
	if result.Source != s.Loot.Catalog.Source.SaveIdentity() || result.Run != drop.Run || result.Object != drop.Object || result.Award != drop.Award {
		return fail(fmt.Errorf("pickup receipt conflict"))
	}
	saved.WireID = role.WireID
	return saved, result, applied, nil
}
