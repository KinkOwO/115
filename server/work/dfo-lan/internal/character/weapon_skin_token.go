package character

import (
	"context"
	"dfolan/internal/inventory"
	"encoding/json"
	"fmt"
)

func (s *Service) weaponShapeReceipt(ctx context.Context, role Character, key string) (json.RawMessage, error) {
	receipts, ok := s.Store.(interface {
		CharacterEventReceipt(context.Context, int64, int64, string) (json.RawMessage, error)
	})
	if !ok {
		return nil, fmt.Errorf("weapon shape replay receipt unavailable")
	}
	return receipts.CharacterEventReceipt(ctx, role.AccountID, role.ID, key)
}

func (s *Service) weaponShapeActor(cur Character) (inventory.ReplicationActor, error) {
	prof, ok := s.Catalog.Professions[cur.Profession]
	if !ok {
		return inventory.ReplicationActor{}, fmt.Errorf("weapon shape profession unavailable")
	}
	var state struct {
		Advancement byte `json:"advancement"`
		Awakening   byte `json:"awakening"`
	}
	if e := json.Unmarshal(cur.State, &state); e != nil {
		return inventory.ReplicationActor{}, e
	}
	return inventory.ReplicationActor{Job: prof.Job, Advancement: state.Advancement, CanUseSkill: s.canLearnSkill(cur.Profession, int(state.Advancement), int(state.Awakening))}, nil
}

func (s *Service) GenerateWeaponShape(ctx context.Context, role Character, slot uint16, mode uint32) (Character, inventory.WeaponReplication, bool, error) {
	if s == nil || s.Store == nil {
		return role, inventory.WeaponReplication{}, false, fmt.Errorf("weapon shape generation without store")
	}
	recipe, ok := s.WeaponSkinRecipes[mode]
	if !ok {
		return role, inventory.WeaponReplication{}, false, fmt.Errorf("weapon shape recipe %d unavailable", mode)
	}
	before, e := inventory.ReadBag(role.State)
	if e != nil {
		return role, inventory.WeaponReplication{}, false, e
	}
	key := fmt.Sprintf("weapon-shape-product-v1:%d:%d:%d:%d", role.ID, before.WeaponSkinTokenSeq, slot, mode)
	var cost inventory.WeaponReplication
	saved, applied, e := s.Store.CommitCharacterEvent(ctx, role.AccountID, role.ID, role.ConfigVersion, key, "weapon-shape-product-v1", func(cur Character) (json.RawMessage, json.RawMessage, error) {
		bag, e := inventory.ReadBag(cur.State)
		if e != nil {
			return nil, nil, e
		}
		if bag.WeaponSkinTokenSeq != before.WeaponSkinTokenSeq {
			return nil, nil, fmt.Errorf("stale weapon shape generation")
		}
		actor, e := s.weaponShapeActor(cur)
		if e != nil {
			return nil, nil, e
		}
		next, c, e := bag.GenerateWeaponShape(slot, s.Equipment, s.WeaponSkinBagRules, recipe, actor)
		if e != nil {
			return nil, nil, e
		}
		cost = c
		state, e := inventory.SaveBag(cur.State, next)
		if e != nil {
			return nil, nil, e
		}
		receipt, e := json.Marshal(cost)
		return state, receipt, e
	})
	if e != nil {
		return role, cost, false, e
	}
	if !applied {
		raw, err := s.weaponShapeReceipt(ctx, role, key)
		if err != nil {
			return saved, cost, false, err
		}
		if err = json.Unmarshal(raw, &cost); err != nil {
			return saved, cost, false, err
		}
	}
	saved.WireID = role.WireID
	return saved, cost, applied, e
}

func (s *Service) RegisterWeaponShape(ctx context.Context, role Character, slot uint16, template uint32) (Character, uint32, bool, error) {
	if s == nil || s.Store == nil {
		return role, 0, false, fmt.Errorf("weapon shape registration without store")
	}
	known := false
	for _, r := range s.WeaponSkinRecipes {
		if r.Product == template {
			known = true
		}
	}
	if !known {
		return role, 0, false, fmt.Errorf("weapon shape product is not in source recipes")
	}
	before, e := inventory.ReadBag(role.State)
	if e != nil {
		return role, 0, false, e
	}
	key := fmt.Sprintf("weapon-shape-register-v1:%d:%d:%d:%d", role.ID, before.WeaponSkinTokenSeq, slot, template)
	var skin uint32
	saved, applied, e := s.Store.CommitCharacterEvent(ctx, role.AccountID, role.ID, role.ConfigVersion, key, "weapon-shape-register-v1", func(cur Character) (json.RawMessage, json.RawMessage, error) {
		bag, e := inventory.ReadBag(cur.State)
		if e != nil {
			return nil, nil, e
		}
		if bag.WeaponSkinTokenSeq != before.WeaponSkinTokenSeq {
			return nil, nil, fmt.Errorf("stale weapon shape registration")
		}
		actor, e := s.weaponShapeActor(cur)
		if e != nil {
			return nil, nil, e
		}
		next, id, e := bag.RegisterWeaponShape(slot, template, s.Equipment, actor)
		if e != nil {
			return nil, nil, e
		}
		skin = id
		state, e := inventory.SaveBag(cur.State, next)
		if e != nil {
			return nil, nil, e
		}
		receipt, e := json.Marshal(map[string]any{"slot": slot, "template": template, "weapon": id})
		return state, receipt, e
	})
	if e != nil {
		return role, 0, false, e
	}
	if !applied {
		raw, err := s.weaponShapeReceipt(ctx, role, key)
		if err != nil {
			return saved, 0, false, err
		}
		var receipt struct {
			Weapon uint32 `json:"weapon"`
		}
		if err = json.Unmarshal(raw, &receipt); err != nil {
			return saved, 0, false, err
		}
		skin = receipt.Weapon
	}
	saved.WireID = role.WireID
	return saved, skin, applied, e
}
