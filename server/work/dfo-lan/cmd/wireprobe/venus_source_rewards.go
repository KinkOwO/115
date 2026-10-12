package main

import (
	"context"
	"dfolan/internal/catalog"
	"dfolan/internal/inventory"
	"dfolan/internal/legion"
	"dfolan/internal/loot"
	"dfolan/internal/workflow"
	"fmt"
	"time"
)

func (w *worldSession) venusSourceRewardPayloads(stage int) ([]byte, []byte, error) {
	if w.store == nil || w.loot == nil || w.loot.RewardBoxes == nil || w.dungeons == nil || w.venus == nil || w.activeDungeon == nil || (!w.activeDungeon.Completed() && !w.venus.downshift) {
		return nil, nil, fmt.Errorf("Venus settlement requires durable source rewards and an owned completed stage")
	}
	choice := w.venus.choice
	physicalStage := stage
	if w.venus.downshift {
		choice, physicalStage = 1, 3
	}
	if stage != legion.VenusEndpoint(choice) || w.activeDungeon.Definition.ID != legion.VenusStageDungeons[physicalStage] || w.activeDungeon.StartedAt.IsZero() {
		return nil, nil, fmt.Errorf("Venus terminal source identity or clock missing")
	}
	elapsed := w.venus.clearElapsed[stage]
	if elapsed <= 0 {
		if w.venus.downshift {
			return nil, nil, fmt.Errorf("Venus expert clear clock missing for phase-four stop")
		}
		elapsed = time.Since(w.activeDungeon.StartedAt)
		if elapsed <= 0 {
			return nil, nil, fmt.Errorf("Venus clear clock is in the future")
		}
		w.venus.clearElapsed[stage] = elapsed
	}
	c := w.dungeons.LegionContents
	if c == nil || c.Contents["Venus Legion"].NativeOperations == nil {
		return nil, nil, fmt.Errorf("Venus source operation missing")
	}
	op, ok := c.Contents["Venus Legion"].NativeOperations.Definitions[uint16(choice)+1]
	if !ok || len(op.Rewards) == 0 {
		return nil, nil, fmt.Errorf("Venus selected source operation missing")
	}
	difficulty := op.Rewards[0].Difficulty
	r, err := catalog.NativePhaseRewards(c, "Venus Legion", legion.VenusStageDungeons[stage], uint16(choice)+1, difficulty, difficulty)
	if err != nil {
		return nil, nil, err
	}
	roots := map[uint32]bool{}
	for _, row := range append(append([]catalog.LegionOperationReward(nil), r.Basic...), r.Additional...) {
		roots[row.Template] = true
	}
	boxes := &apocalypseBundleSource{RewardBoxSource: w.loot.RewardBoxes, catalog: w.loot.Catalog, roots: roots}
	// Preserve the upstream repeat-run policy; quantities and lotteries are source-owned.
	svc := &workflow.LegionRewardService{Store: w.store, Awarder: &inventory.Awarder{Catalog: w.loot.Catalog, Rules: w.loot.BagRules, Equipment: w.loot.Equipment}, Boxes: boxes, Unlimited: true}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	saved, frozen, err := svc.Freeze(ctx, w.role, r, "venus:"+w.activeDungeon.RunID, time.Now())
	if saved.ID != 0 {
		w.role = saved
	}
	if err != nil {
		return nil, nil, err
	}
	project := func(items []loot.Award) []legion.IspinsRewardItem {
		out := make([]legion.IspinsRewardItem, 0, len(items))
		for _, a := range items {
			out = append(out, legion.IspinsRewardItem{Template: a.Template, Amount: a.Amount})
		}
		return out
	}
	basic, additional, err := legion.IspinsSourceRewardPayloads(project(frozen.Basic), project(frozen.Additional), elapsed, frozen.Eligible)
	if err != nil {
		return nil, nil, err
	}
	saved, _, err = svc.Claim(ctx, w.role, frozen.Run)
	if saved.ID != 0 {
		w.role = saved
	}
	if err != nil {
		return nil, nil, err
	}
	return basic, additional, nil
}
