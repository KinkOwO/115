package main

import (
	"dfolan/internal/database"
	"dfolan/internal/game/protocol"
	"fmt"
)

func (w *worldSession) ispinsPotionLimit(dungeon uint32) (uint32, error) {
	r := w.ispins
	if w.dungeons.StackableLimits == nil {
		if r.orderOwned {
			return 0, fmt.Errorf("owned Ispins potion source missing")
		}
		return 0, nil
	}
	limit, ok := w.dungeons.StackableLimits.Dungeons[dungeon]
	if !ok {
		return 0, fmt.Errorf("Ispins dungeon absent from potion limit source")
	}
	if def, ok := w.dungeons.IspinsOperations[r.operationIndex]; ok && def.Type == 13 {
		if !r.cardsOwned[r.stage] {
			return 0, fmt.Errorf("Ispins potion operation parameter not frozen")
		}
		found := false
		for _, card := range r.cards[r.stage] {
			if card.Index == r.operationIndex {
				limit = uint32(card.Value)
				found = true
				break
			}
		}
		if !found || limit == 0 {
			return 0, fmt.Errorf("Ispins potion operation has invalid owned parameter")
		}
	}
	return limit, nil
}

func (w *worldSession) ispinsLimitedPotion(r protocol.UseStackableRequest) bool {
	return w.ispins != nil && w.activeDungeon != nil && w.activeDungeon.Loaded && !w.completionSent && r.List == 0 && w.dungeons != nil && w.dungeons.StackableLimits != nil && w.dungeons.StackableLimits.Potions[r.Template]
}

func (w *worldSession) checkIspinsPotionUse(_ database.Character, r protocol.UseStackableRequest) error {
	if w.ispinsLimitedPotion(r) && w.ispins.potionLimit > 0 && w.ispins.potionUsed >= w.ispins.potionLimit {
		return fmt.Errorf("Ispins source potion limit reached (%d)", w.ispins.potionLimit)
	}
	return nil
}

func (w *worldSession) checkIspinsCoinUse() error {
	if w == nil || w.ispins == nil || w.activeDungeon == nil || w.dungeons == nil {
		return nil
	}
	def, ok := w.dungeons.IspinsOperations[w.ispins.operationIndex]
	if ok && def.AllowCoin != nil && *def.AllowCoin >= 0 && w.ispins.coinsUsed >= uint32(*def.AllowCoin) {
		return fmt.Errorf("Ispins source coin limit reached (%d)", *def.AllowCoin)
	}
	return nil
}
