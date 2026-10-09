package main

import (
	"fmt"
	"log"
	"time"

	"dfolan/internal/game/protocol"
	"dfolan/internal/legion"
)

// 本文件是末世录的**阶段倒计时**（与维纳斯 venusStageTimer / venusStageTimeout 同构）。
//
// 规格来源：D:\115US-001\moshilu\10-军团末世录专有\1474-DUNGEONTIMEOUTTIME.md
//
//	「设置当前副本场景的倒计时。……本页结构表适用于末世录频道119和维纳斯频道99，
//	  均使用秒。」
//	「真实共享加载和资源释放完成后，紧随本人的 N30 发送。后续同阶段换房使用原
//	  开始时间。」
//	「不能以当前发送时间覆盖阶段最初开始时间，不能将频道106的毫秒特例套到末世录
//	  或维纳斯。」
//	「此包不授权门户、不证明加载完成，也不代替服务器超时判定。」
//
// 业主口径（2026-10-08）：「还有副本内的倒计时，倒计时结束自动退出副本，再次进入
// 就是刚刚退出的那一关」，并指定「最好按维纳斯的来」。

// apocalypseStageTimer 同步末世录阶段倒计时（NOTI1474 DUNGEON_TIMEOUT_TIME）。
//
// 正文 8B = [阶段时限秒 @0, 阶段开始 Unix 秒 @4]。时限取 RunPlan 的 [phase info]
// 时钟（plan.PhaseSeconds[stage]），缺失时回退 legion.ApocalypsePhaseLimits。
// 开始时刻在该阶段第一次真实加载完成时冻结进 run.StageClock，重进同一关
// （撤退/超时）复用原值 —— 否则每次发包都会把倒计时续满，超时判死永远不成立。
func (w *worldSession) apocalypseStageTimer(now time.Time) []outboundPacket {
	if w == nil || w.apocalypse == nil || w.activeDungeon == nil || w.legion == nil {
		return nil
	}
	stage := legion.ApocalypseStageOfDungeon(w.activeDungeon.Definition.ID)
	if stage < 0 || stage >= len(legion.ApocalypseStageDungeons) {
		return nil
	}
	limit, ok := w.apocalypsePhaseLimit(stage)
	if !ok || limit == 0 {
		// 攻坚房间不设限时（规格：不存在合法时间范围时不发送）。
		return nil
	}
	run := w.apocalypse
	if run.StageClock[stage].IsZero() {
		run.StageClock[stage] = now
	}
	body, err := protocol.LegionDungeonTimeout115(run.StageClock[stage], time.Duration(limit)*time.Second)
	if err != nil {
		log.Printf("apocalypse stage timer encode failed (stage %d): %v", stage, err)
		return nil
	}
	return []outboundPacket{{"apocalypse_stage_timer_sync", 0, legion.NotiDungeonTimeoutTime, body}}
}

// apocalypsePhaseLimitOverride 是**诊断用**的阶段时限覆盖（秒）。
//
// 业主 2026-10-08 要求「把所有关卡倒计时改为 20 秒，便于测试；测试完成后恢复」，
// 所以这里留一个包级变量：>0 时替换所有战斗关的时限。
//
// ⚠️ 按仓库开关原则（server/AGENTS.md §6）：**只用于本地调试，不设 flag/env 入口**；
// 测试完成后把这个值改回 0 即恢复源时限。
//
// 源时限（[phase info] 时钟 → legion.RunPlan.PhaseSeconds，按副本表下标）：
//
//	下标 0 = 攻坚房间（不设限）/ 1..3 = 300 秒 / 4..5 = 600 秒
var apocalypsePhaseLimitOverride uint32 = 0 // 已恢复源时限（300/300/300/600/600）

// apocalypsePhaseLimit 返回某阶段的倒计时上限（秒），**唯一来源** = RunPlan 的
// [phase info] 时钟（contents/system/legionsystem 的 PhaseClock，经
// legion.BuildEntryPlan 编进 plan.PhaseSeconds）。攻坚房间（下标 0）不设限时。
func (w *worldSession) apocalypsePhaseLimit(stage int) (uint32, bool) {
	if stage <= 0 || w.legion == nil || w.legion.plan == nil {
		return 0, false
	}
	if stage >= len(w.legion.plan.PhaseSeconds) {
		return 0, false
	}
	secs := w.legion.plan.PhaseSeconds[stage]
	if secs <= 0 {
		return 0, false
	}
	if apocalypsePhaseLimitOverride > 0 {
		return apocalypsePhaseLimitOverride, true
	}
	return uint32(secs), true
}

// apocalypseStoryPause 应答末世录终局通关视频期间的 CMD191（剧情暂停/恢复），
// 与 venusStoryPause / forestStoryPause 同构。
//
// 家族协议：暂停与恢复都由服务端回原生 **N170（DUNGEON_EVENT_STORY_PAUSE，
// StoryPauseNotice）**；**恢复（state=1）= 视频播完**，此时追加 leave 态
// N2895（State5 + Outcome1）—— 家族约定里「closes the operation panel once the
// clear movie has finished」的那一帧（维纳斯 `VenusLeaveInfo`）。
//
// 实机取证（业主 2026-10-08 19:27 局，客户端 trace）：
//
//	11:30:56 SEND ENUM_CMDPACKET_DUNGEON_EVENT_STORY_PAUSE (29)
//	11:30:56 RECV ENUM_NOTIPACKET_DUNGEON_EVENT_STORY_PAUSE (16)   ← 通用处理器回的
//	11:30:56 SEND ENUM_CMDPACKET_LEGION_REWARD_END (45)            ← 客户端自己发的 2046
//	11:30:56 SEND ENUM_CMDPACKET_DUNGEON_EVENT_STORY_PAUSE (29)    ← 恢复
//
// 也就是说末世录**确实会**在终局演出期间发 CMD191；此前落到通用处理器，
// 所以「演出播完」这一刻没有任何末世录专属收尾。
func (w *worldSession) apocalypseStoryPause(p []byte) ([]outboundPacket, []map[string]any, error) {
	r, err := protocol.DecodeStoryPause(p)
	if err != nil {
		return nil, nil, err
	}
	run := w.apocalypse
	if run == nil || !run.FinalDone {
		return nil, nil, fmt.Errorf("apocalypse story pause outside the finale")
	}
	notice, err := protocol.StoryPauseNotice(w.role.WireID, r)
	if err != nil {
		return nil, nil, err
	}
	plan := []outboundPacket{{"apocalypse_story_pause", 0, 170, notice}}
	notes := []map[string]any{{
		"kind":         "apocalypse_story_pause",
		"character_id": w.role.ID,
		"state":        r.State,
		"kind_byte":    r.Kind,
	}}
	if r.State == 1 && !run.StoryFinished {
		// 视频播完：发 leave 态收起右上角面板（家族约定 State5 + Outcome1）。
		run.StoryFinished = true
		plan = append(plan, outboundPacket{
			"apocalypse_terminal_leave", 0, legion.NotiLegionInfo,
			legion.ApocalypseInfo(legion.ApocalypseLeaveInfo(run.Choice, run.Stage-1)),
		})
		notes = append(notes, map[string]any{
			"kind": "apocalypse_clear_movie_finished", "character_id": w.role.ID,
		})
	}
	return plan, notes, nil
}

// apocalypseSelectWindowSeconds 是「难度选择框开窗后多久强制关闭」的秒数。
//
// 业主口径（2026-10-08）：「只要识别到难度选择框，15 秒后没有选择难度进入副本，
// 就强制关闭难度选择框，至于难度选择框中的倒计时不用管了」。
//
// 背景：那个 15 秒倒计时**不在服务端** —— 参考抓包里 CMD2354 的两条成功响应
// （两个难度、两次开窗）的操作记录**全是 0**：
//
//	016b000100000000…  byte@8=0x00  u32@9..12=0
//	016b000200000000…  byte@8=0x00  u32@9..12=0
//
// 规范页说的「u32@6 用于倒计时截止值」在实测里并不下发，所以倒计时只能是客户端
// 本地行为。业主要的是「到点把框关掉、别卡死」，所以这里由**服务端主动关窗**。
const apocalypseSelectWindowSeconds = 15

// apocalypseSelectWindow 是一次挂起的「难度框强制关闭」。
type apocalypseSelectWindow struct {
	deadline time.Time
	closed   bool
}

// armApocalypseSelectWindow 在开窗（CMD2354 action1）时登记强制关闭。
func (w *worldSession) armApocalypseSelectWindow(now time.Time) {
	if w == nil || w.apocalypse == nil {
		return
	}
	w.apocalypseSelectWindowPending = &apocalypseSelectWindow{
		deadline: now.Add(time.Duration(apocalypseSelectWindowSeconds) * time.Second),
	}
}

// cancelApocalypseSelectWindow 在确认难度（action2）或重新开团时撤销挂起。
func (w *worldSession) cancelApocalypseSelectWindow() {
	if w == nil || w.apocalypseSelectWindowPending == nil {
		return
	}
	w.apocalypseSelectWindowPending.closed = true
}

// apocalypseSelectWindowDue 在超时后强制关闭难度选择框。
//
// 关闭帧 = CMD2354 成功响应 + 操作记录 `byte@5`（正文 `@8`）置 1 ——
// 规范 `2354-LEGIONOPERATION成功响应.md` 明写「内部 byte@5 非 0 走**关闭/取消**
// 分支」，所以这一位就是客户端认的关窗开关。上一轮我误把 deadline 写进 `@7..10`、
// 压到了这一位（值非 0），客户端于是直接走取消分支、点 Open 毫无反应 ——
// 反过来说明这一位确实是开关。
//
// 关窗后把难度状态复位成**未选**（Choice=FF）并发等待态 N2895：这样即使客户端
// 只是收起窗口、没清掉选择标记，玩家重新开战时也不会被旧状态卡住 ——
// 这正是业主说的「不然就卡死在那里了」。
//
// 若期间已经确认了难度（action2），cancelApocalypseSelectWindow 会把它标掉；
// 已经进图（Entered）时也不再关窗。
func (w *worldSession) apocalypseSelectWindowDue(now time.Time) ([]outboundPacket, []map[string]any) {
	pending := w.apocalypseSelectWindowPending
	if pending == nil || pending.closed || now.Before(pending.deadline) {
		return nil, nil
	}
	pending.closed = true
	run := w.apocalypse
	if run == nil || run.Choice != 0xff || run.Entered {
		return nil, nil
	}
	run.WindowDeadline = time.Time{}
	plan := []outboundPacket{
		{
			Name: "apocalypse_select_window_closed", Kind: 1, ID: legion.CmdOperationSelect,
			Payload: legion.ApocalypseOperationClose(),
		},
		{
			Name: "apocalypse_select_window_reset", Kind: 0, ID: legion.NotiLegionInfo,
			Payload: w.legion.apocalypseInfo(),
		},
	}
	return plan, []map[string]any{{
		"kind":         "apocalypse_select_window_forced_closed",
		"character_id": w.role.ID,
		"seconds":      apocalypseSelectWindowSeconds,
	}}
}

// apocalypseResetStageClock 把某阶段的倒计时开始时刻重置为 now。
//
// 业主口径（2026-10-08）：「在副本内点击撤退，重进副本，还是原来的阶段，但是
// 时间没有重置，这是错误的，重进以后时间需要重置，**不管是点击撤退、死亡后撤退、
// 到时间撤退，重进副本都需要重置时间**」。
//
// 这与 1474 规格的「后续同阶段换房使用原开始时间」**有意分歧**：规格那条针对的是
// 同一阶段内的换房/重载不续时（防止每次发包把倒计时续满），而业主的语义是
// 「每一次重新进入该关都重新计时」。两者并存的方式见调用点：
//   - 阶段内换房/加载回执重复触发 → 不动（apocalypseStageTimer 只在零值时冻结）；
//   - **CMD2045 重新进关**（含撤退续关、超时续关）→ 调本函数重置。
func (w *worldSession) apocalypseResetStageClock(stage int, now time.Time) {
	if w == nil || w.apocalypse == nil {
		return
	}
	if stage < 0 || stage >= len(w.apocalypse.StageClock) {
		return
	}
	w.apocalypse.StageClock[stage] = now
}

// apocalypseStageTimeout 判定阶段倒计时到期（挑战失败），与 venusStageTimeout 同构。
//
// 到期 = now 超过该阶段冻结开始时间 + 时限。1474 规格强调「不代替服务器超时判定」，
// 所以失败判定归服务端。
//
// 退场用原生超时失败信号 **N33（FAIL_CLEAR_DUNGEON，reason 100=timeout）** 开路，
// 客户端自己驱动退场演出；**不发 42-ack**（那是对从未发生的客户端 GIVEUP 请求的
// 伪造应答，同 venusStageTimeout / ispinsTimeout 的既有口径）。城镇序列照
// leaveDungeon 原样，末尾补 N32 原生复活（失败演出把角色置入死亡态）与保留原阶段的
// 等待态 N2895。
//
// **不清进度**（业主口径「再次进入就是刚刚退出的那一关」）：Stage / Cleared /
// Choice / StageClock 全部保留，只置 Suspended，下次 CMD2045 按 ContinueStage()
// 重建同一关。源 DGN 声明 [no giveup panalty]，无惩罚语义。
//
// 结算期守卫：翻牌链完成前后不拽人（Completed / completionSent / resultSent）。
func (w *worldSession) apocalypseStageTimeout(now time.Time, event func(map[string]any)) []outboundPacket {
	if w == nil || w.apocalypse == nil || w.activeDungeon == nil || w.legion == nil ||
		w.activeDungeon.Completed() || w.completionSent || w.resultSent {
		return nil
	}
	stage := legion.ApocalypseStageOfDungeon(w.activeDungeon.Definition.ID)
	if stage < 0 || stage >= len(legion.ApocalypseStageDungeons) {
		return nil
	}
	limit, ok := w.apocalypsePhaseLimit(stage)
	if !ok || limit == 0 {
		return nil
	}
	started := w.apocalypse.StageClock[stage]
	if started.IsZero() || now.Sub(started) < time.Duration(limit)*time.Second {
		return nil
	}
	run := w.apocalypse
	route, err := w.leaveDungeon()
	if err != nil {
		log.Printf("apocalypse stage timeout leave failed (stage %d): %v", stage, err)
		return nil
	}
	w.activeDungeon = nil
	w.selectingDungeon = false
	w.approvedDungeonGate = 0
	w.drops = nil
	w.deathSent = nil
	w.completionSent = false
	w.resultSent = false
	w.resetCards()
	w.apocalypsePending = nil
	w.apocalypseAdvancePending = nil
	w.apocalypseStageCleared = [6]bool{}
	// 保留进度：Stage / Cleared / Choice / StageClock 全部不动，只标记挂起。
	run.Suspended = true
	if event != nil {
		event(map[string]any{
			"kind":         "apocalypse_stage_timeout",
			"character_id": w.role.ID,
			"stage":        stage,
			"choice":       run.Choice,
			"started":      started.Unix(),
			"limit":        int(limit),
			"resume_stage": run.ContinueStage(),
		})
	}
	packets := []outboundPacket{{"apocalypse_time_limit_failed", 0, 33, protocol.DungeonFailClear(100)}}
	for _, p := range route {
		if p.Name != "dungeon_leave_ack" {
			packets = append(packets, p)
		}
	}
	revive, err := protocol.PlayerDeathState(w.role.WireID)
	if err == nil {
		revive[2] = 1
		packets = append(packets, outboundPacket{"apocalypse_timeout_actor_revived", 0, 32, revive})
	}
	// 末尾等待态 N2895：难度与原阶段保留（State2 + 原 Choice/Stage/marks），
	// 右上角面板继续显示已选难度，再点开始即续同一关。
	packets = append(packets, outboundPacket{
		"apocalypse_info_timeout_waiting", 0, legion.NotiLegionInfo, w.legion.apocalypseInfo(),
	})
	return packets
}
