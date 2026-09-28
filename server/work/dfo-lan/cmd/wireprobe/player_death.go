package main

import (
	"crypto/sha256"
	"dfolan/internal/game/protocol"
	"fmt"
	"time"
)

// [MERGE-20260928-DEATH-FAIL-TIMEOUT] 死亡后的失败倒计时。
//
// 客户端进复活 UI 后只会等，不会发请求，所以「倒计时结束 → 挑战失败」只能由服务端
// 推进（见 main.go 的 case 40）。实机截图显示客户端从 9~10 秒开始倒数，这里取 10 秒。
const deathFailTimeout = 10 * time.Second

func (w *worldSession) playerDeath(p []byte, frames ...[]byte) ([]outboundPacket, error) {
	if w == nil || w.role.ID == 0 || w.activeDungeon == nil || !w.activeDungeon.Loaded || w.resultSent {
		return nil, fmt.Errorf("player death requires owned loaded dungeon")
	}
	if _, err := protocol.DecodePlayerDeath(p); err != nil {
		return nil, err
	}
	body, err := protocol.PlayerDeathState(w.role.WireID)
	if err != nil {
		return nil, err
	}
	if w.pilotDeath == nil || w.pilotDeath.Run != w.activeDungeon.RunID {
		w.pilotDeath = &odysseyDeath{Run: w.activeDungeon.RunID, Frames: map[[32]byte]uint32{}, Revives: map[[32]byte]bool{}}
	}
	d := w.pilotDeath
	if len(frames) > 0 {
		key := sha256.Sum256(frames[0])
		if seq, seen := d.Frames[key]; seen {
			if !d.Dead || seq != d.Sequence {
				return []outboundPacket{{"player_death_replay_ack", 1, 40, []byte{1}}}, nil
			}
		} else {
			if len(d.Frames) >= 256 {
				return nil, fmt.Errorf("death report limit exceeded")
			}
			if !d.Dead {
				d.Sequence++
			}
			d.Frames[key] = d.Sequence
		}
	} else if !d.Dead {
		d.Sequence++
	}
	d.Dead = true
	// Repeated reports project the same death state, never rewards or charges.
	plan := []outboundPacket{{"player_death_ack", 1, 40, []byte{1}}, {"player_death_state", 0, 32, body}}
	if w.activeDungeon != nil && w.activeDungeon.Definition.ID == 100003126 {
		// Elvenmere 爬塔地下城：原生禁止复活币（die countdown 0），单人角色死亡即代表挑战失败。
		// 下发 NOTI 33 (ENUM_NOTIPACKET_FAIL_CLEAR_DUNGEON)，驱动客户端进入 DUNGEON_STATE_FAIL_CLEAR，
		// 激活完整的 Death Scene 死亡镜头、死亡动作与挑战失败结算。
		plan = append(plan, outboundPacket{"dungeon_fail_clear", 0, 33, protocol.DungeonFailClear(0)})
	}
	return plan, nil
}
