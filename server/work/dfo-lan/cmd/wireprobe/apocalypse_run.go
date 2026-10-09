package main

import (
	"dfolan/internal/game/protocol"
	"dfolan/internal/legion"
	"fmt"
	"time"
)

// 末世录（内容 107 / 频道 Type 119）玩法层。
//
// 军团信封（CMD2043/2354/2045/2355/2044/2046）的解码与城镇侧应答在
// legion_flow.go；本文件补的是「进图以后」那一段：难度确认 → 攻坚房间 →
// 六个阶段的推进 → 角色分配 → 清关结算。
//
// 证据链（两份，缺一不可）：
//  1. 客户端抓包 D:\zhuabao\captures\20261008-105227：难度1/难度2 各一场全清，
//     逐帧给出 N2895 的字段偏移、CMD2062 的目标副本号、N27/N28/N29 进图帧序。
//  2. US-Local 服务端会话
//     D:\115US-Local\server\work\dfo-lan\runtime\roles_..._20261008_105231_329884_next37\events.jsonl
//     给出同一场的服务端视角（apocalypse_navigation_prepared / _gate_completed /
//     _first_combat_prepared / _boss_death_phase_advanced / _pickup_window_* /
//     _basic_clear_reward / _full_clear_reward_committed）。
//
// US-Local 现役 exe 里这段实现的源码不在这台机器上（只有 exe 与 events），
// 所以本文件按上面两份证据重写，不复制任何未知来源的代码。

const (
	// apocalypseDirectMoveID is ENUM_CMDPACKET_DUNGEON_DIRECT_MOVE: the packet
	// the client uses to load the waiting room and every later phase.
	apocalypseDirectMoveID uint16 = 2062
	// apocalypseNavigationMap is the waiting-room map id the client loads for
	// the navigation dungeon. The entry plan derives it from the PVF dungeon
	// record; this constant only documents what the capture showed
	// (dungeon 100005112 → map 100016816).
	apocalypseNavigationMap = 100016816
	// apocalypseEntryCtxTimeout bounds the PVF lookups one stage entry performs.
	apocalypseEntryCtxTimeout = 10 * time.Second
)

// apocalypseStandbyPartyHandle 实现末世录待机区（频道 Type 119）队伍对话框：
// CMD12 建队、CMD13 离队。请求与维纳斯/伊斯待机区同构，只有队伍类型字节
// 不同（0x26 = 38 = protocol.ApocalypsePartyMode115）；应答照黑鸦/伊斯先例：
// 队长资料两个 op=2 先行 + 单帧 NOTI9。
//
// 边界与其它军团一致：单人 bootstrap 队伍（Party==1 归一化 65535），
// 建队后 arm soloPartyReady。
//
// 注意（抓包 20261008-105227 实证）：玩家在**城镇**用的是通用建队对话框，
// 那条 CMD12 由通用队伍链路应答，不进这里；本处理器只在 Type 119 频道
// （待机区）接管，用于补上遗留事项里「末世录待机区点创建队伍无反应」。
func (w *worldSession) apocalypseStandbyPartyHandle(id uint16, p []byte) (bool, []outboundPacket, error) {
	if w == nil || w.channelType != apocalypseChannelType || w.role.ID == 0 || (id != 12 && id != 13) {
		return false, nil, nil
	}
	fail := func(err error) (bool, []outboundPacket, error) { return true, nil, err }
	if w.characters == nil {
		return fail(fmt.Errorf("末世录待机区角色服务不可用"))
	}
	if id == 13 {
		if len(p) != 0 && len(p) != 8 {
			return fail(fmt.Errorf("末世录离队请求长度无效"))
		}
		for _, b := range p {
			if b != 0 {
				return fail(fmt.Errorf("不支持的末世录离队选项"))
			}
		}
		if w.activeDungeon != nil {
			return fail(fmt.Errorf("请先返回待机区再退出末世录队伍"))
		}
		w.soloPartyReady = false
		return true, []outboundPacket{
			{"末世录队伍解散", 0, 9, protocol.BlackPurgatoryPartyGone(w.characters.ChannelContext)},
		}, nil
	}
	name, err := protocol.DecodeApocalypseStandbyParty(p)
	if err != nil {
		return fail(err)
	}
	party, err := protocol.ApocalypseStandbyPartyReply(name, w.role.WireID, w.characters.ChannelContext)
	if err != nil {
		return fail(err)
	}
	basic, err := w.characters.EntryBasicProbe(w.role, w.characters.ChannelContext)
	if err != nil {
		return fail(err)
	}
	detail, err := w.characters.EntryAddition(w.role)
	if err != nil {
		return fail(err)
	}
	w.soloPartyReady = true
	packets := []outboundPacket{
		{"末世录队长资料", 0, 2, basic},
		{"末世录队长详细资料", 0, 2, detail},
		{"末世录待机区队伍创建", 0, 9, party},
	}
	// N2254 入场角色信息：抓包在进频道/进攻坚房间时由服务端发出，队伍槽位与
	// 角色选择界面读它。建队成功这一帧一并补上（缺失时槽位列表为空）。
	if entry, entryErr := legion.ApocalypseEntryCharacterInfo(); entryErr == nil {
		packets = append(packets, outboundPacket{
			Name: "apocalypse_entry_character_info", Kind: 0, ID: legion.NotiEntryCharacterInfo, Payload: entry,
		})
	}
	return true, packets, nil
}

// apocalypseOperationIndex maps the client's CMD2354 difficulty key onto the
// operation number recorded by the source table (choice+1; see
// legion.PlanForChoice and TestApocalypseChoiceUsesZeroBasedNativeKey).
func apocalypseOperationIndex(choice byte) int {
	if choice == 0xff {
		return 0
	}
	return int(choice) + 1
}

// handleApocalypse routes the legion family for the 末世录 content.
//
// handled=false means "not this layer's packet": CMD2043, CMD2354 and CMD2045
// keep using the generic handler in legion_flow.go so the family keeps one
// start/operation path, while everything the run tracker owns (dungeon-side
// role assignment and the end-of-run teardown) is answered here.
func (s *legionSession) handleApocalypse(w *worldSession, p []byte, id uint16, arrivingSide string) (legionResult, bool, error) {
	switch id {
	case legion.CmdStart:
		// A fresh channel entry always reopens the run at the waiting state, so a
		// leftover difficulty/stage from the previous run cannot leak into it.
		run := s.apocalypseRun()
		// Reset 会同时换一个新的 run id（终局奖励的幂等事件键以它为基）。
		run.Reset()
		if s.world != nil {
			s.world.apocalypsePending = nil
			// 清怪投影的去重表必须一起清：否则第二局的攻坚房间/已打过的关
			// 永远不会再推权威 N2895，症状与第一局「门不开」一模一样。
			s.world.apocalypseStageCleared = [6]bool{}
			s.world.apocalypseAdvancePending = nil
		}
		return legionResult{}, false, nil
	case legion.CmdOperationSelect:
		result, err := s.apocalypseOperation(w, p)
		return result, true, err
	case legion.CmdEnterDungeon:
		result, err := s.apocalypseEnter(w, p)
		return result, true, err
	case legion.CmdRoleSelect:
		result, err := s.apocalypseRole(w, p)
		return result, true, err
	case legion.CmdFail:
		request, err := legion.DecodeFail(p)
		if err != nil {
			return legionResult{}, true, err
		}
		if s.session == nil {
			return legionResult{}, true, fmt.Errorf("legion fail before entering the channel (no CMD2043 yet)")
		}
		s.session.Fail(request.Argument)
		run := s.apocalypseRun()
		operation := apocalypseOperationIndex(run.Choice)
		run.Reset()
		return resultWithEvents(legionResult{Packets: []outboundPacket{{
			Name:    "apocalypse_abandon_ack",
			Kind:    1,
			ID:      legion.CmdFail,
			Payload: legion.FailAck(),
		}}}, map[string]any{
			"kind":          "legion_run_failed",
			"character_id":  w.role.ID,
			"argument":      request.Argument,
			"arriving_side": arrivingSide,
			"operation":     operation,
			"family":        "apocalypse",
		}), true, nil
	case legion.CmdRewardEnd:
		request, err := legion.DecodeRewardEnd(p)
		if err != nil {
			return legionResult{}, true, err
		}
		if s.session == nil {
			return legionResult{}, true, fmt.Errorf("legion reward end before entering the channel (no CMD2043 yet)")
		}
		run := s.apocalypseRun()
		note := map[string]any{
			"kind":           "apocalypse_reward_end_ack",
			"character_id":   w.role.ID,
			"first":          request.First,
			"second":         request.Second,
			"flag":           request.Flag,
			"arriving_side":  arrivingSide,
			"operation":      apocalypseOperationIndex(run.Choice),
			"phases_cleared": run.PhaseCleared,
			"stage":          run.Stage,
			"run":            run.ID,
		}
		if s.plan != nil {
			note["table_reward_label"] = s.plan.RewardLabel
			note["table_reward_values"] = s.plan.RewardValues
			note["table_total_seconds"] = s.plan.TotalSeconds
		}
		// 终局奖励链：客户端关掉翻牌界面就是结算信号。若之前已经通过清关
		// 延迟任务发过（Rewarded），这里只回 ACK；否则现在补发一次，保证
		// 「客户端点了结算但服务端没发奖」不会发生。
		var rewardPackets []outboundPacket
		if !run.Rewarded {
			grant, gErr := w.apocalypseTerminalReward(legion.CmdRewardEnd, "apocalypse_reward_end")
			if gErr != nil {
				note["reward_chain"] = "refused"
				note["reward_error"] = gErr.Error()
			} else {
				rewardPackets = grant.Plan
				note["reward_chain"] = "delivered"
				for _, e := range grant.Events {
					note["reward_"+(fmt.Sprint(e["kind"]))] = e
				}
			}
		} else {
			note["reward_chain"] = "already_delivered"
		}
		s.session.EndReward()
		s.plan = nil
		// ★ 终局通关视频触发帧（State3）挂在这里 —— 客户端**一定会发** CMD2046
		// （关掉结算窗就是信号），而规格原文也正好说它「Sent with the terminal
		// CMD2046 ACK」：
		//
		//	维纳斯 `VenusFinalInfo`：「the terminal state that **triggers the clear
		//	movie** … family convention with the ispins N2255 "final" (State 3,
		//	Outcome 1) — Sent with the terminal CMD2046 ACK — **the client then plays
		//	the clear movie**」。
		//
		// 为什么不挂在翻牌处理器上（`cardPick` / `autoPickSettlementCard`）：
		// 军团翻牌界面**不走 CMD69/70/71**（客户端收到 N2252 后自己
		// `Open IRDPopupWindow Type : 642`），所以 `cardLayoutSent` 永远为 false，
		// `autoPickSettlementCard` 直接 return、`cardPick` 也不会被调用 ——
		// 实机 2026-10-08 19:47 那局 `apocalypse_clear_movie_started` **从未出现**，
		// 通关视频自然也不出。
		if moviePackets, movieNotes := w.readyApocalypseClearMovie(); len(moviePackets) > 0 {
			rewardPackets = append(rewardPackets, moviePackets...)
			for _, e := range movieNotes {
				note["movie_"+(fmt.Sprint(e["kind"]))] = e
			}
		}
		run.Ended = true
		// Ended 标记保留到下一次 CMD2043（Reset 会清掉它），这样结算后的
		// 终局态 N2895 仍然显示 State0；其余字段回到未进图的样子。
		run.Stage = 0
		run.PhaseCleared = 0
		run.Cleared = 0
		run.RoleSet = false
		run.Entered = false
		w.apocalypsePending = nil
		w.apocalypseStageCleared = [6]bool{}
		w.apocalypseAdvancePending = nil
		result := legionResult{Packets: append(rewardPackets, outboundPacket{
			Name:    "apocalypse_reward_end_ack",
			Kind:    1,
			ID:      legion.CmdRewardEnd,
			Payload: legion.RewardEndAck(),
		})}
		return resultWithEvents(result, note), true, nil
	default:
		return legionResult{}, false, nil
	}
}

// ApocalypseDirectMove answers a CMD2062 the client sends while an apocalypse
// run is open. It is the entry point the dungeon dispatcher calls before its
// generic "direct move without an active dungeon" guard (see
// apocalypseDirectMove).
func (s *legionSession) ApocalypseDirectMove(w *worldSession, dungeonID uint32) (legionResult, error) {
	if s == nil || !s.apocalypseRan() {
		return legionResult{}, fmt.Errorf("no apocalypse run on this connection")
	}
	return s.apocalypseDirectMove(w, dungeonID)
}

// apocalypseRun returns this connection's apocalypse run, creating it lazily.
// The run is reset on every CMD2043 so a fresh channel entry always starts from
// the waiting state.
//
// The pointer is mirrored onto the connection's worldSession, because the
// CMD2062 direct move is dispatched by the dungeon layer, which has no
// legionSession of its own (see directMoveDungeon).
func (s *legionSession) apocalypseRun() *legion.ApocalypseRunState {
	if s.apocalypse == nil {
		s.apocalypse = legion.NewApocalypseRunState()
		if s.world != nil {
			s.world.apocalypse = s.apocalypse
		}
	}
	return s.apocalypse
}

// apocalypseInfo builds the NOTI2895 body for the current run state.
//
// Field sources, from the 2026-10-08 capture (all 22 LEGION_INFO bodies of the
// two difficulty clears, aligned and diffed). Payload offsets include the u16
// content prefix, so the body offset is the payload offset minus 2:
//
//	@4  choice      0xff until CMD2354 action2 confirms a difficulty
//	@5  state       2 from the channel entry, 0 again after CMD2046
//	@9  following   **FFFFFFFF for the whole run** (a sentinel here, never a
//	                stage number); only the teardown frame leaves it alone too
//	@13 stage       只由 **CMD2062 载入某一关**推进：0 在城镇、
//	                CMD2045 确认作战后仍是 0、载入攻坚房间后 1、载入第 N 关后 N
//	@29+12i marks   slot i = i 当 i <= 已到达阶段，否则 0xff。已到达阶段 =
//	                PhaseCleared+1（CMD2045 之后就算「已到达 1」）
//	@109 role count 1 after the first CMD2355, 0 again after the run ends
//
// 字段语义是 2026-10-08 用 tools/apocalypse-port/infofields.py 把参考抓包逐帧
// 重新解出来校正的。此前实现把 @9 在进图后写成 0（应当是 FFFFFFFF 哨兵），
// 客户端因此**不发 CMD2062 去载入攻坚房间** —— 表现就是「选完难度进不去副本」。
func (s *legionSession) apocalypseInfo() []byte {
	run := s.apocalypseRun()
	state := legion.ApocalypseWaitingInfo()
	state.OperationChoice = run.Choice
	// ★ 唯一的刻度换算点：run.Stage 是**副本表下标**（0=攻坚房、1=第1关…），
	// 发布出去的 @13 是「下标 + 1」（抓包锚点：攻坚房载入后 @13=1@41.12s、
	// 第1关 @13=2@47.54s、第2关 @13=3@73.59s）。客户端把收到的 @13 原样回送成
	// 续关请求 CMD2045 @17，所以 ContinueStage() 正好等于 Stage。
	state.Stage = uint32(run.Stage)
	// marks 由「已到达的最高槽号」派生（见 legion.ApocalypseMarksFor 的注释）：
	// 未进图 = -1（全 0xff）；CMD2045 确认作战 = 槽 0（[00 01 …]）；
	// 载入第 N 关 / 清掉第 N 关 = 槽 N。
	state.StageMarks = legion.ApocalypseMarksFor(run.MarkCount())
	// 目标索引标记（载荷 @128+4*i）：客户端**重建「已经打过哪几关」**的唯一依据
	// （规格 G0382：14069A750 检验它非 0 才认这一关已打）。只发 Stage/Marks 而
	// 不发这一段时，撤退回城后客户端仍认为一关都没打，继续时发
	// CMD2045(stage 0) 被服务端按保存阶段拒绝 —— 实机 2026-10-08 16:24 的
	// `apocalypse resume stage 3 does not match the saved stage 0` 就是这个成因。
	//
	// 已清掉的关（1..run.Cleared）标记置 1；攻坚房间（slot0）是导航，不算战斗关，
	// 保持 0。
	for i := 1; i <= run.Cleared && i < 6; i++ {
		state.TargetMarks[i] = 1
	}
	if run.Ended {
		state.State = 0
	} else {
		state.State = 2
	}
	// @9 是哨兵，不是关卡号；整场保持 FFFFFFFF（Reset 的初值就是它）。
	state.Following = ^uint32(0)
	if run.RoleSet && !run.Ended {
		state.RoleCount = 1
	}
	return legion.ApocalypseInfo(state)
}

// apocalypseOperationPlan validates the difficulty the client confirmed through
// CMD2354 against the compiled apocalypse table. The client keys the table with
// choice+1 (legion.PlanForChoice), so an undeclared choice is a refusal rather
// than a guess.
func (s *legionSession) apocalypseOperationPlan() (*legion.RunPlan, error) {
	run := s.apocalypseRun()
	if run.Choice == 0xff {
		return nil, fmt.Errorf("apocalypse operation refused: no difficulty was confirmed (no CMD2354 action2)")
	}
	if s.catalog == nil || s.clock == nil {
		return nil, nil
	}
	plan, err := legion.PlanForChoice(s.catalog, s.clock, run.Choice)
	if err != nil {
		return nil, fmt.Errorf("apocalypse operation refused: %w", err)
	}
	return plan, nil
}

// apocalypseOperation handles CMD2354 for the apocalypse content.
//
// The capture shows two calls per run: action 1 opens/refreshes the operation
// screen (echoed), action 2 confirms the difficulty and is answered with the
// selected difficulty state (NOTI2895, choice = key) before the plain ack.
func (s *legionSession) apocalypseOperation(w *worldSession, p []byte) (legionResult, error) {
	request, err := legion.DecodeOperationSelect(p)
	if err != nil {
		return legionResult{}, err
	}
	if request.Channel != legion.OperationChannelCode {
		return legionResult{}, fmt.Errorf("legion operation channel %d, want %d", request.Channel, legion.OperationChannelCode)
	}
	run := s.apocalypseRun()
	// 选择难度窗的倒计时截止值（操作记录 byte@6..9 = 正文 @9..12）。
	//
	// 2026-10-08 修正：**必须下发**。此前按「时基未校准」暂不下发，实机表现是
	// 难度框倒计时恒为 **0**、**永不自动关闭** —— 框内又没有关闭按钮，不选难度
	// 就整场卡死（业主报）。维纳斯的同类框正常，差别就在它的 ACK 带截止值：
	//
	//	deadline := uint32(time.Now().Unix()) + legion.VenusSelectionSeconds
	//	legion.VenusOperationAck(1, false, deadline, 0)
	//
	// 维纳斯注释还写明「客户端自己对 0 不做任何事」，与末世录的现象完全吻合。
	// 时基与维纳斯同口径取绝对 UNIX 秒；规格仍标注该项**待实机校准**，
	// 若倒计时显示异常，第一嫌疑即时基。
	//
	// 另：此前一次失败是把截止值写到正文 @7..10（整体提前 2B），压掉了真正的
	// 关闭位 byte@5（正文 @8）⇒ 点 Open 完全没反应。**不是客户端不认该字段。**
	// ★ 倒计时截止值：**下发**（业主 2026-10-09 明确要求把倒计时加回来）。
	//
	// 演进史（三段实机，别再翻回去）：
	//  1. 最初不下发（该格 = 0）⇒ 业主报「难度框倒计时恒为 0、永不自动关窗」；
	//  2. 于是写入绝对 UNIX 秒 ⇒ 倒计时 15→0 与自动关窗都正常 ✓；
	//  3. 但同一时期地图内出现三症状（复活币数字 99、难度2 出现复活提示、
	//     药水被禁），当时**误判**为该格所致，A' 版把它改回 0 —— 实测
	//     **数字仍是 99**，⇒ 该格无辜。真凶是第4/5轮在房间里补发的两帧 N2895
	//     （已删除，见 apocalypse_stage.go / apocalypse_direct_move 的注释）。
	//  4. 补发删除后（A''）数字恢复 8 ✓、药水 ✓、续关 ✓，因此本版把截止值加回来。
	//
	// 时基：绝对 UNIX 秒（与维纳斯 VenusOperationAck 同口径）。规格仍标注
	// 「时基待实机校准」；若日后地图内再出现与作战记录相关的异常，
	// 该格是第一嫌疑（客户端会把整条记录存进当前作战结构体）。
	var deadline uint32
	if request.Action == 1 {
		deadline = uint32(time.Now().Unix()) + legion.ApocalypseSelectionSeconds
		run.WindowDeadline = time.Unix(int64(deadline), 0)
		// 服务端侧兜底：开窗后 15 秒没确认难度就推关闭态（业主 2026-10-08）。
		// 客户端的自动关窗由上面的截止值驱动；这一条是并列的保险，不替代它。
		s.world.armApocalypseSelectWindow(time.Now())
	} else {
		// action2 = 已确认难度：窗已在处理，不再需要计时（维纳斯同款传 0）。
		run.WindowDeadline = time.Time{}
		s.world.cancelApocalypseSelectWindow()
	}
	result := legionResult{Packets: []outboundPacket{{
		Name:    "legion_operation_ack",
		Kind:    1,
		ID:      legion.CmdOperationSelect,
		Payload: legion.ApocalypseOperationAck(request.Action, deadline),
	}}}
	notes := []map[string]any{{
		"kind":         "legion_operation_recorded",
		"character_id": w.role.ID,
		"action":       request.Action,
		"auxiliary":    request.Auxiliary,
		"choice":       run.Choice,
		"operation":    apocalypseOperationIndex(run.Choice),
	}}
	// 抓包：动作 2 携带被选难度的 CTP 键（0x00 = 难度1、0x01 = 难度2），
	// 动作 1 的 @17 是界面刷新用的旧值（0x0d / 0xff），不能当难度。
	if request.Action == 2 {
		if request.Auxiliary == 0xff {
			return legionResult{}, fmt.Errorf("apocalypse difficulty refused: CMD2354 action2 carried no key (0xff)")
		}
		run.Choice = request.Auxiliary
		plan, err := s.apocalypseOperationPlan()
		if err != nil {
			return legionResult{}, err
		}
		// 难度确认后客户端先收到难度态 NOTI2895，再收 2354 应答（抓包 39.62s）。
		result.Packets = append([]outboundPacket{{
			Name:    "legion_info",
			Kind:    0,
			ID:      legion.NotiLegionInfo,
			Payload: s.apocalypseInfo(),
		}}, result.Packets...)
		note := map[string]any{
			"kind":         "legion_difficulty_selected",
			"character_id": w.role.ID,
			"choice":       run.Choice,
			"operation":    apocalypseOperationIndex(run.Choice),
		}
		if plan != nil {
			note["phase_order"] = plan.PhaseOrder
			note["phase_seconds"] = plan.PhaseSeconds
			note["reward_label"] = plan.RewardLabel
		}
		notes = append(notes, note)
	}
	result.Events = notes
	return result, nil
}

// apocalypseEnter handles CMD2045 (LEGION_ENTER_DUNGEON) for the apocalypse
// content: the client confirms an operation and asks the server to start it.
//
// 参考抓包 D:\zhuabao\captures\20261008-105227 的 41.10–41.38s 是**决定性证据**：
//
//	41.10s c2s CMD2045
//	41.11s s2c CMD2045 ACK
//	41.12s s2c N2895（初始目标 marks=[00 01 ff…]）
//	41.12s s2c N2 ×2（角色资料）
//	41.13s s2c N14 ×2、N13（物品台账）
//	41.14s s2c N2839（誓约系统）、N3（角色状态→副本态）
//	41.15s s2c N27（选图上下文）、N28（副本信息）、N29（START_MAP）
//	41.36s c2s CMD37（加载完成）   ← 客户端只是回一个加载完成，**没有 CMD2062**
//
// 也就是说**进图由 CMD2045 一次性完成**，攻坚房间是服务端在这里直接载入的。
// 此前实现以为要等客户端的 CMD2062，于是 CMD2045 只回 ACK + 状态，
// 客户端收不到 N27/N28/N29 就永远停在城镇 —— 实机症状正是「点完难度进不去副本」。
//
// CMD2062 只在**后续阶段之间**使用（客户端清完一关后直进下一关），
// 那条路径由 apocalypseDirectMove 处理。
func (s *legionSession) apocalypseEnter(w *worldSession, p []byte) (legionResult, error) {
	request, err := legion.DecodeEnterDungeon(p)
	if err != nil {
		return legionResult{}, err
	}
	if request.Channel != legion.OperationChannelCode {
		return legionResult{}, fmt.Errorf("legion enter channel %d, want %d", request.Channel, legion.OperationChannelCode)
	}
	run := s.apocalypseRun()
	if run.Choice == 0xff {
		return legionResult{}, fmt.Errorf("apocalypse enter before a difficulty was confirmed (no CMD2354 action2)")
	}
	// ★ 续关的阶段授权（规格 D:\115US-001\moshilu 的 2045-LEGIONENTERDUNGEON.md
	// G0452 / 2895-LEGIONINFO末世录状态.md G0452）：
	//
	//	「已暂停的同一作战走ResumeApocalypse：阶段须等于保存值，全员回城写入完成
	//	 才重建当前未完成Boss，保留RunID、难度与已清路线。」
	//	「NPC继续门为State2，对应C2045(content107, 保存阶段)。」
	//
	// 刻度：run.Stage = 已载入的**副本表下标**（攻坚房 0、第1关 1……）；
	// 发布的 NOTI2895 @13 与客户端的续关请求 CMD2045 @17 都是 **run.Stage + 1**
	// （见 ContinueStage 的注释与参考锚点）。换算只在这里做一次。
	//
	// ★ A4（2026-10-09 03:0x 实机证据）：**续关一律以服务端保存的阶段为准**，
	// 不再对客户端的 @17 做任何 ±1 分支。当前所在房间 = `want - 1`。
	//
	// 为什么删掉原来的 `case want, want-1`：客户端 @17 来自它**最后收到的
	// NOTI2895 @13**，而「房间里补发 N2895」已按 A2 删除（补发会把客户端的作战
	// 状态冲掉，见 apocalypse_stage.go）⇒ 客户端手上**经常是旧值**。旧代码对
	// 陈旧一格的值走 `case want-1`，算 `requested-1 = want-2` ⇒ **落早一关**。
	//
	// 实机证据（会话 roles_..._20261009_030620_232753_next37，同一条 run 内）：
	//
	//	19:10:43  服务端 run.Stage=3、客户端报 2 → 旧代码 ResumeStage=1
	//	          ⇒ 载入 100004995（第1关）= 业主报的「回到前一关」
	//	19:11:06  服务端 3、客户端 3      → 载入 100005057（第2关）✓
	//
	// 两种结果并存 ⇒ 业主报的「不一定回到刚退出那一关，可能前一关/下一关」。
	// 本场只有这一条 run（Suspended = 同一作战已暂停），服务端才是权威。
	// 客户端报来的值只用于**诊断**（不符即记 resume_stage_stale）。
	var resumeStale bool
	if run.Suspended {
		want := run.ContinueStage()
		if int(request.Stage) != want {
			resumeStale = true
		}
		run.ResumeStage = want - 1
		if run.ResumeStage < 0 {
			run.ResumeStage = 0
		}
	} else if request.Stage != 0 {
		return legionResult{}, fmt.Errorf("apocalypse first entry carries stage %d, want 0", request.Stage)
	} else {
		run.ResumeStage = 0
	}
	plan, err := s.apocalypseOperationPlan()
	if err != nil {
		return legionResult{}, err
	}
	// 直进攻坚房间：复用 CMD2062 那条路径的载入逻辑（dungeon.Select +
	// dungeonEntryPlanImpl + N3/N27 插入），只把应答换成 CMD2045 的族信封 ACK。
	//
	// 载入失败时**不拒绝应答**：客户端在等 CMD2045 的结果码，回拒绝只会让它
	// 弹一句无法定位的错误文案；这里照常回 ACK + 状态帧，并把失败原因记成
	// apocalypse_stage_entry_failed（缺少 N27/N28/N29 是立即可见的症状）。
	// 载入目标：首次入场 = 攻坚房间（下标 0）；撤退续关 = **当前该打的那一关**
	// （run.ResumeStage）。G0452 的「重建当前未完成Boss」指的就是后者。
	entryDungeon := legion.ApocalypseNavigationDungeon
	if run.ResumeStage > 0 && run.ResumeStage < len(legion.ApocalypseStageDungeons) {
		entryDungeon = legion.ApocalypseStageDungeons[run.ResumeStage]
	}
	sel := protocol.DungeonSelection{ID: entryDungeon, Difficulty: 0, Party: 65535}
	sess, frames, entryErr := w.apocalypseStageEntry(sel)
	run.Entered = true
	run.Suspended = false
	// ★ 每一次 CMD2045 进关都**重置该关倒计时**（业主 2026-10-08：
	// 「不管是点击撤退、死亡后撤退、到时间撤退，重进副本都需要重置时间」）。
	// 阶段内的换房/重载不重置，那由 apocalypseStageTimer 的「零值才冻结」保证。
	w.apocalypseResetStageClock(run.ResumeStage, time.Now())
	// 本局第一次进图时冻结开始时刻（N2252 的 @7760 通关耗时以它为基准）。
	if run.RunStartedAt.IsZero() {
		run.RunStartedAt = time.Now()
	}
	run.RoleSet = false
	// ★ Stage 语义（规格 2895-LEGIONINFO末世录状态.md G0452）：
	// 「不能用导航门已开把暂停 Stage0 投影成 1」—— 导航门开**不算**阶段推进。
	// Stage 只由真实清关推进（apocalypseStageProjection 里 run.Stage = next），
	// 所以这里不再把它置 1。续关时 Stage 也保持保存值不变。
	if run.PhaseCleared < 1 {
		run.PhaseCleared = 1
	}
	s.plan = plan

	packets := []outboundPacket{
		{
			Name: "apocalypse_enter_ack",
			// 客户端只读第一个 dword 当结果码（0=接受，252/380=两种失败文案）。
			Kind:    1,
			ID:      legion.CmdEnterDungeon,
			Payload: legion.EnterDungeonAck(),
		},
		{
			Name:    "apocalypse_initial_destinations",
			Kind:    0,
			ID:      legion.NotiLegionInfo,
			Payload: s.apocalypseInfo(),
		},
	}
	if entryErr == nil {
		for i, pkt := range frames {
			if i == 0 {
				// 进图序列自带的那个 select_ack(16) 在这里没有意义：CMD2045 的
				// 应答已经单独发过（参考抓包 41.11s 只有一帧 ACK）。
				continue
			}
			packets = append(packets, pkt)
		}
	}
	// N2254 不在 CMD2045 之后发：参考抓包整场只在进频道（2.38s）与
	// 进攻坚房间前（10.36s）各发过一次，41.1x 段没有它。

	note := map[string]any{
		"kind":          "apocalypse_navigation_prepared",
		"character_id":  w.role.ID,
		"operation":     apocalypseOperationIndex(run.Choice),
		"choice":        run.Choice,
		"stage":         run.Stage,
		"request_stage": request.Stage,
		"dungeon":       sel.ID,
		"ack":           "cmd2045-enter",
	}
	if resumeStale {
		// 客户端报的阶段是旧值、服务端按保存阶段续关。这条事件留着用于收敛
		// 「还有哪些路径漏发 NOTI2895」。
		note["resume_stage_stale"] = true
		note["resume_stage"] = run.ResumeStage
	}
	if entryErr != nil {
		note["stage_entry_failed"] = entryErr.Error()
		note["kind"] = "apocalypse_stage_entry_failed"
	} else {
		note["map"] = sess.Room.Map
		note["monsters"] = len(sess.Monsters)
	}
	if plan != nil {
		note["phase_order"] = plan.PhaseOrder
		note["phase_seconds"] = plan.PhaseSeconds
		note["gate_flow"] = plan.GateFlow
	}
	return legionResult{Packets: packets, Events: []map[string]any{note}}, nil
}

// apocalypseCoinLimit 返回本次作战**每关**允许的复活币次数。
//
// 真源：PVF `apocalypse.ctp` 的 `[operation data set]` → `[allow coin]`
// （导入 internal/catalog/apocalypse_import.go 的 colAllowCoin；
//
//	二进 internal/legion/phase.go 的 RunPlan.AllowCoin / AllowCoinConfigured）。
//
// 实测只有作战①（难度1）带该列、值 `-1, 8`，作战②/③/④ 都没有该列。
// 业主 2026-10-08 定调语义：**最后一个数 = 每关上限；无该列 = 禁止复活**。
//
// 返回 (limit, applicable)：
//   - applicable=false：本连接当前不在末世录关卡里，调用方按「不适用」处理；
//   - applicable=true 且 limit<=0：**在末世录关卡里但禁止复活**（难度2/3/4，
//     或还没确认作战、plan 尚未建立）。
func (w *worldSession) apocalypseCoinLimit() (int, bool) {
	if w == nil || w.activeDungeon == nil {
		return 0, false
	}
	return w.apocalypseOperationCoinLimit(w.activeDungeon.Definition.ID)
}

// apocalypseOperationCoinLimit 是上面的**按副本号**版本。
//
// 为什么需要两个入口：进图帧序（N28 / N29 / N1584）是在 `activeDungeon` 建立
// **之前**按 `sel.ID` 组装的，那时只能凭副本号判断，读不到 activeDungeon。
func (w *worldSession) apocalypseOperationCoinLimit(stageDungeonID uint32) (int, bool) {
	if w == nil || w.legion == nil || w.apocalypse == nil {
		return 0, false
	}
	if !legion.IsApocalypseStageDungeon(stageDungeonID) {
		return 0, false
	}
	plan := w.legion.plan
	if plan == nil || !plan.AllowCoinConfigured || len(plan.AllowCoin) == 0 {
		return 0, true
	}
	limit := int(plan.AllowCoin[len(plan.AllowCoin)-1])
	if limit < 0 {
		limit = 0
	}
	return limit, true
}

// apocalypseCheckCoinBudget 是末世录的复活币闸门（照巴卡尔 CheckCoinBudget 同型）。
// 返回的 applicable 供调用方决定「复活成功后要不要扣次数」。
func (w *worldSession) apocalypseCheckCoinBudget() (bool, error) {
	limit, applicable := w.apocalypseCoinLimit()
	if !applicable {
		return false, nil
	}
	run := w.apocalypse
	if limit <= 0 {
		return true, fmt.Errorf("末世录：本次作战（难度 %d）未声明 [allow coin]，禁止使用复活币（已用 %d 次）",
			apocalypseOperationIndex(run.Choice), run.RevivesUsedThisStage)
	}
	if run.RevivesUsedThisStage >= limit {
		return true, fmt.Errorf("末世录：本关复活币已用尽（%d/%d），进入下一关后重置",
			run.RevivesUsedThisStage, limit)
	}
	return true, nil
}

// apocalypseSpendCoinBudget 记一次本关的复活币消耗（与 CheckCoinBudget 配对）。
func (w *worldSession) apocalypseSpendCoinBudget() {
	if w == nil || w.apocalypse == nil {
		return
	}
	w.apocalypse.RevivesUsedThisStage++
}

// apocalypseRole handles CMD2355 (APOCALYPSE_ROLE_SELECT). The capture shows the
// client announcing the same role twice right after the waiting room loads; the
// packet is edge-triggered on the client side (it only sends when the value
// changed), so recording it twice is harmless.
//
// The value at @13 is always 1 in both difficulty clears while @17 carries the
// 107 content tag, which matches legion.DecodeRoleSelect's layout.
func (s *legionSession) apocalypseRole(w *worldSession, p []byte) (legionResult, error) {
	request, err := legion.DecodeRoleSelect(p)
	if err != nil {
		return legionResult{}, err
	}
	if request.Channel != legion.OperationChannelCode {
		return legionResult{}, fmt.Errorf("legion role channel %d, want %d", request.Channel, legion.OperationChannelCode)
	}
	run := s.apocalypseRun()
	if !run.Entered {
		return legionResult{}, fmt.Errorf("apocalypse role before the run was entered (no CMD2045 yet)")
	}
	if request.Role == 0 {
		return legionResult{}, fmt.Errorf("apocalypse role 0 is not a declared source role")
	}
	run.Role = request.Role
	run.RoleSet = true
	return legionResult{
		Packets: []outboundPacket{
			{
				Name:    "apocalypse_role_state",
				Kind:    0,
				ID:      legion.NotiLegionInfo,
				Payload: s.apocalypseInfo(),
			},
			{
				Name:    "apocalypse_role_ack",
				Kind:    1,
				ID:      legion.CmdRoleSelect,
				Payload: legion.RoleSelectAck(),
			},
		},
		Events: []map[string]any{{
			"kind":         "apocalypse_role_prepared",
			"character_id": w.role.ID,
			"role":         request.Role,
			"seat":         run.Seat,
			"stage":        run.Stage,
		}},
	}, nil
}

// apocalypseDirectMove handles CMD2062 (DUNGEON_DIRECT_MOVE) while an apocalypse
// run is open.
//
// This is the packet behind the "创建队伍后无法直接进入开始攻坚房间" report:
// after CMD2045 the client loads the waiting room itself through CMD2062, but
// directMoveDungeon refuses every 2062 that arrives without an active dungeon
// session ("direct move without an active dungeon"), and CMD2045 deliberately
// does not create one. The run therefore never leaves the town screen.
//
// The request carries the target dungeon id at @13 (100005112 for the waiting
// room, then the five combat dungeons). The response is the standard entry
// sequence: CMD2062 ack, N3 actor state, N27 selection context, N28 dungeon
// info and N29 start map.
func (s *legionSession) apocalypseDirectMove(w *worldSession, id uint32) (legionResult, error) {
	run := s.apocalypseRun()
	if !run.Entered {
		return legionResult{}, fmt.Errorf("apocalypse direct move before the operation was confirmed (no CMD2045 yet)")
	}
	// 客户端自己发了 CMD2062 = 走快路径：撤销服务端的兜底推进任务，避免同一关
	// 被载入两次。见 apocalypseStageAdvanceDelay。
	w.cancelApocalypseAdvance()
	stage := legion.ApocalypseStageOfDungeon(id)
	if stage < 0 {
		return legionResult{}, fmt.Errorf("dungeon %d is not an apocalypse stage dungeon", id)
	}
	if stage == 0 && run.Stage > 0 {
		return legionResult{}, fmt.Errorf("apocalypse waiting room %d is not re-enterable once stage %d is active", id, run.Stage)
	}
	if stage > 0 && stage != run.PhaseCleared+1 {
		// 阶段必须顺序推进：抓包两场都是 1→2→3→4→5。
		return legionResult{}, fmt.Errorf("apocalypse stage %d requested while %d phases are cleared", stage, run.PhaseCleared)
	}
	sel := protocol.DungeonSelection{ID: id, Difficulty: 0, Party: 65535}
	sess, frames, err := w.apocalypseStageEntry(sel)
	if err != nil {
		return legionResult{}, err
	}
	// 参考抓包（20261008-105227，用 tools/apocalypse-port/infofields.py 逐帧解出）
	// 的 N2895 序列是：
	//
	//	CMD2045 之后            stage=0 marks=[00 01 ff ff ff ff]
	//	CMD2062 载入攻坚房间后   stage=1 marks=[00 01 ff ff ff ff]
	//	CMD2062 载入第 1 关后    stage=1 marks=[00 01 ff ff ff ff]
	//	第 1 关击杀确认          stage=2 marks=[00 01 02 ff ff ff]
	//	CMD2062 载入第 2 关后    stage=2 marks=[00 01 02 ff ff ff]
	//	……第 5 关击杀确认 stage=5 marks=[00 01 02 03 04 ff]
	//
	// 即：载入攻坚房间把 Stage 推到 1（marks 不变），之后每载入一关 Stage +1，
	// 每清一关 marks 多一格。PhaseCleared 记「已到达阶段」，Reached() 再兜底。
	// run.Stage 是**副本表下标**（0=攻坚房、1=第1关…）；发布出去的
	// NOTI2895 @13 才 +1（见 apocalypseInfo 与 ContinueStage 的注释）。
	// 载入副本表下标 stage 的房间 → 「已进入过的房间数」= stage + 1，
	// 这正是 NOTI2895 @13 与续关请求 CMD2045 @17 的刻度。
	run.Stage = stage + 1
	if run.PhaseCleared < run.Stage {
		run.PhaseCleared = run.Stage
	}
	plan := append([]outboundPacket{
		{"apocalypse_direct_move_ack", 1, apocalypseDirectMoveID, []byte{1}},
	}, frames...)
	// ★ 载入新房间后必须补发权威 NOTI2895（与 apocalypseAdvanceDue 同款）。
	//
	// 客户端把收到的 N2895 @13 **原样回送**成续关请求 CMD2045 @17。这条
	// 「客户端主动 CMD2062 直进」的快路径原先只加载房间、不发 N2895，于是客户端
	// 手上的阶段号停在**进入本关之前**的值。实机 2026-10-08 16:47 的完整链条：
	//
	//	16:47:15  CMD2045 续关 → N2895 @13=2（在第 1 关）
	//	16:47:31  apocalypse_advance_swept（sweep 排了推进）
	//	16:47:33  客户端自己发 CMD2062 → 本函数载入 index 2（第 2 关），Stage=3
	//	          **但不发 N2895** ⇒ 客户端仍以为自己在 stage 2
	//	16:47:45  角色死亡 → 16:47:55 请离（Stage=3）
	//	16:47:57  点「进入」→ 客户端发 @17=2 → 命中 case want-1 ⇒ 载入 index 1
	//	          ⇒ 业主看到的「重新进入的是**前一关**」
	//
	// 位置与推进路径一致：放在进图帧序之后（先载完新房间，再告知当前阶段）。
	// ★ A2：这里同样曾补发 `apocalypse_direct_move_stage_published`，已删除
	//（理由同上：老版本不发这类帧、地图内状态正常；补发会把客户端作战状态冲掉）。
	// 续关由服务端的 resumeStale 容忍分支兜底，不依赖这些补发。
	note := map[string]any{
		"kind":         "apocalypse_navigation_advanced",
		"character_id": w.role.ID,
		"stage":        run.Stage,
		"dungeon":      id,
		"map":          sess.Room.Map,
		"monsters":     len(sess.Monsters),
		"phase_key":    run.Stage,
	}
	if stage > 0 {
		note["kind"] = "apocalypse_following_combat_prepared"
	}
	return legionResult{Packets: plan, Events: []map[string]any{note}}, nil
}
