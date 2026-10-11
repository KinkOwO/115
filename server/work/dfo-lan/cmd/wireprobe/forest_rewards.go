package main

import (
	"context"
	"dfolan/internal/game/protocol"
	"dfolan/internal/inventory"
	"dfolan/internal/legion"
	"dfolan/internal/loot"
	"dfolan/internal/workflow"
	"fmt"
	"time"
)

// Forest keeps the upstream operator reward policy requested for integration.
// Its material quantities are not represented as native PVF reward rules.
// Gear candidates are the current source-prepared pool, never captured items.
func (w *worldSession) settleForestRewards(stage int) ([]legion.ForestRewardItem, []legion.ForestRewardItem, []outboundPacket, error) {
	if w.store == nil || w.loot == nil || w.forest == nil || w.activeDungeon == nil {
		return nil, nil, nil, fmt.Errorf("forest settlement requires durable source rewards")
	}
	service := &workflow.LegionRewardService{Store: w.store, Awarder: &inventory.Awarder{Catalog: w.loot.Catalog, Rules: w.loot.BagRules, Equipment: w.loot.Equipment}}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	content := "ForestOfAwakeningNormal"
	if w.forest.hard {
		content = "ForestOfAwakeningHard"
	}
	saved, frozen, err := service.FreezePrepared(ctx, w.role, "forest:"+w.activeDungeon.RunID, content, w.activeDungeon.Definition.ID, func() ([]loot.Award, []loot.Award, error) {
		var basic, additional []legion.ForestRewardItem
		if w.forest.hard {
			basic, additional = legion.ForestHardFinalBasic(), legion.ForestHardFinalAdditional()
		} else if stage == len(legion.ForestStageDungeons)-1 {
			basic, additional = legion.ForestNormalFinalBasic(), legion.ForestNormalFinalAdditional()
		} else {
			additional = legion.ForestNormalStageRewards()
		}
		count := 0
		for _, item := range basic {
			if item.Template == 0 {
				count++
			}
		}
		gear := legion.VenusRollFlipGear(venusFlipGearPool, count)
		if len(gear) != count {
			return nil, nil, fmt.Errorf("forest source gear pool unavailable")
		}
		var first, second []loot.Award
		n := 0
		for _, item := range basic {
			if item.Template == 0 {
				item.Template = gear[n]
				n++
			}
			first = append(first, loot.Award{Template: item.Template, Amount: item.Amount})
		}
		for _, item := range additional {
			second = append(second, loot.Award{Template: item.Template, Amount: item.Amount})
		}
		return first, second, nil
	})
	if saved.ID != 0 {
		w.role = saved
	}
	if err != nil {
		return nil, nil, nil, err
	}
	before, err := inventory.ReadBag(w.role.State)
	if err != nil {
		return nil, nil, nil, err
	}
	saved, _, err = service.Claim(ctx, w.role, frozen.Run)
	if saved.ID != 0 {
		w.role = saved
	}
	if err != nil {
		// The frozen pending plan survives full bags or a failed commit and
		// is retried through the existing character-entry Recover path.
		return nil, nil, nil, fmt.Errorf("forest frozen reward pending: %w", err)
	}
	toRows := func(items []loot.Award) []legion.ForestRewardItem {
		out := make([]legion.ForestRewardItem, len(items))
		for i, item := range items {
			out[i] = legion.ForestRewardItem{Template: item.Template, Amount: item.Amount}
		}
		return out
	}
	after, err := inventory.ReadBag(w.role.State)
	if err != nil {
		return nil, nil, nil, err
	}
	payload, err := protocol.InventoryUpdate(inventory.ChangedItemRows(before, after))
	if err != nil {
		return nil, nil, nil, err
	}
	return toRows(frozen.Basic), toRows(frozen.Additional), []outboundPacket{{"forest_flip_inventory_updated", 0, 14, payload}}, nil
}
