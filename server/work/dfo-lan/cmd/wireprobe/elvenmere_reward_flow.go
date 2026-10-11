package main

import (
	"context"
	"dfolan/internal/character"
	"dfolan/internal/dungeon"
	"dfolan/internal/game/protocol"
	"dfolan/internal/inventory"
	"dfolan/internal/workflow"
	"fmt"
)

// Resolve rewards from the current PVF; preserve native packet layouts.
// Cache state changes happen after the durable, idempotent floor transaction.
func (w *worldSession) commitElvenmereRewards(ctx context.Context, floor byte) ([]outboundPacket, error) {
	if w.loot == nil && w.progression == nil {
		return nil, nil
	}
	if w.store == nil || w.loot == nil || !w.activeDungeon.Completed() {
		return nil, fmt.Errorf("elvenmere floor requires a completed owned run and reward storage")
	}
	if w.dungeons == nil {
		return nil, fmt.Errorf("elvenmere source catalog unavailable")
	}
	rule, err := dungeon.ElvenmereFloorReward(w.dungeons.Elvenmere, floor)
	if err != nil {
		return nil, err
	}
	g := workflow.ElvenmereFloorGrant{Run: w.activeDungeon.RunID, Floor: floor}
	g.StartFloor = w.activeDungeon.ElvenmereStartFloor
	g.ExpectedWeekStart = w.activeDungeon.ElvenmereWeekStart
	g.WeeklyItem, g.WeeklyCount = rule.Weekly.Template, rule.Weekly.Count
	g.SeasonItem, g.SeasonCount = rule.Season.Template, rule.Season.Count
	if w.progression != nil {
		g.Experience = rule.Experience
	}
	before, err := inventory.ReadBag(w.role.State)
	if err != nil {
		return nil, err
	}
	saved, receipt, err := (&workflow.LootService{Store: w.store, Loot: w.loot}).CommitElvenmereFloor(ctx, w.role, g, w.progression)
	if err != nil {
		return nil, err
	}
	after, err := inventory.ReadBag(saved.State)
	if err != nil {
		return nil, err
	}
	var plan []outboundPacket
	if rows := inventory.ChangedItemRows(before, after); len(rows) > 0 {
		body, e := protocol.InventoryUpdate(rows)
		if e != nil {
			return nil, e
		}
		plan = append(plan, outboundPacket{"elvenmere_inventory_updated", 0, 14, body})
	}
	newLevel := w.level
	if receipt.Grant.Experience > 0 {
		body, e := character.ExperiencePayload(saved)
		if e != nil {
			return nil, e
		}
		newLevel = body[0]
		plan = append(plan, outboundPacket{"elvenmere_experience_updated", 0, 37, body})
	}
	progress, _, e := dungeon.ReadElvenmereProgress(saved.State)
	if e != nil {
		return nil, e
	}
	previousLevel := w.level
	w.role, w.level = saved, newLevel
	w.activeDungeon.WeeklyRewards, w.activeDungeon.SeasonRewards = progress.RewardFlags()
	if newLevel != previousLevel {
		if skills, e := w.automaticSkillRefresh(); e == nil {
			plan = append(plan, skills...)
		}
	}
	return plan, nil
}
