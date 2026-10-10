package main

import (
	"context"
	"dfolan/internal/dungeon"
	"dfolan/internal/game/protocol"
	"dfolan/internal/legion"
	"fmt"
	"time"
)

// apocalypseStageEntry 载入一个末世录阶段副本并生成进图帧序。
//
// 与 venusStageEntryShared 同构，但有一条末世录专属差异（抓包
// D:\zhuabao\captures\20261008-105227 · US-Local 会话 20261008_105231 实证）：
// 难度与阶段状态走 NOTI2895 LEGION_INFO（由 apocalypse_run.go 在命令应答里
// 发出），不是维纳斯的 N2655；而第一关是攻坚房间
// （contents/2026/apocalypse/town/navigationroom_watingroom.map，客户端以
// CMD2062 直进，US-Local 实录 apocalypse_navigation_prepared）。
//
// 返回的 frames[0] 是 1 字节成功应答，调用方按命令语义替换或保留。
func (w *worldSession) apocalypseStageEntry(sel protocol.DungeonSelection) (*dungeon.Session, []outboundPacket, error) {
	if w == nil || w.dungeons == nil {
		return nil, nil, fmt.Errorf("apocalypse stage entry without a dungeon catalog")
	}
	s, err := dungeon.Select(*w.dungeons, sel, w.level, nil)
	if err != nil {
		return nil, nil, fmt.Errorf("apocalypse dungeon %d entry unavailable: %w", sel.ID, err)
	}
	// [LEGION-ARENA-BOSS] 末世录每一关的战斗都发生在**进图房间**，源迷宫的
	// [boss map] 是未使用元数据。不置这一位，客户端清完房间后上报的 CMD117
	// 会被通用 BossCheck 判死 —— 实机症状：进图成功、清完怪、然后
	// `dungeon_request_refused: boss check target is not a source boss in this room`，
	// 于是**进不了下一个房间**。与伊斯/苏醒之森同款处理。
	s.ArenaBoss = true
	// ★ 复活币按**关卡**计数：每次真实载入一间房就把「本关已用次数」归零。
	// 三条进关路径（CMD2045 进/续关、CMD2062 客户端直进、服务端主动推进）
	// 都经过本函数，所以这里是唯一的归零点。
	// 上限由所选作战的 `[allow coin]` 决定，见 apocalypseCoinLimit。
	if w.apocalypse != nil {
		w.apocalypse.RevivesUsedThisStage = 0
	}
	noteMazeEntry(s)
	ctx, cancel := context.WithTimeout(context.Background(), apocalypseEntryCtxTimeout)
	defer cancel()
	var channelCtx *[2]byte
	if w.characters != nil {
		channelCtx = &w.characters.ChannelContext
	}
	frames, err := w.dungeonEntryPlanImpl(ctx, "apocalypse_stage_ack", 16, sel, s, channelCtx)
	if err != nil {
		return nil, nil, err
	}
	// N28 之前补 N3（角色状态 → 副本态）与 N27（选图上下文），与维纳斯/SemiRaid
	// 军团进图序列同款；客户端在收到 N27 前还停在选图状态，缺它会停在选图界面。
	actorState, se := protocol.UserState(w.role.WireID, protocol.UserStateDungeon)
	if se != nil {
		return nil, nil, se
	}
	inserted := false
	plan := make([]outboundPacket, 0, len(frames)+3)
	for _, pkt := range frames {
		if pkt.ID == 28 && !inserted {
			plan = append(plan,
				outboundPacket{"apocalypse_actor_dungeon", 0, 3, actorState},
				outboundPacket{"apocalypse_navigation_selection", 0, 27, protocol.EnterDungeonSelection()},
			)
			inserted = true
		}
		plan = append(plan, pkt)
	}
	// 末世录进图会话样板：与维纳斯/军团共用字段清理，避免上一场的残留
	// 状态（掉落、死亡计数、结算标志）渗进新阶段。阶段清怪标记按副本号保留：
	// 转阶段后新副本的槽是空的，而同一关重载（死亡重进）不该再推一次 N2895。
	w.deathSent = map[uint16]bool{}
	w.drops = nil
	w.resetCards()
	w.completionSent = false
	w.completionErr = nil
	w.resultSent = false
	w.selectingDungeon = false
	w.approvedDungeonGate = 0
	w.pendingTownArrival = nil
	w.leaveScene()
	w.activeDungeon = s
	return s, plan, nil
}

// apocalypseStageAdvanceDelay 是**战斗关**（第 1..5 关）「BOSS 死 → 服务端推进
// 下一关」的等待时间。
//
// 参考抓包 20261008-105227 的 47.49–47.56s 是：s2c N38 → c2s CMD117 →
// s2c N2895 → **c2s CMD2062（47.56）** → s2c 下一关的整套进图帧。
// 1 号这边把 N2895 发得与参考**逐字节相同**、前后帧序也完全一致，客户端
// 仍然不发 CMD2062（2026-10-08 三局实测），所以推进不能再依赖它。
//
// 这里的策略是：清关后先照常发 N2895，然后**服务端自己**在 delay 之后推进
// 下一关（与 CMD2045 那次进图完全同一条路径）。客户端的 CMD2062 仍然是快
// 路径 —— 它先到就取消这次挂起，避免重复进图。
//
// 时长经过两轮校正：
//
//	800ms — 本实现最初自定（「短于玩家感知阈值」），无依据；
//	5s    — 照 2 号实录的拾取窗 policy_seconds=5（2 号权威抓包 105.8s 清关后
//	        1.4 秒就发下三帧、节奏其实更紧）；
//	**3s** — 业主 2026-10-08 明确口径：「第 1 关 BOSS 打完后 3 秒自动进入下一关，
//	        1 到 4 关都这样设计」。
const apocalypseStageAdvanceDelay = 3 * time.Second

// apocalypseNavigationAdvanceDelay 是**攻坚房间**（stage0，选完难度进来的那张
// 没有怪的地图）「碰到门 → 进下一关」的等待时间。
//
// 业主 2026-10-08：「第 0 关（选择难度后进入的地图，没有怪物）**碰到门后立马
// 进入下一关**，现在需要等待 5 秒」。所以这一档是 **0**：攻坚房间的门一开就
// 立刻推进，不要任何额外停留。
//
// 攻坚房间没有 BOSS，也就不存在「看完死亡演出再走」的需求；参考抓包那一跳
// （47.49 N2895 → 47.56 下一关进图）本来就只有 **70ms**，即时推进与参考一致。
const apocalypseNavigationAdvanceDelay = 0

// apocalypseAdvance 是一次挂起的「服务端主动推进下一关」。
type apocalypseAdvance struct {
	at time.Time
	// stage 是要载入的目标阶段下标（apocalypseStageDungeons 的下标）。
	stage int
	// done 置位后本任务作废（客户端提前发 CMD2062 时会走快路径）。
	done bool
}

// armApocalypseAdvance 登记一次延迟推进。
//
// 重复调用会**替换**已有的挂起任务（而不是被它挡住）—— 实测客户端的行为不一致：
// 有的关卡它发 CMD117（投影跑，排一次），有的关卡它直接走门（投影不跑）。若把
// 「已有挂起」当成「别再排」，就会永久卡在某个关卡（2026-10-08 15:25 实机：
// 第1关清关后客户端没发 CMD117，于是第1关的挂起排不上，第2关清关时又被
// 第1关残留的挂起挡住，角色停在副本里）。
func (w *worldSession) armApocalypseAdvance(stage int, delay time.Duration) bool {
	if w == nil || w.apocalypse == nil || w.activeDungeon == nil {
		return false
	}
	if stage < 0 || stage >= len(legion.ApocalypseStageDungeons) {
		return false
	}
	if p := w.apocalypseAdvancePending; p != nil && !p.done && p.stage == stage && time.Now().Before(p.at) {
		// 同一目标的挂起还在计时：不重复排。
		return false
	}
	w.apocalypseAdvancePending = &apocalypseAdvance{at: time.Now().Add(delay), stage: stage}
	return true
}

// cancelApocalypseAdvance 在客户端自己发 CMD2062 时调用：走快路径，撤销挂起。
func (w *worldSession) cancelApocalypseAdvance() {
	if w == nil || w.apocalypseAdvancePending == nil {
		return
	}
	w.apocalypseAdvancePending.done = true
}

// apocalypseAdvanceDue 在到期时推进下一关，返回要发的整套进图帧。
// 未到期、没有挂起任务或已被取消时返回 nil。
func (w *worldSession) apocalypseAdvanceDue(now time.Time) ([]outboundPacket, []map[string]any) {
	pending := w.apocalypseAdvancePending
	if pending == nil || pending.done || now.Before(pending.at) {
		return nil, nil
	}
	pending.done = true
	if w.apocalypse == nil || w.legion == nil {
		return nil, nil
	}
	run := w.legion.apocalypseRun()
	if !run.Entered || run.Ended {
		return nil, nil
	}
	sel := protocol.DungeonSelection{ID: legion.ApocalypseStageDungeons[pending.stage], Difficulty: 0, Party: 65535}
	sess, frames, err := w.apocalypseStageEntry(sel)
	if err != nil {
		return nil, []map[string]any{{
			"kind":    "apocalypse_stage_advance_failed",
			"stage":   pending.stage,
			"dungeon": sel.ID,
			"reason":  err.Error(),
		}}
	}
	// frames[0] 是 1 字节成功应答（走 select_ack 语义）。服务端主动推进时没有
	// 对应的客户端命令，所以丢掉它，只发真正的进图帧序。
	plan := make([]outboundPacket, 0, len(frames))
	for i, pkt := range frames {
		if i == 0 {
			continue
		}
		plan = append(plan, pkt)
	}
	// ★ run.Stage 记的是**副本表下标**（0=攻坚房、1=第1关、2=第2关……）；
	// 发布出去的 NOTI2895 @13 才是「下标 + 1」（攻坚房=1、第1关=2、第2关=3，
	// 抓包锚点 41.12s/47.54s/73.59s）。发布时 +1 在 apocalypseInfo 里做。
	//
	// 此前这里写 `pending.stage + 1`，把「发布刻度」存进了状态量，于是：
	//   16:33 客户端回送 @17=2（它在第 1 关），而 ContinueStage 返回 2 → 跳到第 2 关；
	//   16:43 改成存下标后客户端回送 @17=1，服务端期望 0 → 直接拒绝。
	// 两个方向都失败过，所以状态量统一成下标、发布量统一 +1，只有一处换算。
	// 载入 pending.stage 号房间 → 已进入过的房间数 = pending.stage + 1。
	run.Stage = pending.stage + 1
	// ★ 推进后必须**补发权威 NOTI2895**。
	//
	// 客户端把收到的 N2895 @13 **原样回送**成续关请求 CMD2045 @17（见
	// apocalypseInfo 的刻度注释）。此前这条「服务端主动推进」的路径只加载房间、
	// 不发 N2895，客户端手上的阶段号就停在推进之前的值，于是实机出现
	// 「角色死亡被请离后点右上角『进入』没有任何反应」：
	//
	//	apocalypse_navigation_cleared  N2895 @13=0   ← 客户端最后看到的阶段
	//	apocalypse_server_advanced     run.Stage=2    ← 服务端自己推进，未告知客户端
	//	CMD2045 @17=0 → "apocalypse resume stage 0 does not match the saved stage 2"
	//
	// 战斗关的清关投影（apocalypseStageProjection）本来就会发这一帧，所以只有
	// 「攻坚房/导航房清空后自动进下一关」这条路径漏了。位置放在进图帧序**之后**：
	// 先让客户端把新房间载完，再告知它当前阶段。
	// ★ A2（2026-10-09 实机对照结论）：这里曾经补发一帧 N2895，已删除。
	//
	// 第4轮曾在此补发 `apocalypse_advance_stage_published`，目的是让客户端知道
	// 服务端主动推进后的新阶段（客户端把 N2895 @13 原样回送成 CMD2045 @17）。
	// 但业主实机对照（会话 20261009_023423 的帧序 vs 老版本 20:42）证明：
	// **老版本从不发这类「房间里补发的 N2895」，而它地图内一切正常**；补发之后
	// 地图内出现同源症状：复活币数字 99（老版本 8）、难度2 死亡出现复活提示。
	// ⇒ 每进一间房多给客户端一帧作战信息，会把客户端的作战状态冲掉。
	//
	// 该补发**已无必要**：续关阶段由服务端**容忍陈旧阶段**兜底
	//（apocalypse_run.go 的 resumeStale 分支：客户端报旧值也按保存阶段续关）。
	note := map[string]any{
		"kind":         "apocalypse_server_advanced",
		"character_id": w.role.ID,
		"stage":        run.Stage,
		"dungeon":      sel.ID,
		"map":          sess.Room.Map,
		"monsters":     len(sess.Monsters),
	}
	return plan, []map[string]any{note}
}

// apocalypseAdvanceSweep 是推进链的**权威兜底**：只要「当前正在打的末世录关卡
// 已经清空」而没有任何挂起推进，就补排一次。
//
// 为什么不能只依赖清怪投影：实测客户端行为不一致 —— 有的关卡它在怪死后发
// CMD117（投影跑），有的关卡它直接走门不发（2026-10-08 15:25 第 1 关就是
// 后者），于是投影没跑、N2895 没发、下一关也没排，角色停在副本里。
// 这个 sweep 每 tick 检查一次房间状态，与客户端发了什么无关，所以推进链不会
// 再被客户端的随机行为打断。
//
// 返回的 packets 恒为 nil（只挂任务，发送由 apocalypseAdvanceDue 负责）。
func (w *worldSession) apocalypseAdvanceSweep(now time.Time) []map[string]any {
	if w == nil || w.apocalypse == nil || w.activeDungeon == nil || w.legion == nil {
		return nil
	}
	run := w.apocalypse
	if !run.Entered || run.Ended || run.Rewarded {
		return nil
	}
	stage := legion.ApocalypseStageOfDungeon(w.activeDungeon.Definition.ID)
	if stage < 0 {
		return nil
	}
	// 房间里还有活怪 = 还没清关，不该推进。
	if len(w.activeDungeon.LivingMonsters()) != 0 {
		return nil
	}
	// 终点关走终局翻牌链，不在此推进（终点按**难度**取，难度1 = 第 3 关）。
	if stage >= legion.ApocalypseEndpoint(run.Choice) {
		return nil
	}
	next := stage + 1
	if p := w.apocalypseAdvancePending; p != nil && !p.done {
		return nil
	}
	if !w.armApocalypseAdvance(next, apocalypseStageAdvanceDelay) {
		return nil
	}
	return []map[string]any{{
		"kind":         "apocalypse_advance_swept",
		"character_id": w.role.ID,
		"stage":        stage,
		"next_stage":   next,
		"dungeon":      w.activeDungeon.Definition.ID,
	}}
}

// apocalypseStageDungeon returns the dungeon id of one apocalypse stage. The id
// table itself lives in internal/legion so the wire layer never invents one.
func apocalypseStageDungeon(stage int) (uint32, error) {
	if stage < 0 || stage >= len(legion.ApocalypseStageDungeons) {
		return 0, fmt.Errorf("apocalypse stage %d is not declared by the source clock", stage)
	}
	return legion.ApocalypseStageDungeons[stage], nil
}

// apocalypseStageProjection 是末世录的阶段清怪投影，与 venusStageProjection 同构：
// 当前阶段最后一只战斗怪确认死亡后，服务端把权威 N2895 推进到「已到达下一阶段」，
// 客户端据此退出副本模块并用 CMD2062 直进下一关。
//
// 抓包证据（D:\zhuabao\captures\20261008-105227）：
//
//	46.09s s2c N2895（stage=2, marks=[0,1,2], follow=0）→ 46.10s c2s CMD2062 → 下一关
//	82.83s s2c N2895（stage=3, ...）              → 85.00s c2s CMD2062
//
// 也就是**服务端先推 N2895，客户端才发 2062**。没有这份投影时客户端的阶段记录
// 停在原地，它会反复重选当前关（维纳斯当年就是这个无限进图循环）。
//
// 终点关（第 5 关）清完不只是推进：它还要挂起终局翻牌链，所以这里顺手
// MarkCompleted + armApocalypseSettlement，把流程交给 completeDungeon →
// completeApocalypseStage（与维纳斯终点关同一个分工）。
func (w *worldSession) apocalypseStageProjection(event func(map[string]any)) []outboundPacket {
	if w == nil || w.apocalypse == nil || w.activeDungeon == nil {
		return nil
	}
	stage := legion.ApocalypseStageOfDungeon(w.activeDungeon.Definition.ID)
	if stage < 0 {
		return nil
	}
	run := w.apocalypse
	if w.apocalypseStageCleared[stage] || len(w.activeDungeon.LivingMonsters()) != 0 {
		return nil
	}
	w.apocalypseStageCleared[stage] = true
	note := map[string]any{
		"kind":         "apocalypse_stage_cleared",
		"character_id": w.role.ID,
		"stage":        stage,
		"dungeon":      w.activeDungeon.Definition.ID,
		"map":          w.activeDungeon.Room.Map,
	}
	// 攻坚房间（stage0）也**必须**发权威 N2895：参考抓包 47.54s 那一帧
	// （choice=00 state=2 stage=0 follow=FFFFFFFF marks=[00 01 ff…]）就发生在
	// 这里，客户端把它当成「本关通过」的权威信号，随后自己发 CMD2062 直进第 1 关。
	//
	// 曾经这里只记事件、返回 nil —— 客户端的通关门槛因此永远不被认可，
	// 实机症状是「站在门里面没反应、门不开」。
	if stage == 0 {
		note["navigation_room"] = true
		event(note)
		// 下一关 = apocalypseStageDungeons[1]；攻坚房间**即时推进**（业主口径：
		// 「碰到门后立马进入下一关」）。
		w.armApocalypseAdvance(1, apocalypseNavigationAdvanceDelay)
		return []outboundPacket{{
			"apocalypse_navigation_cleared", 0, legion.NotiLegionInfo, w.legion.apocalypseInfo(),
		}}
	}
	next := stage + 1
	// 某关清完后：Stage 推到「已打完的下一关」（客户端据此发 CMD2062 直进），
	// marks 比它多一格（清一关多一格），与参考抓包一致：
	// 第1关清完 → stage=2 marks=[00 01 02 ff…]。
	run.PhaseCleared = next
	run.Cleared = stage
	if run.Stage < next {
		run.Stage = next
	}
	note["next_stage"] = run.Stage
	// ★ 终点关判定必须按**难度**，不能写死最后一关。
	//
	// 规格 2252-LEGIONBASICREWARD.md 的实现段原文：
	//
	//	「末世录模式消费者 140696EA0 先以虚表+80→140696D50 检查模式当前阶段是否
	//	  等于难度终点（**第一档 3，第二档 5**）」
	//
	// 也就是难度1 打到**第 3 关**就算通关（不是 5 关）。本实现此前写死
	// `len(ApocalypseStageDungeons)-1`（恒为 5），实机后果是难度1 与难度2
	// 流程完全一样（业主 2026-10-08：「测试难度1，发现流程和难度2一模一样，
	// 正常情况难度1打完第 3 个 BOSS 算通关」）。
	if stage >= legion.ApocalypseEndpoint(run.Choice) {
		// 终点关：交给终端结算分支（翻牌链），这里只标记完成。
		w.activeDungeon.MarkCompleted()
		note["terminal"] = true
		note["endpoint"] = legion.ApocalypseEndpoint(run.Choice)
		event(note)
		return nil
	}
	event(note)
	w.armApocalypseAdvance(next, apocalypseStageAdvanceDelay)
	return []outboundPacket{{
		"apocalypse_combat_phase_advanced", 0, legion.NotiLegionInfo, w.legion.apocalypseInfo(),
	}}
}

// completeApocalypseStage 是末世录阶段本的清关钩子（由 completeDungeon 调用）。
//
// 职责划分：真正的**权威推进帧 N2895** 由 apocalypseStageProjection 在清怪时发出
// （含攻坚房间，见那里的注释）；本函数只负责「终点关挂起终局翻牌链」。
//
//  1. 攻坚房间（stage0）与中途关：清空即投影 N2895，客户端随后自己发
//     CMD2062 直进下一关（参考抓包 47.56s / 72.14s），这里不结算。
//  2. **难度终点关**（难度1 = 第 3 关、难度2 = 第 5 关，规格 2252 的
//     「第一档 3，第二档 5」）清空：挂起终局翻牌链（N14→N2895→N2252→N2253），
//     由连接定时器在 apocalypseSettlementDelay 后发出。
//
// ⚠️ 终点必须按**难度**取 `ApocalypseEndpoint(run.Choice)`：此前写死
// `len(ApocalypseStageDungeons)-1`（恒为 5），导致难度1 也要打满 5 关
// （业主 2026-10-08：「流程和难度2一模一样，正常情况难度1打完第 3 个 BOSS 算通关」）。
func (w *worldSession) completeApocalypseStage() ([]outboundPacket, error) {
	run := w.apocalypse
	if run == nil || w.activeDungeon == nil {
		return nil, nil
	}
	stage := legion.ApocalypseStageOfDungeon(w.activeDungeon.Definition.ID)
	if stage < 0 {
		return nil, nil
	}
	if stage < legion.ApocalypseEndpoint(run.Choice) {
		// 攻坚房间与中途关：清空只意味着可以走门/等客户端直进，不结算。
		return nil, nil
	}
	w.armApocalypseSettlement(legion.CmdRewardEnd, "apocalypse_terminal_settlement", apocalypseSettlementDelay)
	return nil, nil
}
