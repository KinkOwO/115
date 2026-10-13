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
	// 活动 1093：能量掉落表随会话带下去（nil = 未装配，那一段整块跳过）。
	w.drops.ExtraDrop = w.loot.ExtraDrop
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
		w.loot.Omen.SetState(w.role.ID, w.omenHeldRun, w.omenMissesRun)
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

// rewardBoostTingPacketID 是 noti 2755 REWARD_BOOST_DUNGEON_TING_INFO。
// 注册点 `0x1453150D8`（注册器 `sub_1459A3DD0`，与 2756/2859 用的 `sub_14599D5D0` 不是同一个）
// → handler `sub_145310880`，载荷定长 4 字节；见 protocol.RewardBoostTingInfo。
const rewardBoostTingPacketID = 2755

// Send the frozen award grade before NOTI30 releases the client's ACT scripts.
func (w *worldSession) borderRewardPackets() ([]outboundPacket, error) {
	d := w.activeDungeon
	if !loot.IsBorderDungeon(d.Definition.ID) {
		return nil, nil
	}
	if w.loot == nil || !w.loot.Attunement.Enabled() {
		return nil, fmt.Errorf("Border reward catalog unavailable")
	}
	// noti 2755 与档位无关，只认「这一场是哪一座 reward boost 副本」，所以放在
	// 档位换算之前组好：下面「没有可演出档位」那条早退**不能**把它一起吞掉。
	ting := outboundPacket{
		"reward_boost_ting", 0, rewardBoostTingPacketID, protocol.RewardBoostTingInfo(d.Definition.ID),
	}
	log.Printf("reward boost ting (noti %d): dungeon=%d payload=%s",
		rewardBoostTingPacketID, d.Definition.ID, hex.EncodeToString(ting.Payload))
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
		return []outboundPacket{ting}, nil
	}
	payload, err := protocol.BorderRewardInfo(slot, grade)
	if err != nil {
		return nil, err
	}
	// 记下本场已经下发的档位：进本之后加入本场的那一件（命运的凝械）要比它高时，
	// 得有人知道该把它抬上去（见 raiseBorderSphereGrade）。
	w.borderSphereGrade = grade
	log.Printf("border reward (noti 2756): dungeon=%d maze=%d grade=%s(%d) -> %s(%d); payload %s",
		d.Definition.ID, d.Maze.Index, loot.TierForGradeValue(grade), grade,
		loot.AnimationSlot(slot), slot, hex.EncodeToString(payload))
	return append([]outboundPacket{ting},
		outboundPacket{"border_frozen_reward_grade", 0, 2756, payload}), nil
}

// raiseBorderSphereGrade 在「本场加入了一件更高档的装备」时重发一帧 2756，把
// **掉落演出格**与**本场最高奖励品质**一起抬上去。
//
// 口径（业主 2026-10-12）：本场有太初级物品 ⇒ 球色就到太初；对照组是 maze 0 那两场
// （史诗奖单 ⇒ 史诗球 + 史诗装备）。而进本那一帧只看得见当时掷出的奖单：
// 「命运的凝械」那件是玩家进本**之后**才选的，于是太初的货配着史诗的那一档。
//
// ⚠️ 这一帧里两个值各有去处，别混（详见 protocol.BorderRewardInfo 的取证）：
// word0 是**天上那四格掉落演出**（`*Drop_00.ani`，next176 已闭环），
// word1 才是 `getHellDungeonItemMaxRarity` —— 副本里那颗**球**的阶段分支读的是它
// （早期源码注释：44 史诗「球结束即掉落」/ 45 太初「诺克斯之后再回到球阶段」）。
// 2026-10-12 实机证明 2838（珠子/天平档位）不是这颗球的写手：(45,45) 提前到 N29
// 之前下发，球仍是金色。
//
// **只抬不降**：降回去既不报新信息，也会与奖单矛盾。换算与「演出表没有这一格就不发」
// 见 loot.AnimationGrade。
func (w *worldSession) raiseBorderSphereGrade(grade uint32) []outboundPacket {
	if grade <= w.borderSphereGrade {
		return nil
	}
	slot, ok := loot.AnimationGrade(grade)
	if !ok {
		return nil
	}
	payload, err := protocol.BorderRewardInfo(slot, grade)
	if err != nil {
		// 四格换算与载荷值域是同一套判据，走到这里说明改动过其中之一；
		// 球色没抬上去不影响那件照样掉，说话即可，不能吞。
		log.Printf("border sphere grade (noti 2756): %s(%d) 没能翻成演出格: %v",
			loot.TierForGradeValue(grade), grade, err)
		return nil
	}
	w.borderSphereGrade = grade
	log.Printf("border sphere grade (noti 2756): 本场加入 %s(%d) -> %s(%d); payload %s",
		loot.TierForGradeValue(grade), grade, loot.AnimationSlot(slot), slot, hex.EncodeToString(payload))
	return []outboundPacket{{"border_sphere_grade", 0, 2756, payload}}
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
