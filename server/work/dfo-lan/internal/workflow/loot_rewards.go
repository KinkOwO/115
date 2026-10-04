package workflow

import (
	"context"
	"dfolan/internal/database"
	"dfolan/internal/dungeon"
	"dfolan/internal/loot"
	"encoding/json"
	"errors"
	"fmt"
)

// LootService commits and replays state prepared by the loot domain.
type LootService struct {
	Store *database.Store
	Loot  *loot.Service
}

func (s *LootService) FreezeMoonReward(ctx context.Context, role database.Character, run *dungeon.Session, p loot.MoonRewardPolicy) (loot.MoonRewardPlan, error) {
	var out loot.MoonRewardPlan
	if s == nil || s.Loot == nil || s.Store == nil {
		return out, fmt.Errorf("Moon reward before owned final")
	}
	out, e := s.Loot.PlanMoonReward(LootRole(role), run, p)
	if e != nil {
		return out, e
	}

	body, e := json.Marshal(out)
	if e != nil {
		return out, e
	}
	_, _, e = s.Store.CommitCharacterEvent(ctx, role.AccountID, role.ID, p.Source, "moon-clear:"+run.RunID, loot.MoonRewardModel, func(current database.Character) (json.RawMessage, json.RawMessage, error) {
		return current.State, body, nil
	})
	if e != nil {
		return out, e
	}
	return s.ReadMoonReward(ctx, role, run.RunID)
}
func (s *LootService) ReadMoonReward(ctx context.Context, role database.Character, run string) (loot.MoonRewardPlan, error) {
	var p loot.MoonRewardPlan
	if s == nil || s.Loot == nil || s.Store == nil || !loot.MoonRunID(run) {
		return p, fmt.Errorf("invalid Moon reward owner/run")
	}
	raw, e := s.Store.CharacterEventReceipt(ctx, role.AccountID, role.ID, "moon-clear:"+run)
	if e != nil {
		return p, e
	}
	return s.Loot.DecodeMoonReward(LootRole(role), run, raw)
}
func (s *LootService) ClaimMoonReward(ctx context.Context, role database.Character, run string) (database.Character, bool, error) {
	p, e := s.ReadMoonReward(ctx, role, run)
	if e != nil {
		return role, false, e
	}
	saved, fresh, e := s.Store.CommitCharacterEvent(ctx, role.AccountID, role.ID, p.Source, "moon-grant:"+run, loot.MoonRewardModel, func(current database.Character) (json.RawMessage, json.RawMessage, error) {
		state, e := s.Loot.ApplyMoonRewards(current.State, p)
		if e != nil {
			return nil, nil, e
		}
		receipt, _ := json.Marshal(p)
		return state, receipt, nil
	})
	saved.WireID = role.WireID
	return saved, fresh, e
}

// Recovery is per owning character and only discovers durable, unclaimed
// clear plans. Full bags retain their plan. It never re-rolls with new policy.
func (s *LootService) RecoverMoonRewards(ctx context.Context, role database.Character) (database.Character, error) {
	runs, e := s.Store.PendingMoonRewardRuns(ctx, role.ID, loot.MoonRewardModel)
	if e != nil {
		return role, e
	}
	for _, run := range runs {
		saved, _, err := s.ClaimMoonReward(ctx, role, run)
		if err != nil {
			return role, err
		}
		role = saved
	}
	return role, nil
}

// Freeze is a durable plan, not an award. No reward is granted until PickCard.
func (s *LootService) FreezeCards(ctx context.Context, role database.Character, d *dungeon.Session, r loot.CardRules, seed uint32) (loot.CardPlan, error) {
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
	_, _, e = s.Store.CommitCharacterEvent(ctx, role.AccountID, role.ID, p.Source, key, r.Model, func(current database.Character) (json.RawMessage, json.RawMessage, error) {
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
func (s *LootService) PickCard(ctx context.Context, role database.Character, d *dungeon.Session, p loot.CardPlan, index byte) (database.Character, loot.CardReceipt, bool, error) {
	var receipt loot.CardReceipt
	if index > 3 || d == nil || !d.Completed() || p.Run != d.RunID || p.Source != s.Loot.Catalog.Source.SaveIdentity() {
		return role, receipt, false, fmt.Errorf("invalid owned card selection")
	}
	return s.pickFrozenCard(ctx, role, p, index)
}

func (s *LootService) pickFrozenCard(ctx context.Context, role database.Character, p loot.CardPlan, index byte) (database.Character, loot.CardReceipt, bool, error) {
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
	saved, applied, e := s.Store.CommitCharacterEvent(ctx, role.AccountID, role.ID, p.Source, key, p.Model, func(current database.Character) (json.RawMessage, json.RawMessage, error) {
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

func (s *LootService) FreezeBlackPurgatoryCards(ctx context.Context, role database.Character, d *dungeon.Session, seed uint32) (loot.CardPlan, error) {
	p := loot.CardPlan{}
	if s == nil || s.Store == nil {
		return p, fmt.Errorf("黑鸦奖励尚未加载或挑战未通关")
	}
	p, err := s.Loot.PlanBlackPurgatoryCards(LootRole(role), d, seed)
	if err != nil {
		return p, err
	}
	raw, err := s.Store.FreezeBlackPurgatoryReward(ctx, role.AccountID, role.ID, p.Source, p.Run, p.Model, func() (json.RawMessage, error) {
		return s.Loot.PrepareBlackPurgatoryCards(p, seed)
	})
	if err != nil {
		return loot.CardPlan{}, err
	}
	if err = json.Unmarshal(raw, &p); err != nil {
		return loot.CardPlan{}, err
	}
	if err = s.Loot.ValidateBlackPurgatoryCards(p, d); err != nil {
		return loot.CardPlan{}, err
	}
	return p, nil
}

// 地面拾取和掉线补领共用分支回执，不依赖重登后已失效的场景物体编号。
func (s *LootService) pickBlackPurgatoryBoss(ctx context.Context, role database.Character, run string, index byte, expected loot.Award) (database.Character, loot.BlackPurgatoryBossReceipt, bool, error) {
	var receipt loot.BlackPurgatoryBossReceipt
	if s == nil || s.Store == nil {
		return role, receipt, false, fmt.Errorf("黑鸦领主奖励归属无效")
	}
	if err := s.Loot.ValidateBlackPurgatoryBossOwner(LootRole(role), index); err != nil {
		return role, receipt, false, err
	}
	raw, err := s.Store.CharacterEventReceipt(ctx, role.AccountID, role.ID, "cardplan:"+run)
	if err != nil {
		return role, receipt, false, err
	}
	var plan loot.CardPlan
	if err = json.Unmarshal(raw, &plan); err != nil {
		return role, receipt, false, err
	}
	if err = s.Loot.ValidateBlackPurgatoryBossPlan(LootRole(role), plan, run, index, expected); err != nil {
		return role, receipt, false, err
	}
	key := fmt.Sprintf("black-purgatory-boss-pick:%s:%d", run, index)
	saved, applied, err := s.Store.CommitCharacterEvent(ctx, role.AccountID, role.ID, plan.Source, key, plan.BossModel,
		func(current database.Character) (json.RawMessage, json.RawMessage, error) {
			return s.Loot.PrepareBlackPurgatoryBoss(LootRole(current), run, index, expected)
		})
	if err != nil {
		return role, receipt, false, err
	}
	raw, err = s.Store.CharacterEventReceipt(ctx, role.AccountID, role.ID, key)
	if err == nil {
		err = json.Unmarshal(raw, &receipt)
	}
	if err != nil {
		return role, receipt, false, err
	}
	if receipt.Run != run || receipt.Index != index || receipt.Award != expected {
		return role, receipt, false, fmt.Errorf("黑鸦领主领取回执不一致")
	}
	saved.WireID = role.WireID
	return saved, receipt, applied, nil
}

// 只补发已持久化的奖单；普通副本旧回执不参与，背包满时保留待领。
func (s *LootService) RecoverBlackPurgatoryCards(ctx context.Context, role database.Character) (database.Character, error) {
	if s.Loot.BlackPurgatory == nil {
		return role, nil
	}
	var plans []loot.CardPlan
	err := s.Store.ReadPendingBlackPurgatoryRewards(ctx, role.AccountID, role.ID, loot.BlackPurgatoryCardModel, func(raw json.RawMessage) error {
		var p loot.CardPlan
		if err := json.Unmarshal(raw, &p); err != nil {
			return err
		}
		plans = append(plans, p)
		return nil
	})
	if err != nil {
		return role, err
	}
	var failures []error
	for _, p := range plans {
		var pending error
		saved, _, _, err := s.pickFrozenCard(ctx, role, p, 0)
		if err != nil {
			pending = err
		} else {
			role = saved
		}
		for i, item := range p.BossItems {
			if item == (loot.Award{}) {
				continue
			}
			saved, _, _, err := s.pickBlackPurgatoryBoss(ctx, role, p.Run, byte(i+1), item)
			if err != nil {
				pending = errors.Join(pending, err)
			} else {
				role = saved
			}
		}
		if pending == nil {
			saved, _, err = s.Store.CommitCharacterEvent(ctx, role.AccountID, role.ID, p.Source,
				"black-purgatory-recovered:"+p.Run, loot.BlackPurgatoryCardModel,
				func(current database.Character) (json.RawMessage, json.RawMessage, error) {
					data, err := json.Marshal(p.Run)
					return current.State, data, err
				})
			if err == nil {
				saved.WireID = role.WireID
				role = saved
			} else {
				pending = err
			}
		}
		if pending != nil {
			failures = append(failures, fmt.Errorf("挑战%s尚有奖励待领：%w", p.Run, pending))
		}
	}
	return role, errors.Join(failures...)
}
