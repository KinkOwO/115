package main

import (
	"context"
	"dfolan/internal/game/protocol"
	"dfolan/internal/inventory"
	"dfolan/internal/workflow"
	"fmt"
	"time"
)

func (w *worldSession) bakalRewardService() *workflow.BakalRewardService {
	if w.store == nil || w.loot == nil {
		return nil
	}
	return &workflow.BakalRewardService{Store: w.store, Awarder: &inventory.Awarder{Catalog: w.loot.Catalog, Rules: w.loot.BagRules, Equipment: w.loot.Equipment}}
}

func (w *worldSession) bakalSettlementPlan(now time.Time) ([]outboundPacket, error) {
	run, clears, ready := w.bakalOpening.SettlementIdentity()
	if !ready || run == w.bakalSettledRun || now.Before(w.bakalRewardRetryAt) {
		return nil, nil
	}
	w.bakalRewardRetryAt = now.Add(5 * time.Second)
	service := w.bakalRewardService()
	if service == nil {
		return nil, fmt.Errorf("Bakal clear requires durable source rewards")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	saved, p, err := service.Freeze(ctx, w.role, w.bakalRules, run, clears, now)
	if saved.ID != 0 {
		w.role = saved
	}
	if err != nil {
		return nil, err
	}
	if p.Eligible {
		saved, _, err = service.Claim(ctx, w.role, run)
		if saved.ID != 0 {
			w.role = saved
		}
		if err != nil {
			return nil, err
		}
	}
	body, err := w.loot.Bootstrap(workflow.LootRole(w.role))
	if err != nil {
		return nil, err
	}
	w.bakalSettledRun = run
	elapsed := w.bakalRules.TimeLimit - w.bakalOpening.Remaining(now)
	return []outboundPacket{{"bakal_source_reward_inventory", 0, 13, body}, {"bakal_clear_result", 0, 588, protocol.BakalClearResult115(elapsed)}}, nil
}
