package main

import (
	"context"
	"dfolan/internal/boostup"
	"dfolan/internal/game/protocol"
	"dfolan/internal/loot"
	"dfolan/internal/workflow"
	"encoding/binary"
	"fmt"
)

// boostChallengeRequest 处理 event 665（毕业后的可选挑战）：680 领奖、681 查询。
//
// 领奖幂等键取本树唯一口径（连接随机数 + 传输帧摘要，见 requestKeySession）；
// 没有传输帧（测试/内部重放）时不能用正文当帧，那会让同一正文的两次不同请求
// 撞同一个键。
func (w *worldSession) boostChallengeRequest(ctx context.Context, p []byte, id uint16, raw ...[]byte) ([]outboundPacket, error) {
	refuse := func(e error) ([]outboundPacket, error) {
		return []outboundPacket{{"boost_challenge_refused", 1, id, protocol.EventRefusal115(boostup.ChallengeEventID, 102, id == 681)}}, e
	}
	if w == nil || w.boostup == nil || len(w.boostup.Challenges) == 0 || w.loot == nil || w.characters == nil || w.role.ID == 0 || w.role.AccountID != w.account {
		return refuse(fmt.Errorf("owned challenge services unavailable"))
	}
	var req protocol.BoostChallengeRequest115
	if id == 680 {
		var e error
		req, e = protocol.DecodeBoostChallengeRequest115(p)
		if e != nil {
			return refuse(e)
		}
		if w.activeDungeon != nil || w.inTutorial || w.selectingDungeon || w.specialWarpPending {
			return refuse(fmt.Errorf("challenge reward requires town"))
		}
	} else {
		r, e := protocol.DecodeEventRequest115(p, false)
		if e != nil || r.Event != boostup.ChallengeEventID {
			return refuse(fmt.Errorf("invalid challenge query"))
		}
	}
	next, _, e := (&workflow.LootService{Store: w.store, Loot: w.loot}).ReconcileBoostChallenge(ctx, w.role, w.boostup, w.characters.BoostChallengeFacts)
	if e != nil {
		return refuse(e)
	}
	w.role = next
	var values [10]uint32
	if id == 680 {
		frame := p
		if len(raw) == 1 {
			frame = raw[0]
		} else if len(raw) > 1 {
			return refuse(fmt.Errorf("ambiguous event transport frame"))
		}
		key, e := w.boostOperations.requestKey(append([]byte("boost-challenge|"), frame...))
		if e != nil {
			return refuse(e)
		}
		var sent bool
		next, sent, e = (&workflow.LootService{Store: w.store, Loot: w.loot}).ClaimBoostChallenge(ctx, w.role, w.boostup, req, key)
		if e != nil {
			return refuse(e)
		}
		w.role = next
		if sent && w.notifyBoostMail != nil {
			w.notifyBoostMail(w.role.ID)
		}
		values[0], values[1] = uint32(req.Index), uint32(req.Action)
	}
	body, e := loot.BoostChallengeSnapshot(w.boostup, workflow.LootRole(w.role))
	if e != nil {
		return refuse(e)
	}
	return []outboundPacket{{"boost_challenge_status", 0, 2722, body}, {"boost_challenge_ack", 1, id, protocol.EventReply115(boostup.ChallengeEventID, values)}}, nil
}

func isBoostChallengeRequest(p []byte) bool {
	return len(p) >= 4 && binary.LittleEndian.Uint32(p) == boostup.ChallengeEventID
}
