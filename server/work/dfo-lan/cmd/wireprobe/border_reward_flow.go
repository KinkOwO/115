package main

import (
	"context"
	"dfolan/internal/catalog"
	"dfolan/internal/database"
	"dfolan/internal/game/protocol"
	"dfolan/internal/loot"
	"encoding/hex"
	"fmt"
	"log"
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
	// 调律之边界：改用「只拆 [instantly open]」的包装源（源标记判据）。
	// 其它玩法（含小深渊/终末之边界）**保持旧行为** —— 这条规则先在一个玩法上验证，
	// 见 next176 §19.3。
	if w.loot.InstantlyOpenBoxes != nil &&
		catalog.DungeonType(w.activeDungeon.Definition) == attunementPlayType {
		w.drops.RewardBoxes = w.loot.InstantlyOpenBoxes
	}
	w.drops.Omen = w.loot.Omen
	// 两条线的档位：本场进本时由 oathInfoPackets 预掷并随 2838 下发（见 oath_info.go）。
	// 掉落按同一档选池，保证颜色与奖励一致。
	w.drops.Tiers = w.attunementRunTiers
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
	// 本仓的档位阶梯是 40..45 = normal..primeval，客户端**四格掉落演出表**比它早两格
	// （换算与实测依据见 loot.AnimationGrade）。没有对应格就不发这一帧：拿最近一格顶上
	// 会让演出说谎，而**不发**同样不对 —— 客户端此时会保留模块构造初值 -1，那也是越界
	// （这才是「每次必定播太初」的旧现象）。所以下面把「没有可演出档位」与「本场根本没
	// 有装备奖励」一并当作不发，两者都只发生在池子给不出东西的时候。
	slot, ok := loot.AnimationGrade(grade)
	if !ok {
		return nil, nil
	}
	payload, err := protocol.BorderRewardInfo(slot)
	if err != nil {
		return nil, err
	}
	log.Printf("border reward (noti 2756): grade=%s(%d) -> %s(%d); payload %s",
		loot.TierForGradeValue(grade), grade, loot.AnimationSlot(slot), slot, hex.EncodeToString(payload))
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
