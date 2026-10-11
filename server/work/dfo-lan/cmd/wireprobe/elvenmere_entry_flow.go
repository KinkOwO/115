package main

import (
	"context"
	"dfolan/internal/dungeon"
	"dfolan/internal/game/protocol"
	"dfolan/internal/workflow"
	"fmt"
)

func (w *worldSession) elvenmereSelection(ctx context.Context, r protocol.DungeonSelection) (protocol.DungeonSelection, dungeon.ElvenmereProgress, error) {
	var progress dungeon.ElvenmereProgress
	if w.loot == nil || w.store == nil || w.dungeons == nil || w.dungeons.Elvenmere == nil {
		return r, progress, fmt.Errorf("elvenmere owned source/storage unavailable")
	}
	rules := w.dungeons.Elvenmere
	floor := r.Extra
	if floor == 0 {
		floor = 1
	}
	if floor > uint16(rules.MaxFloor) {
		return r, progress, fmt.Errorf("elvenmere selection is out of source range")
	}
	area, err := rules.AreaForFloor(byte(floor))
	if err != nil {
		return r, progress, err
	}
	saved, err := (&workflow.LootService{Store: w.store, Loot: w.loot}).RestoreElvenmereProgress(ctx, w.role)
	if err != nil {
		return r, progress, err
	}
	progress, _, err = dungeon.ReadElvenmereProgress(saved.State)
	if err != nil {
		return r, progress, err
	}
	if progress.MaxCleared >= rules.MaxFloor {
		return r, progress, fmt.Errorf("elvenmere is fully cleared for this account/week")
	}
	if progress.MaxCleared == 0 {
		if floor != uint16(area.First) {
			return r, progress, fmt.Errorf("elvenmere first entry must use a source zone start")
		}
		r.Extra = uint16(area.First)
	} else {
		start, err := rules.AreaForFloor(progress.StartFloor)
		if err != nil {
			return r, progress, err
		}
		next := progress.MaxCleared + 1
		if floor != uint16(start.First) && floor != uint16(next) {
			return r, progress, fmt.Errorf("elvenmere zone is bound by this account's first clear")
		}
		r.Extra = uint16(next)
	}
	w.role = saved
	return r, progress, nil
}
