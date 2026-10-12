package main

import (
	"dfolan/internal/game/protocol"
	"dfolan/internal/legion"
	"encoding/binary"
	"fmt"
)

func (w *worldSession) ispinsPotionLimit(stage int, dungeon uint32) (uint32, error) {
	if !w.ispins.sourceCards {
		frames, err := legion.IspinsReplayFrames("stackable_dungeon_limit")
		if err != nil {
			return 0, err
		}
		return binary.LittleEndian.Uint32(frames[0].Body), nil
	}
	if w.dungeons.StackableLimits == nil {
		return 0, fmt.Errorf("Ispins potion source missing")
	}
	limit, ok := w.dungeons.StackableLimits.Dungeons[dungeon]
	if !ok {
		return 0, fmt.Errorf("Ispins dungeon missing potion source")
	}
	if def := w.dungeons.IspinsOperations[w.ispins.operationIndex]; def.Type == 13 {
		if !w.ispins.cardsOwned[stage] {
			return 0, fmt.Errorf("Ispins potion parameter not frozen")
		}
		for _, card := range w.ispins.cards[stage] {
			if card.Index == w.ispins.operationIndex {
				return uint32(card.Value), nil
			}
		}
		return 0, fmt.Errorf("Ispins selected potion card not owned")
	}
	return limit, nil
}

func (w *worldSession) ispinsLimitedPotion(r protocol.UseStackableRequest) bool {
	return w.ispins != nil && w.ispins.sourceCards && w.activeDungeon != nil && w.activeDungeon.Loaded && !w.completionSent && r.List == 0 && w.dungeons != nil && w.dungeons.StackableLimits != nil && w.dungeons.StackableLimits.Potions[r.Template]
}

func (w *worldSession) checkIspinsPotionUse() error {
	if w.ispins.potionUsed >= w.ispins.potionLimit {
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
