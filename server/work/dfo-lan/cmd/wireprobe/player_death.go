package main

import (
	"crypto/sha256"
	"dfolan/internal/game/protocol"
	"dfolan/internal/legion"
	"encoding/hex"
	"fmt"
	"time"
)

// [MERGE-20260928-DEATH-FAIL-TIMEOUT] 死亡后的失败倒计时。
//
// 客户端进复活 UI 后只会等，不会发请求，所以「倒计时结束 → 挑战失败」只能由服务端
// 推进（见 main.go 的 case 40）。实机截图显示客户端从 9~10 秒开始倒数，这里取 10 秒。
const deathFailTimeout = 10 * time.Second

// deathFailTimeoutReason 是 NOTI33 FAIL_CLEAR_DUNGEON 的原因码：100 = 倒计时超时
// （0 = 「默认死亡」，奥德赛禁复活的立即判负用它，与 Elvenmere 同形）。
const deathFailTimeoutReason byte = 100

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

// deathFailLeave 是「死亡 → 挑战失败 → 回城」那一步的实际动作，原样取自 case 40 里
// 10 秒定时器的闭包（[MERGE-20260928-DEATH-FAIL-TIMEOUT]）：
//
//	① 发 NOTI33 FAIL_CLEAR_DUNGEON（reason：100 = 倒计时超时，0 = 默认死亡）；
//	② 走 leaveDungeon 把玩家送回城 —— 只发 FAIL_CLEAR 客户端不会自己走
//	   （实机 2026-09-28：收到 FAIL_CLEAR 后 25 秒毫无动作，直到玩家手动放弃才回城）；
//	③ 自己清副本会话（这里绕过了主循环里 dungeon_leave_ack 的清理）。
//
// 调用点：
//   - case 40 的 10 秒定时器（死亡后没复活 → 超时判负），reason=100；
//   - **奥德赛禁复活时立即调用**（业主 2026-10-06：死亡即回城，不等倒计时），reason=0。
//
// 重复调用是安全的：第一次跑完会清掉 activeDungeon，第二次在入口直接返回。
// deathFailLeave 是失败结算出口：发 NOTI33 FAIL_CLEAR_DUNGEON + 回城链。
// 超时判负（reason=100）继续用它，行为与加 mod 之前一致。
func (c *gameConnection) deathFailLeave(reason byte) { c.deathLeave(reason, true) }

// deathGiveUpLeave 是不做失败结算的出口：只走 leaveDungeon() 的放弃/离场链，不发 N33。
// 为什么需要它（业主 2026-10-06 实机）：用失败结算出口后客户端会进虚弱状态；
// 服务端没有虚弱这个概念（全仓 grep 无命中），那是客户端收到失败结算后自己进的 ——
// protocol.DungeonFailClear 注释写明 reason 0 = default defeat/death，
// 会 trigger the native player death scene and failure settlement。
// 死亡即回城只需要回城、不需要失败结算 => 走这条链；leaveDungeon() 自带
// N32 state=1（满血满蓝、解除幽灵态），所以回城是干净的。
func (c *gameConnection) deathGiveUpLeave() { c.deathLeave(0, false) }

func (c *gameConnection) deathLeave(reason byte, sendFailClear bool) {
	if c == nil || c.worldState == nil {
		return
	}
	w := c.worldState
	d := w.pilotDeath
	if d == nil || !d.Dead || w.activeDungeon == nil {
		return
	}
	// 维纳斯军团口径（2026-10-07 业主要求）：死亡被请离副本后，再次进入同关
	// 必须重置关卡倒计时——清掉该阶段的冻结开始时刻，重进后从满额时限重新
	// 起算（第三十八轮「死亡重载复用原值」的口径就此作废）。进度/难度锁/
	// 遗物保留不变；复活币复活（不请离）不受影响，超时判定照常。
	if w.venus != nil && w.venus.entered {
		if stage, err := legion.VenusStageOfDungeon(w.activeDungeon.Definition.ID); err == nil &&
			stage >= 0 && stage < len(w.venus.stageClock) {
			w.venus.stageClock[stage] = time.Time{}
			c.event(map[string]any{"kind": "venus_death_stage_timer_reset", "character_id": w.role.ID,
				"stage": stage})
		}
	}
	// 末世录军团口径（2026-10-08 实机）：死亡被判负请离副本后，run 必须标记为
	// **挂起**，否则玩家点右上角面板的「进入」时服务端会把续关请求当成**首次入场**
	// 而拒绝 —— 实测 events.jsonl：
	//
	//	{"id":2045,"kind":"legion_refused",
	//	 "reason":"apocalypse first entry carries stage 2, want 0",
	//	 "request_hex":"…6b00000002000000000000"}
	//
	// 也就是客户端**正确**地带着保存阶段（stage=2）发 CMD2045，而
	// `apocalypseEnter` 只在 `run.Suspended` 为真时才走续关分支（规格 2045
	// G0452：「已暂停的同一作战走 ResumeApocalypse：阶段须等于保存值」）。
	// 这条死亡链原先只为维纳斯清了阶段时钟，漏了末世录的挂起标记 ⇒
	// 症状正是业主报的「点进入没有任何反应」。
	//
	// 保留 Stage / Cleared / Choice / RunID 全部不动，只置 Suspended；
	// 该关倒计时由 apocalypseEnter 按 ResumeStage 重置（第 546 行）。
	// 复活币复活（不请离）不走本函数，不受影响。
	if w.apocalypse != nil && !w.apocalypse.Suspended {
		run := w.apocalypse
		run.Suspended = true
		c.event(map[string]any{"kind": "apocalypse_death_suspended", "character_id": w.role.ID,
			"stage": run.Stage, "resume_stage": run.ContinueStage(), "choice": run.Choice})
	}
	// 苏醒之森军团口径（2026-10-09 业主实机）：死亡被判负请离副本后，该关
	// 倒计时必须重置（「死亡强制退出，右上角的倒计时没有刷新，正常情况应该
	// 重置」）。与维纳斯同款做法：清掉冻结的起算时刻，重进按满额 60 分钟
	// 重新起算；进度/通关状态保留。复位帧由调用方补发（deathLeave 里发完
	// leave 链之后）—— 这里先把时钟清掉并记一条事件。
	if w.forest != nil && w.activeDungeon != nil {
		if stage := w.forestStageOfActiveRun(); stage >= 0 {
			reset := w.forestStageTimerReset(time.Now(), stage, "death")
			if c.sendPlan(reset, nil) != nil {
				return
			}
			c.event(map[string]any{"kind": "forest_death_stage_timer_reset", "character_id": w.role.ID, "stage": stage})
		}
	}
	// [AZURE-DEATH-AFTER-CLEAR] 结算已经走完的**只回城、不补 FAIL_CLEAR**：
	// 补了会把一场已经通关并发了奖的挑战标成失败。
	if sendFailClear && !w.resultSent {
		if err := c.output.send(0, 33, protocol.DungeonFailClear(reason)); err != nil {
			return
		}
	}
	leave, e := w.leaveDungeon()
	if e != nil {
		c.event(map[string]any{"kind": "death_fail_leave_error", "error": e.Error()})
		return
	}
	if c.sendPlan(leave, func(p outboundPacket) {
		if p.ID == 1361 {
			c.event(map[string]any{"kind": p.Name, "character_id": w.role.ID, "id": p.ID, "type": p.Kind, "plain_hex": hex.EncodeToString(p.Payload), "path": "death_fail_leave"})
		}
	}) != nil {
		return
	}
	w.activeDungeon = nil
	w.bleedingMineStart = nil
	c.event(map[string]any{"kind": "death_fail_leave", "run": d.Run, "reason": reason, "steps": len(leave) + 1})
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
