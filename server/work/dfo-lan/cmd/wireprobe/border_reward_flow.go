package main

import (
	"context"
	"dfolan/internal/database"
	"dfolan/internal/game/protocol"
	"dfolan/internal/loot"
	"fmt"
	"time"
)

// Shared creation preserves the reward/omen configuration for loading and death.
func (w *worldSession) ensureDropSession() {
	if w.drops != nil && w.drops.Run == w.activeDungeon.RunID {
		return
	}
	// Drops span every job's gear at every level by design; that
	// breadth is a feature, not a bug, so the pool is not narrowed
	// to what this character can wear.
	dropCatalog := w.loot.Catalog
	if len(w.loot.DropCatalog.Items) > 0 {
		dropCatalog = w.loot.DropCatalog
	}
	w.drops = loot.NewSession(dropCatalog, w.loot.Tables, w.loot.Rules, w.loot.Equipment, w.activeDungeon.RunID, w.account, w.role.ID, w.role.WireID)
	if w.activeDungeon.Definition.Odyssey {
		w.drops.Currency = w.loot.Currency
		w.drops.ChapterDrop = w.loot.ChapterDrop
	}
	w.drops.Attunement = w.loot.Attunement
	w.drops.RewardBoxes = w.loot.RewardBoxes
	w.drops.Omen = w.loot.Omen
	// 天平档位：本场进本时由 oathInfoPackets 算好（见 oath_info.go 的
	// oathTierRun）。它与征兆是两条平行线，各自发放互不抑制。
	w.drops.OathTier = w.oathTierRun
	if w.loot.Omen != nil && w.omenHeldReady {
		// 本场开始时的持有数：-omen-state 时来自角色存档
		// （loadOmenRunState），否则来自 -omen-hold 诊断。账本本身是内存的，
		// 所以新的一场必须重新预载，否则会沿用上一场结算后的值。
		w.loot.Omen.Set(w.role.ID, w.omenHeldRun)
	} else if w.loot.Omen != nil && !w.omenHoldApplied && w.omenHold >= 0 {
		// 诊断入口，每个会话只应用一次：放到指定阶段后就交回正常的
		// 累积/结算路径，免得每进一次副本都被拽回同一格。
		w.loot.Omen.Set(w.role.ID, uint32(w.omenHold))
		w.omenHoldApplied = true
	}
	store := w.store
	if store == nil && w.characters != nil {
		store = w.store
	}
	if store != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		if hasGrowth, _ := store.HasActivePremium(ctx, w.account, database.PremiumGrowth, time.Now()); hasGrowth {
			w.drops.QuestDropBonusPercent = 20
		}
		cancel()
	}
}

// Send the frozen award grade before NOTI30 releases the client's ACT scripts.
func (w *worldSession) borderRewardPackets() ([]outboundPacket, error) {
	d := w.activeDungeon
	if !loot.IsBorderDungeon(d.Definition.ID) {
		return nil, nil
	}
	if w.loot == nil || !w.loot.Attunement.Enabled() {
		return nil, fmt.Errorf("Border reward catalog unavailable")
	}
	w.ensureDropSession()
	seed, err := randomSeed()
	if err != nil {
		return nil, err
	}
	grade, err := w.drops.PrepareBorderRewards(d, seed)
	if err != nil {
		return nil, err
	}
	payload, err := protocol.BorderRewardInfo(grade)
	if err != nil {
		return nil, err
	}
	return []outboundPacket{{"border_frozen_reward_grade", 0, 2756, payload}}, nil
}

func insertBorderBeforeLoaded(plan, border []outboundPacket) []outboundPacket {
	if len(border) == 0 {
		return plan
	}
	for i, p := range plan {
		if p.ID == 30 {
			out := append([]outboundPacket(nil), plan[:i]...)
			out = append(out, border...)
			return append(out, plan[i:]...)
		}
	}
	return append(plan, border...)
}
