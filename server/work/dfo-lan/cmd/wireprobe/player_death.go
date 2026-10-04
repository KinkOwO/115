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
	if w == nil || w.role.ID == 0 || w.activeDungeon == nil || !w.activeDungeon.Loaded {
		return nil, fmt.Errorf("player death requires owned loaded dungeon")
	}
	// 结算已经走完（N35 clear reward 已发）之后才死的：**结果不改写，但必须把玩家送回城**。
	//
	// 实机 2026-10-04 蔚蓝号：玩家打死 BOSS、结算链跑完（N31/N1658 → C46 → N35 → 翻牌），
	// 2 秒后又被 BOSS 的亡语机制秒了。此时 resultSent 已经是 true，旧判据直接把这次死亡
	// 拒掉 —— 客户端拿不到任何死亡应答，玩家卡在死亡画面，**只能退游戏**。
	//
	// 官服同一趟也是这个形状：c2s `C40 DIE_CHARACTER ×2` 配 s2c `N32 DIE_STATE ×4`，
	// 死亡照答，通关结果不受影响。所以这里照做：只回死亡确认与死亡状态，然后走
	// leaveDungeon 那条回城链（主循环看到 dungeon_leave_ack 会清掉副本会话）。
	// **不发 N33 FAIL_CLEAR**：那会让客户端把已经通关的一场当成失败。
	if w.resultSent {
		return w.deathAfterSettlement()
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

// deathAfterSettlement 处理「结算已走完之后的死亡」：结果不改写，只把玩家送回城。
//
// 这里**不动 pilotDeath** —— 它是「死亡倒计时 → 挑战失败」那条链的状态，置空后
// case 40 里那个 10 秒定时器会因 d == nil 直接返回，不会在通关之后再补一发
// FAIL_CLEAR（那正是会把已通关的一场标成失败的写法）。
func (w *worldSession) deathAfterSettlement() ([]outboundPacket, error) {
	body, err := protocol.PlayerDeathState(w.role.WireID)
	if err != nil {
		return nil, err
	}
	// **不要立刻把玩家弹回城**：征讨地下城允许用复活币（CMD41 → useCoinRevive，
	// 本仓已实现），立刻回城会让客户端根本不进复活界面、也就永远发不出 CMD41
	// （实机 2026-10-04 的日志里 client_frame 没有任何 id=41）。
	//
	// 所以这里只做「让死亡状态成立」这一件事：记下本局死亡、回 ACK40 + N32（state 0，
	// 进原生复活 UI）。真正的出口交给 case 40 的 10 秒倒计时 —— 它会根据 resultSent
	// 决定要不要补 N33 FAIL_CLEAR（已结算的不补，只回城），回城链里那一帧
	// N32 state=1（满血满蓝、解除幽灵态）由 leaveDungeon 照常补。
	if w.pilotDeath == nil || w.pilotDeath.Run != w.activeDungeon.RunID {
		w.pilotDeath = &odysseyDeath{Run: w.activeDungeon.RunID, Frames: map[[32]byte]uint32{}, Revives: map[[32]byte]bool{}}
	}
	w.pilotDeath.Dead = true
	return []outboundPacket{
		{"player_death_ack", 1, 40, []byte{1}},
		{"player_death_state", 0, 32, body},
	}, nil
}
