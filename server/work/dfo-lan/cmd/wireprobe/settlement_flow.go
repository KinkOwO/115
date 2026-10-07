package main

import (
	"context"
	"crypto/rand"
	"dfolan/internal/character"
	"dfolan/internal/game/protocol"
	"dfolan/internal/inventory"
	"dfolan/internal/legion"
	"dfolan/internal/workflow"
	"encoding/binary"
	"fmt"
	"log"
	"time"
)

// canRechallenge reports whether another run of the settled dungeon is
// allowed - the condition behind NOTI 261 value 9. It mirrors the gate
// restartDungeon applies so the lit button is never a dead end: a run with
// no fatigue cost (or an exhausted fatigue pool) is answered before the
// player presses anything.
func (w *worldSession) canRechallenge(ctx context.Context) bool {
	if w == nil {
		return false
	}
	d := w.activeDungeon
	if d == nil || !d.Completed() || d.Definition.ID == blackPurgatorySquadDungeon || d.Definition.Tower != nil {
		return false
	}
	if w.fatigue == nil || d.Definition.NoFatigue || w.fatigue.Rules.RoomCost <= 0 {
		return true
	}
	fp, err := w.fatigue.State(ctx, w.account, w.role.ID, time.Now())
	if err != nil {
		return false
	}
	return fp.Used < fp.Limit
}

func (w *worldSession) dungeonResult(p []byte) ([]outboundPacket, error) {
	// [ISPINS-ARENA-BOSS] 伊斯大陆会话的 CMD46 整包吞掉：官服 s4 实证
	// （c2s 帧 338/393/442/489）每阶段 boss 死亡后客户端都会发 141B 的
	// 通用结算请求，但官服对它没有任何专门应答（s2c 全流无 kind=1 id=46；
	// 结算链 N31 家族已在死亡时刻发出，也无 N34/N37/N26/N261/N19/N2758/
	// N29/N21 通用结算族）。2026-10-03 四测：generic 路径发出这 8 个包后
	// 客户端 1.2s 内 op=682 崩溃退出。
	if w != nil && w.ispins != nil && w.activeDungeon != nil {
		return nil, nil
	}
	// 维纳斯终局翻牌：CMD46 整包吞掉（仿伊斯）。军团翻牌链 N31→N2252→N2253
	// 已在 completeVenusStage 发出，通用结算（N34/N35/N261+8张牌翻牌，图5/图6）
	// 不适用——团本里不存在那种通用结算面板。
	if w != nil && w.venus != nil && w.activeDungeon != nil && legion.IsVenusStageDungeon(w.activeDungeon.Definition.ID) {
		return nil, nil
	}
	// 苏醒之森：CMD46 = 下一关作战窗触发器（官服 21:43:02.326→.340：CMD46
	// 的应答就是 N2563 state6，通用结算面板不适用）。forestResult 内部静默
	// 收尾副本会话（官服无回城包链，客户端自行完成场景切换）。连战模式下
	// 客户端在 2062 被拒后还会补发 CMD46——此时 activeDungeon 已被上一轮
	// forestResult 清掉，所以这里只按 run 存在性分流（0231 会话实证：绕过
	// 本分支会落进通用结算，弹占位符兜底面板卡死流程）。
	if w != nil && w.forest != nil {
		return w.forestResult(p)
	}
	if w == nil || w.progression == nil || w.activeDungeon == nil || !w.activeDungeon.Completed() || !w.completionSent {
		return nil, fmt.Errorf("result before committed boss completion")
	}
	r, e := protocol.DecodePlayResult(p)
	if e != nil {
		return nil, e
	}
	if r.Actor != w.role.WireID {
		return nil, fmt.Errorf("result actor mismatch")
	}
	if w.resultSent {
		return nil, nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	tower := w.activeDungeon.Definition.Tower
	if tower != nil {
		progress, _, err := w.towerProgress(ctx, tower)
		if err != nil {
			return nil, err
		}
		retry := progress.HighestCleared == tower.Floor && progress.LastRun == w.activeDungeon.RunID
		if !retry && (progress.HighestCleared+1 != tower.Floor || progress.EntriesToday == 0) {
			return nil, fmt.Errorf("%s tower clear is not the next available floor", tower.Key)
		}
	}
	now := time.Now()
	var awarder *inventory.Awarder
	if w.loot != nil {
		awarder = &inventory.Awarder{Catalog: w.loot.Catalog, Rules: w.loot.BagRules, Equipment: w.loot.Equipment}
	}
	role, receipt, _, e := w.progression.ClearWithTowerRewards(ctx, w.role, w.activeDungeon, r.RankPoint, now, awarder)
	if e != nil {
		return nil, e
	}
	if tower != nil {
		if _, err := w.store.AdvanceTowerFloor(ctx, w.account, towerPolicy(tower), tower.Floor, w.activeDungeon.RunID); err != nil {
			return nil, err
		}
	}
	role.WireID = w.role.WireID
	best := receipt.BestElapsed
	if best == 0 {
		best = receipt.Elapsed
	}
	notice, e := protocol.PlayResultNoticeRecord(role.WireID, receipt.Grade, receipt.Rank, receipt.Elapsed, best, receipt.AllClear, receipt.NewRecord)
	if e != nil {
		return nil, e
	}
	rewards := protocol.ClearRewardState{BaseExperience: receipt.Base, ScoreExperience: receipt.Score, MonsterExperience: receipt.MonsterExperience}
	if w.loot != nil && w.loot.CardPolicy != nil && !w.isBoostGuideRun() {
		if w.cardPlan == nil {
			var seed uint32
			if e = binary.Read(rand.Reader, binary.LittleEndian, &seed); e != nil {
				return nil, e
			}
			p, err := (&workflow.LootService{Store: w.store, Loot: w.loot}).FreezeCards(ctx, role, w.activeDungeon, *w.loot.CardPolicy, seed)
			if err != nil {
				return nil, err
			}
			w.cardPlan = &p
		}
		// Solo participant0: physical card choice is reported separately by71.
		if w.cardPlan.Gold > 0 {
			rewards.Cards[0] = append(rewards.Cards[0], protocol.CardReward{Template: 0, Amount: w.cardPlan.Gold})
		}
		for _, item := range w.cardPlan.Items {
			if item.Amount > 0 {
				rewards.Cards[0] = append(rewards.Cards[0], protocol.CardReward{Template: item.Template, Amount: item.Amount})
			}
		}
	}
	reward, e := protocol.ClearReward(rewards)
	if e != nil {
		return nil, e
	}
	experience, e := character.ExperiencePayload(role)
	if e != nil {
		return nil, e
	}
	var itemUpdate []byte
	if len(receipt.TowerRewards) != 0 {
		before, err := inventory.ReadBag(w.role.State)
		if err != nil {
			return nil, err
		}
		after, err := inventory.ReadBag(role.State)
		if err != nil {
			return nil, err
		}
		itemUpdate, err = protocol.InventoryUpdate(inventory.ChangedItemRows(before, after))
		if err != nil {
			return nil, err
		}
	}
	preClear := w.role
	w.role, w.level = role, experience[0]
	// Starter Boost 662：引导通关在结算里推进训练关卡（源里的 [guide] dungeon）。
	// 结算已经提交，补帧失败只记日志——不能让进度校验把玩家卡在已通关的副本里。
	var boostPlan []outboundPacket
	if w.boostup != nil && w.isBoostGuideRun() {
		p, boostErr := w.completeBoostGuide()
		if boostErr != nil {
			log.Printf("boost guide settlement progress role=%d: %v", w.role.ID, boostErr)
		} else {
			boostPlan = p
		}
	}
	plan := []outboundPacket{{"dungeon_play_result", 0, 34, notice}, {"dungeon_clear_experience", 0, 37, experience}, {"dungeon_clear_reward", 0, 35, reward}}
	plan = append(plan, boostPlan...)
	// 665 的通关计数就藏在这笔结算事务里，面板只认 2722：不补这一帧，玩家打完
	// 「秩序终结者」回到城里看到的还是旧计数（实机 2026-10-06 16:43）。
	plan = append(plan, boostChallengeClearProgress(w.boostup, preClear, w.role)...)
	if len(itemUpdate) != 0 {
		plan = append(plan, outboundPacket{"tower_inventory_reward", 0, 14, itemUpdate})
	}
	if tower != nil && tower.Key == "grief" {
		rows := make([]protocol.TowerRewardItem, 0, len(receipt.TowerRewards))
		for _, item := range receipt.TowerRewards {
			rows = append(rows, protocol.TowerRewardItem{Template: item.Template, Amount: item.Amount})
		}
		body, err := protocol.TowerGriefClearReward(tower.Floor, rows...)
		if err != nil {
			return nil, err
		}
		plan = append(plan, outboundPacket{"tower_grief_clear_reward", 0, 1255, body})
	}
	// NOTI261（ENUM_NOTIPACKET_EPLP_RECHALLENGE）必须**跟在 NOTI35 之后**：
	// 结算面板由 35 构建，261 只负责把「继续挑战」入口与右侧箭头置为可用（9）
	// 或置灰（1）。不发它时面板照常显示，但入口永远点不动、右侧也不出箭头。
	//
	// 亮 9 的条件与 restartDungeon 的准入保持一致（同一套疲劳判定），
	// 这样「按钮亮着」就等价于「按下去能成」，不会给玩家一个点了会失败的入口。
	rechallenge := protocol.EplpRechallengeBlocked
	if w.canRechallenge(ctx) {
		rechallenge = protocol.EplpRechallengeReady
	}
	plan = append(plan, outboundPacket{"eplp_rechallenge", 0, 261, protocol.EplpRechallenge(rechallenge)})
	if receipt.CreatureExperienceGained > 0 {
		creatures, err := inventory.CreatureListPayload(role.State)
		if err != nil {
			return nil, err
		}
		growth, err := inventory.CreatureGrowthPayload(role.State)
		if err != nil {
			return nil, err
		}
		plan = append(plan, outboundPacket{"dungeon_clear_creature_list", 0, 105, creatures}, outboundPacket{"dungeon_clear_creature_growth", 0, 102, growth})
	}
	// 结算会重刷整个技能窗口，所以 id29（VP 帧）必须跟着 id19 一起补发。
	// 少发 id29 时客户端把 Enhance/Evolve/VP 渲染成已清空（case 2347 的注释
	// 里已经踩过一次），玩家看到的就是「VP 点和学过的 VP 技能全没了」——
	// 存档其实是完好的：2026-09-23 实机查库确认 FenF（id 9）的
	// skill_variations.options 仍带着 5 个已选项，technique_points 为 0 只是
	// 因为 5 点已经全部分配出去（ReconcileTechniquePoints 的 5 − 已选）。
	if w.characters != nil {
		if restore, e := w.characters.EntrySkills(role); e == nil {
			plan = append(plan, outboundPacket{"skill_state_restored", 0, 19, restore})
			plan, e = appendSkillPresetRestore(plan, w.characters, role, "skill_preset_restored_after_settlement")
			if e != nil {
				return nil, e
			}
		}
		if variation, e := w.characters.VariationRestore(role); e == nil && len(variation) > 0 {
			plan = append(plan, outboundPacket{"skill_variation_response", 1, 29, variation})
		}
	}
	if w.quests != nil {
		available, e := w.availableQuestPayload(ctx)
		if e != nil {
			return nil, e
		}
		plan = append(plan, outboundPacket{"clear_available_quests", 0, 21, available})
	}
	return plan, nil
}
