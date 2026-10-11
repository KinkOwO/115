package main

import (
	"dfolan/internal/database"
	"dfolan/internal/game/protocol"
	"dfolan/internal/legion"
	"fmt"
)

func (w *worldSession) dungeonPotionLimit(dungeon uint32) (uint32, error) {
	if w.dungeons == nil || w.dungeons.StackableLimits == nil {
		return 0, fmt.Errorf("dungeon potion source unavailable")
	}
	limit, ok := w.dungeons.StackableLimits.Dungeons[dungeon]
	if !ok {
		return 0, fmt.Errorf("dungeon absent from potion source")
	}
	return limit, nil
}

func (w *worldSession) forestLimitedPotion(r protocol.UseStackableRequest) bool {
	return w.forest != nil && w.activeDungeon != nil && w.activeDungeon.Loaded && !w.completionSent && r.List == 0 && w.dungeons != nil && w.dungeons.StackableLimits != nil && w.dungeons.StackableLimits.Potions[r.Template]
}

func (w *worldSession) checkContentPotionUse(current database.Character, r protocol.UseStackableRequest) error {
	if err := w.checkIspinsPotionUse(current, r); err != nil {
		return err
	}
	if w.forestLimitedPotion(r) {
		stage := w.forest.stage
		if stage < 0 || stage >= len(w.forest.potionUsed) {
			return fmt.Errorf("forest potion phase invalid")
		}
		limit, err := w.forestPotionLimit(w.activeDungeon.Definition.ID)
		if err != nil {
			return err
		}
		if limit > 0 && uint32(w.forest.potionUsed[stage]) >= limit {
			return fmt.Errorf("forest source potion limit reached (%d)", limit)
		}
	}
	if w.venusLimitedPotion(r) {
		stage, err := legion.VenusStageOfDungeon(w.activeDungeon.Definition.ID)
		if err != nil || stage >= len(w.venus.potionUsed) {
			return fmt.Errorf("venus potion phase invalid")
		}
		limit, err := w.dungeonPotionLimit(w.activeDungeon.Definition.ID)
		if err != nil {
			return err
		}
		if limit > 0 && uint32(w.venus.potionUsed[stage]) >= limit {
			return fmt.Errorf("venus source potion limit reached (%d)", limit)
		}
	}
	return nil
}

func (w *worldSession) forestPotionLimit(dungeon uint32) (uint32, error) {
	return w.dungeonPotionLimit(dungeon)
}
func (w *worldSession) venusLimitedPotion(r protocol.UseStackableRequest) bool {
	return w.venus != nil && w.activeDungeon != nil && w.activeDungeon.Loaded &&
		!w.completionSent && legion.IsVenusStageDungeon(w.activeDungeon.Definition.ID) &&
		r.List == 0 && w.dungeons != nil && w.dungeons.StackableLimits != nil &&
		w.dungeons.StackableLimits.Potions[r.Template]
}
