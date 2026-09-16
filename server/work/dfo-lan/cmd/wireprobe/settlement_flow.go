package main

import (
	"context"
	"crypto/rand"
	"dfolan/internal/character"
	"dfolan/internal/game/protocol"
	"encoding/binary"
	"fmt"
	"time"
)

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
		rewards.Cards[0] = []protocol.CardReward{{Template: 0, Amount: w.cardPlan.Gold}}
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
	if w.quests != nil {
		available, e := w.availableQuestPayload(ctx)
		if e != nil {
			return nil, e
		}
		plan = append(plan, outboundPacket{"clear_available_quests", 0, 21, available})
	}
	return plan, nil
}
