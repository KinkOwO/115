package workflow

import (
	"context"
	"dfolan/internal/dungeon"
	"dfolan/internal/loot"
	"dfolan/internal/storage"
	"encoding/json"
	"fmt"
)

// LootService commits and replays state prepared by the loot domain.
type LootService struct {
	Store *storage.Store
	Loot  *loot.Service
}

func (s *LootService) FreezeMoonReward(ctx context.Context, role storage.Character, run *dungeon.Session, p loot.MoonRewardPolicy) (loot.MoonRewardPlan, error) {
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
	_, _, e = s.Store.CommitCharacterEvent(ctx, role.AccountID, role.ID, p.Source, "moon-clear:"+run.RunID, loot.MoonRewardModel, func(current storage.Character) (json.RawMessage, json.RawMessage, error) {
		return current.State, body, nil
	})
	if e != nil {
		return out, e
	}
	return s.ReadMoonReward(ctx, role, run.RunID)
}
func (s *LootService) ReadMoonReward(ctx context.Context, role storage.Character, run string) (loot.MoonRewardPlan, error) {
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
func (s *LootService) ClaimMoonReward(ctx context.Context, role storage.Character, run string) (storage.Character, bool, error) {
	p, e := s.ReadMoonReward(ctx, role, run)
	if e != nil {
		return role, false, e
	}
	saved, fresh, e := s.Store.CommitCharacterEvent(ctx, role.AccountID, role.ID, p.Source, "moon-grant:"+run, loot.MoonRewardModel, func(current storage.Character) (json.RawMessage, json.RawMessage, error) {
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
func (s *LootService) RecoverMoonRewards(ctx context.Context, role storage.Character) (storage.Character, error) {
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
