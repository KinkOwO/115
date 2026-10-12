package main

import (
	"context"
	"dfolan/internal/inventory"
	"dfolan/internal/legion"
	"dfolan/internal/workflow"
	"fmt"
	"time"
)

func (w *worldSession) apocalypseSourceRewards() (workflow.ApocalypseRewardPlan, error) {
	var empty workflow.ApocalypseRewardPlan
	r := w.apocalypse
	if r == nil || !r.Entered || r.ID == "" || w.activeDungeon == nil || !w.activeDungeon.Completed() || w.legion == nil || w.legion.catalog == nil || w.dungeons == nil || w.dungeons.LegionContents == nil || w.store == nil || w.loot == nil {
		return empty, fmt.Errorf("apocalypse settlement requires an owned completed run and durable source rewards")
	}
	stage := legion.ApocalypseStageOfDungeon(w.activeDungeon.Definition.ID)
	if stage != legion.ApocalypseEndpoint(r.Choice) || r.Cleared != stage {
		return empty, fmt.Errorf("apocalypse settlement does not own the terminal stage")
	}
	reward, err := legion.GuaranteedClearReward(w.legion.catalog, uint32(r.Choice)+1)
	if err != nil {
		return empty, err
	}
	contents := *w.dungeons.LegionContents
	items, err := apocalypseBasicProducts(contents, w.loot.Catalog, w.loot.RewardBoxes, reward.Label, r.ID)
	if err != nil {
		return empty, err
	}
	if r.RunStartedAt.IsZero() || time.Now().Before(r.RunStartedAt) {
		return empty, fmt.Errorf("apocalypse run clock missing")
	}
	p := workflow.ApocalypseRewardPlan{Run: r.ID, Source: w.role.ConfigVersion, Content: contents.Source.Checksum, Rules: w.legion.catalog.SHA256, Operation: uint32(r.Choice) + 1, Phase: byte(stage), ElapsedMillis: uint64(time.Since(r.RunStartedAt) / time.Millisecond), Reward: reward, Additional: items, AdditionalRules: contents.SHA256}
	s := &workflow.ApocalypseRewardService{Store: w.store, Awarder: &inventory.Awarder{Catalog: w.loot.Catalog, Rules: w.loot.BagRules, Equipment: w.loot.Equipment}}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	saved, frozen, err := s.Freeze(ctx, w.role, p)
	if saved.ID != 0 {
		w.role = saved
	}
	if err != nil {
		return empty, err
	}
	if frozen.Run != p.Run || frozen.Source != p.Source || frozen.Content != p.Content || frozen.Rules != p.Rules || frozen.Operation != p.Operation || frozen.Phase != p.Phase || frozen.Reward != p.Reward || frozen.AdditionalRules != p.AdditionalRules {
		return empty, fmt.Errorf("apocalypse frozen source identity mismatch")
	}
	saved, receipt, err := s.Claim(ctx, w.role, frozen.Run)
	if saved.ID != 0 {
		w.role = saved
	}
	if err != nil {
		return empty, err
	}
	if !receipt.Matches(frozen) {
		return empty, fmt.Errorf("apocalypse committed reward receipt mismatch")
	}
	return frozen, nil
}
