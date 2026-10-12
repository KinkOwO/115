package main

import (
	"context"
	"dfolan/internal/catalog"
	"dfolan/internal/inventory"
	"dfolan/internal/workflow"
	"fmt"
	"time"
)

func (w *worldSession) freezeIspinsRewards(now time.Time) (*workflow.LegionRewardService, workflow.LegionRewardPlan, error) {
	var empty workflow.LegionRewardPlan
	// Unbound diagnostic sessions have no content or award authority.
	if w.activeDungeon.Definition.ID == 0 {
		return nil, empty, nil
	}
	if w.store == nil || w.loot == nil || w.dungeons == nil || w.ispins == nil {
		return nil, empty, fmt.Errorf("Ispins settlement requires durable source rewards")
	}
	r, err := catalog.IspinsPhaseRewards(w.dungeons.LegionContents, w.dungeons.IspinsOperations, w.activeDungeon.Definition.ID, w.ispins.operationIndex)
	if err != nil {
		return nil, empty, err
	}
	limited, err := ispinsWeeklyLimited()
	if err != nil {
		return nil, empty, err
	}
	s := &workflow.LegionRewardService{Store: w.store, Awarder: &inventory.Awarder{Catalog: w.loot.Catalog, Rules: w.loot.BagRules, Equipment: w.loot.Equipment}, Boxes: w.loot.RewardBoxes, Unlimited: !limited}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	saved, plan, err := s.Freeze(ctx, w.role, r, w.activeDungeon.RunID, now)
	if saved.ID != 0 {
		w.role = saved
	}
	return s, plan, err
}
func (w *worldSession) claimIspinsRewards(s *workflow.LegionRewardService, p workflow.LegionRewardPlan, packets []outboundPacket) error {
	if s == nil || !p.Eligible {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	saved, _, err := s.Claim(ctx, w.role, p.Run)
	if saved.ID != 0 {
		w.role = saved
	}
	if err != nil {
		return err
	}
	// Rebuild from the committed owner; never inject captured inventory slots.
	for i := range packets {
		switch packets[i].Name {
		case "ispins_saved_inventory_refreshed":
			packets[i].Payload, err = w.loot.Bootstrap(workflow.LootRole(w.role))
		case "ispins_settlement_character_info":
			packets[i].Payload, err = w.characters.EntryBasicProbe(w.role, w.characters.ChannelContext)
		case "ispins_settlement_character_detail":
			packets[i].Payload, err = w.characters.EntryAddition(w.role)
		}
		if err != nil {
			return err
		}
	}
	return nil
}
