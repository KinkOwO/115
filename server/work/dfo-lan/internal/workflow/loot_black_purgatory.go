package workflow

import (
	"context"
	"dfolan/internal/dungeon"
	"dfolan/internal/loot"
	"dfolan/internal/storage"
	"encoding/json"
	"errors"
	"fmt"
)

func (s *LootService) FreezeBlackPurgatoryCards(ctx context.Context, role storage.Character, d *dungeon.Session, seed uint32) (loot.CardPlan, error) {
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
func (s *LootService) pickBlackPurgatoryBoss(ctx context.Context, role storage.Character, run string, index byte, expected loot.Award) (storage.Character, loot.BlackPurgatoryBossReceipt, bool, error) {
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
		func(current storage.Character) (json.RawMessage, json.RawMessage, error) {
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
func (s *LootService) RecoverBlackPurgatoryCards(ctx context.Context, role storage.Character) (storage.Character, error) {
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
				func(current storage.Character) (json.RawMessage, json.RawMessage, error) {
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
