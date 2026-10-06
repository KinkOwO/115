package workflow

import (
	"context"
	"dfolan/internal/database"
	"dfolan/internal/game/protocol"
	"dfolan/internal/inventory"
	"encoding/json"
	"fmt"
)

// WearService coordinates durable equipment transitions; rules remain in inventory.
type WearService struct {
	inventory.WearService
	Store *database.Store
}

func (s *WearService) Move(ctx context.Context, role database.Character, key string, r protocol.ItemMoveRequest) (database.Character, bool, error) {
	if s == nil || s.Store == nil {
		return role, false, fmt.Errorf("wear storage unavailable")
	}
	saved, applied, e := s.Store.CommitCharacterEvent(ctx, role.AccountID, role.ID, role.ConfigVersion, key, "ordinary-equipment-move-v1", func(current database.Character) (json.RawMessage, json.RawMessage, error) {
		bag, err := inventory.ReadBag(current.State)
		if err != nil {
			return nil, nil, err
		}
		_, migrated, err := s.rules().NormalizeCloneAvatars(bag)
		if err != nil {
			return nil, nil, err
		}
		raw, e := s.rules().MoveOrdinary(InventoryRole(current), r)
		if e != nil {
			return nil, nil, e
		}
		receipt, e := json.Marshal(r)
		if migrated {
			receipt, e = json.Marshal(struct {
				protocol.ItemMoveRequest
				Before, After json.RawMessage
			}{r, current.State, raw})
		}
		return raw, receipt, e
	})
	saved.WireID = role.WireID
	return saved, applied, e
}

func (s *WearService) ApplyAmplifyGrimoire(ctx context.Context, role database.Character, key string, r protocol.AmplifyOptionRequest) (database.Character, inventory.AmplifyGrimoireReceipt, error) {
	var out inventory.AmplifyGrimoireReceipt
	if s == nil || s.Store == nil || s.Catalog == nil {
		return role, out, fmt.Errorf("打红字需要有效装备目录及角色存档")
	}

	if err := s.WearService.ValidateApplyAmplifyGrimoire(r); err != nil {
		return role, out, err
	}
	golden, pure, value := inventory.ClassifyAmplifyBook(r.BookTemplate)

	return commitEquipmentEvent(ctx, s.Store, role, key, "amplify-grimoire-v1", func(current database.Character) (json.RawMessage, inventory.AmplifyGrimoireReceipt, error) {
		return s.WearService.ApplyAmplifyGrimoire(InventoryRole(current), r, value, golden, pure)
	})
}

func (s *WearService) ApplyAmplifyTicket(ctx context.Context, role database.Character, key string, r protocol.ReinforcementRequest) (database.Character, inventory.AmplifyTicketReceipt, error) {
	var out inventory.AmplifyTicketReceipt
	if s == nil || s.Store == nil || s.Catalog == nil {
		return role, out, fmt.Errorf("增幅券需要有效装备目录及角色存档")
	}

	if err := s.WearService.ValidateApplyAmplifyTicket(r); err != nil {
		return role, out, err
	}
	return commitEquipmentEvent(ctx, s.Store, role, key, "amplify-ticket-v1", func(current database.Character) (json.RawMessage, inventory.AmplifyTicketReceipt, error) {
		return s.WearService.ApplyAmplifyTicket(InventoryRole(current), r)
	})
}

func (s *WearService) ApplyAmplifyUpgrade(ctx context.Context, role database.Character, key string, r protocol.ReinforcementRequest) (database.Character, inventory.AmplifyUpgradeReceipt, error) {
	var out inventory.AmplifyUpgradeReceipt
	if s == nil || s.Store == nil || s.Catalog == nil {
		return role, out, fmt.Errorf("增幅需要有效装备目录及角色存档")
	}

	if err := s.WearService.ValidateApplyAmplifyUpgrade(r); err != nil {
		return role, out, err
	}
	return commitEquipmentEvent(ctx, s.Store, role, key, "amplify-upgrade-v1", func(current database.Character) (json.RawMessage, inventory.AmplifyUpgradeReceipt, error) {
		return s.WearService.ApplyAmplifyUpgrade(InventoryRole(current), r)
	})
}

func (s *WearService) ApplyEnchantByBead(ctx context.Context, role database.Character, key string, r protocol.EnchantByBeadRequest) (database.Character, inventory.EnchantReceipt, error) {
	var out inventory.EnchantReceipt
	if s == nil || s.Store == nil || s.Catalog == nil {
		return role, out, fmt.Errorf("附魔需要有效装备目录及角色存档")
	}

	if err := s.WearService.ValidateApplyEnchantByBead(r); err != nil {
		return role, out, err
	}
	return commitEquipmentEvent(ctx, s.Store, role, key, "enchant-bead-v1", func(current database.Character) (json.RawMessage, inventory.EnchantReceipt, error) {
		return s.WearService.ApplyEnchantByBead(InventoryRole(current), r)
	})
}

func (s *WearService) ApplyRefine(ctx context.Context, role database.Character, key string, r protocol.RefineRequest) (database.Character, inventory.RefineReceipt, error) {
	var out inventory.RefineReceipt
	if s == nil || s.Store == nil || s.Catalog == nil {
		return role, out, fmt.Errorf("锻造需要有效装备目录及角色存档")
	}

	if err := s.WearService.ValidateApplyRefine(r); err != nil {
		return role, out, err
	}
	return commitEquipmentEvent(ctx, s.Store, role, key, "refine-v1", func(current database.Character) (json.RawMessage, inventory.RefineReceipt, error) {
		return s.WearService.ApplyRefine(InventoryRole(current), r)
	})
}

func (s *WearService) ApplyInherit(ctx context.Context, role database.Character, key string, r protocol.InheritRequest) (database.Character, []inventory.InheritReceipt, error) {
	if s == nil || s.Store == nil || s.Catalog == nil {
		return role, nil, fmt.Errorf("装备继承需要有效装备目录及角色存档")
	}

	if err := s.WearService.ValidateApplyInherit(r); err != nil {
		return role, nil, err
	}
	return commitEquipmentEvent(ctx, s.Store, role, key, "inherit-v1", func(current database.Character) (json.RawMessage, []inventory.InheritReceipt, error) {
		return s.WearService.ApplyInherit(InventoryRole(current), r.Entries)
	})
}

func (s *WearService) ReinforceWithTicket(ctx context.Context, role database.Character, key string, r protocol.ReinforcementRequest) (database.Character, inventory.ReinforcementReceipt, error) {
	var out inventory.ReinforcementReceipt
	if s == nil || s.Store == nil || s.Catalog == nil || s.Catalog.Source.SaveIdentity() != role.ConfigVersion {
		return role, out, fmt.Errorf("强化需要有效装备目录及角色存档")
	}
	saved, out, err := commitEquipmentEvent(ctx, s.Store, role, key, "fixed-reinforcement-ticket-v1", func(current database.Character) (json.RawMessage, inventory.ReinforcementReceipt, error) {
		return s.ApplyReinforcement(InventoryRole(current), r)
	})

	if err == nil && out.Request != r {
		err = fmt.Errorf("强化回执与请求不符")
	}
	return saved, out, err
}

func (s *WearService) ReinforceWithMaterial(ctx context.Context, role database.Character, key string, r protocol.ReinforcementRequest) (database.Character, inventory.GoldReinforcementReceipt, error) {
	var out inventory.GoldReinforcementReceipt
	if s == nil || s.Store == nil || s.Catalog == nil || s.Catalog.Source.SaveIdentity() != role.ConfigVersion {
		return role, out, fmt.Errorf("金币强化需要有效装备目录及角色存档")
	}

	if err := s.WearService.ValidateReinforceWithMaterial(r); err != nil {
		return role, out, err
	}
	applied := false
	saved, _, _, err := s.Store.CommitAccountMaterialEvent(ctx, role.AccountID, role.ID, role.ConfigVersion, key, "gold-reinforcement-v1", func(current database.Character, counts json.RawMessage) (json.RawMessage, json.RawMessage, error) {
		next, nextCounts, receipt, err := s.ApplyGoldReinforcement(InventoryRole(current), counts, key, r)
		if err != nil {
			return nil, nil, err
		}
		out = receipt
		applied = true
		return next, nextCounts, nil
	})
	if err != nil {
		return role, out, err
	}
	if !applied {
		// 重放：事务里没有再次执行，回执从存档字段取回。
		stored, e := inventory.ReadGoldReinforcementReceipt(saved.State, key)
		if e != nil {
			return role, out, e
		}
		out = stored
	}
	saved.WireID = role.WireID
	if _, err = protocol.ReinforcementGoldReply(r, out.MaterialRemaining, out.Old, out.Level, out.Result); err != nil {
		return role, out, err
	}
	return saved, out, nil
}

func (s *WearService) CommitKnightDeck(ctx context.Context, role database.Character, key string, deck [protocol.KnightDeckSize]uint32) (database.Character, bool, error) {
	if s == nil || s.Store == nil {
		return role, false, inventory.ShieldRefusal("knight shield storage unavailable")
	}
	saved, applied, err := s.Store.CommitCharacterEvent(ctx, role.AccountID, role.ID, role.ConfigVersion, key, "knight-shield-deck-v1", func(current database.Character) (json.RawMessage, json.RawMessage, error) {
		raw, result, e := s.rules().ApplyKnightDeck(InventoryRole(current), deck)
		if e != nil {
			return nil, nil, e
		}
		receipt, e := json.Marshal(struct {
			Deck [protocol.KnightDeckSize]uint32 `json:"deck"`
			inventory.KnightDeckApply
		}{deck, result})
		return raw, receipt, e
	})
	saved.WireID = role.WireID
	return saved, applied, err
}

func (s *WearService) rules() *inventory.WearService {
	rules := s.WearService
	rules.PremiumStore = PremiumReader{Store: s.Store}
	return &rules
}
