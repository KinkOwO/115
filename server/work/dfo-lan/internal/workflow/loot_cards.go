package workflow

import (
	"context"
	"dfolan/internal/dungeon"
	"dfolan/internal/loot"
	"dfolan/internal/storage"
	"encoding/json"
	"fmt"
)

// Freeze is a durable plan, not an award. No reward is granted until PickCard.
func (s *LootService) FreezeCards(ctx context.Context, role storage.Character, d *dungeon.Session, r loot.CardRules, seed uint32) (loot.CardPlan, error) {
	var p loot.CardPlan
	if d == nil || !d.Completed() || role.ConfigVersion != s.Loot.Catalog.Source.SaveIdentity() {
		return p, fmt.Errorf("card plan before owned completion")
	}
	if d.Definition.ID == loot.BlackPurgatorySquadDungeon {
		return s.FreezeBlackPurgatoryCards(ctx, role, d, seed)
	}
	p, e := s.Loot.PlanCards(LootRole(role), d, r, seed)
	if e != nil {
		return p, e
	}
	key := "cardplan:" + d.RunID
	_, _, e = s.Store.CommitCharacterEvent(ctx, role.AccountID, role.ID, p.Source, key, r.Model, func(current storage.Character) (json.RawMessage, json.RawMessage, error) {
		b, e := json.Marshal(p)
		return current.State, b, e
	})
	if e != nil {
		return loot.CardPlan{}, e
	}
	b, e := s.Store.CharacterEventReceipt(ctx, role.AccountID, role.ID, key)
	if e != nil {
		return loot.CardPlan{}, e
	}
	if e = json.Unmarshal(b, &p); e != nil {
		return p, e
	}
	if p.Run != d.RunID || p.Source != s.Loot.Catalog.Source.SaveIdentity() || p.Model != r.Model || p.Gold == 0 {
		return p, fmt.Errorf("card plan source conflict")
	}
	return p, nil
}
func (s *LootService) PickCard(ctx context.Context, role storage.Character, d *dungeon.Session, p loot.CardPlan, index byte) (storage.Character, loot.CardReceipt, bool, error) {
	var receipt loot.CardReceipt
	if index > 3 || d == nil || !d.Completed() || p.Run != d.RunID || p.Source != s.Loot.Catalog.Source.SaveIdentity() {
		return role, receipt, false, fmt.Errorf("invalid owned card selection")
	}
	return s.pickFrozenCard(ctx, role, p, index)
}

func (s *LootService) pickFrozenCard(ctx context.Context, role storage.Character, p loot.CardPlan, index byte) (storage.Character, loot.CardReceipt, bool, error) {
	var receipt loot.CardReceipt
	if index > 3 || p.Source != s.Loot.Catalog.Source.SaveIdentity() || role.ConfigVersion != p.Source || p.Run == "" {
		return role, receipt, false, fmt.Errorf("翻牌奖励归属无效")
	}
	// Re-read frozen server plan; values received from the transport never
	// choose an item, amount or reward formula.
	b, e := s.Store.CharacterEventReceipt(ctx, role.AccountID, role.ID, "cardplan:"+p.Run)
	if e != nil {
		return role, receipt, false, e
	}
	var stored loot.CardPlan
	if e = json.Unmarshal(b, &stored); e != nil {
		return role, receipt, false, e
	}
	if stored != p {
		return role, receipt, false, fmt.Errorf("unfrozen card plan")
	}
	key := "cardpick:" + p.Run
	saved, applied, e := s.Store.CommitCharacterEvent(ctx, role.AccountID, role.ID, p.Source, key, p.Model, func(current storage.Character) (json.RawMessage, json.RawMessage, error) {
		return s.Loot.PrepareFrozenCard(LootRole(current), p, index)
	})
	if e != nil {
		return role, receipt, false, e
	}
	b, e = s.Store.CharacterEventReceipt(ctx, role.AccountID, role.ID, key)
	if e != nil {
		return role, receipt, false, e
	}
	if e = json.Unmarshal(b, &receipt); e != nil {
		return role, receipt, false, e
	}
	if receipt.Plan != p || receipt.Index > 3 {
		return role, receipt, false, fmt.Errorf("card receipt conflict")
	}
	saved.WireID = role.WireID
	return saved, receipt, applied, nil
}
