package main

import (
	"context"
	"crypto/rand"
	"dfolan/internal/character"
	"dfolan/internal/game/protocol"
	"dfolan/internal/inventory"
	"encoding/binary"
	"fmt"
	"time"
)

// canRechallenge reports whether another run of the settled dungeon is
// allowed - the condition behind NOTI 261 value 9. It mirrors the gate
// restartDungeon applies so the lit button is never a dead end: a run with
// no fatigue cost (or an exhausted fatigue pool) is answered before the
// player presses anything.
func (w *worldSession) canRechallenge(ctx context.Context) bool {
	if w == nil || w.activeDungeon == nil || !w.activeDungeon.Completed() {
		return false
	}
	d := w.activeDungeon
	if d.Definition.ID == blackPurgatorySquadDungeon {
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
	role, receipt, _, e := w.progression.Clear(ctx, w.role, w.activeDungeon, r.RankPoint, time.Now())
	if e != nil {
		return nil, e
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
	if w.loot != nil && w.loot.CardPolicy != nil {
		if w.cardPlan == nil {
			var seed uint32
			if e = binary.Read(rand.Reader, binary.LittleEndian, &seed); e != nil {
				return nil, e
			}
			p, err := w.loot.FreezeCards(ctx, role, w.activeDungeon, *w.loot.CardPolicy, seed)
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
	w.role, w.level = role, experience[0]
	plan := []outboundPacket{{"dungeon_play_result", 0, 34, notice}, {"dungeon_clear_experience", 0, 37, experience}, {"dungeon_clear_reward", 0, 35, reward}}
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
