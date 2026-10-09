package main

import (
	"context"
	"dfolan/internal/database"
	"dfolan/internal/dungeon"
	"dfolan/internal/game/protocol"
	"dfolan/internal/inventory"
	"dfolan/internal/legion"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"log"
	"time"
)

// 苏醒之森（Forest of Awakening，频道 Type 96）待机区流程。
// 借鉴已实机修复的伊斯大陆（Type 81）与美神维纳斯（Type 99）：
// 待机区「创建队伍」CMD12/13 先行，开战/进图等家族命令在后续轮次按
// 实机报文逐条补齐。军团表（contents/system/legionsystem/legionsystem.cos）
// 里该内容为 ForestOfAwakeningNormal / ForestOfAwakeningHard 两块，
// 本频道 [dungeon info data] 缺失，开战轮次需先取证副本号。

// forestStandbyPartyHandle 实现苏醒之森待机区队伍对话框：CMD12 建队、
// CMD13 离队。请求与伊斯/维纳斯待机区同构，队伍类型 0x18（Normal）/
// 0x19（Extreme，ForestOfAwakeningHard）都接受并回显——只认 0x18 时
// Extreme 建队被拒、客户端弹「daily entrance limit」（21:33 会话实证）。
// 应答照黑鸦/伊斯/维纳斯先例：队长资料两个 op=2 先行 + 单帧 NOTI9。玩法
// 边界同维纳斯：单人 bootstrap 队伍（Party==1 归一化 65535），建队后 arm
// soloPartyReady；Extreme 标记随 run 带入后续硬模式流程（本轮只通建队，
// 开战/进图的内容号待实机取证，禁止照抄 Normal 的 104）。
func (w *worldSession) forestStandbyPartyHandle(id uint16, p []byte) (bool, []outboundPacket, error) {
	if w.channelType != 96 || w.role.ID == 0 || (id != 12 && id != 13) {
		return false, nil, nil
	}
	fail := func(err error) (bool, []outboundPacket, error) { return true, nil, err }
	if w.characters == nil {
		return fail(fmt.Errorf("苏醒之森待机区角色服务不可用"))
	}
	if id == 13 {
		// 离队与伊斯/维纳斯同款：原生 CMD13 body 为空或 8B 零填充；NOTI9
		// action3 清空八个成员槽。
		if len(p) != 0 && len(p) != 8 {
			return fail(fmt.Errorf("苏醒之森离队请求长度无效"))
		}
		for _, b := range p {
			if b != 0 {
				return fail(fmt.Errorf("不支持的苏醒之森离队选项"))
			}
		}
		if w.activeDungeon != nil {
			return fail(fmt.Errorf("请先返回待机区再退出苏醒之森队伍"))
		}
		packets := []outboundPacket{
			{"苏醒之森队伍解散", 0, 9, protocol.BlackPurgatoryPartyGone(w.characters.ChannelContext)},
		}
		if w.forestPartyHard || (w.forest != nil && w.forest.hard) {
			// 官服 2026-10-08 22:08:45.691：离队后补一帧 Extreme 复位态
			// （N2565 state0e），右上角面板收干净。判定同时看建队标记 ——
			// 终局点「返回城镇」时 run 已作废（w.forest = nil），只看 run 会漏发。
			packets = append(packets, outboundPacket{"forest_hard_info_idle", 0, legion.NotiForestHardInfo, legion.ForestHardIdleInfo()})
		}
		w.soloPartyReady = false
		w.forestPartyHard = false
		w.forestEntryPending = nil
		w.forest = nil
		return true, packets, nil
	}
	request, err := protocol.DecodeForestStandbyParty(p)
	if err != nil {
		return fail(err)
	}
	// Extreme 建队应答**回显官服队伍类型 0x19**（2026-10-08 抓包改正）：
	// 官服 session_s13 的建队应答就是 0x19，客户端据此进入 Extreme 模式
	// （开战内容号 105、无选音符窗、N2565 状态、CMD2062 连战过段）。
	//
	// 第十四轮曾因「成员未集结」把应答改成 0x18，那是**误判**：该弹窗的真正
	// 成因是客户端建队后会自己走到 Extreme 集结区 town198 area3（官服
	// 22:05:29.112 的 SET_USER_AREA 实锤；军团表 waiting_area = 198/3），
	// 而我们当时把角色留在 198/0/1 没放行这一步。world 目录里 198/1 → 198/3
	// 是源生门户边（bounds 8,242,80,220），放行后集结检查自然通过。
	// 回 0x18 的代价是客户端整场按 Normal 口径跑（内容号 104、弹选音符窗），
	// 与本模式完全不符。
	party, err := protocol.ForestStandbyPartyReply(request.Name, w.role.WireID, w.characters.ChannelContext, request.Hard)
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
	w.forestPartyHard = request.Hard
	return true, []outboundPacket{
		{"苏醒之森队长资料", 0, 2, basic},
		{"苏醒之森队长详细资料", 0, 2, detail},
		{"苏醒之森待机区队伍创建", 0, 9, party},
	}, nil
}

// forestRun 是一次苏醒之森挑战的会话状态。苏醒之森每关回城重选音符
// （官服无 CMD2062 直进），cleared 逐关累积；choice/ target 对应
// N2563 的选择标志与音符目标。
type forestRun struct {
	choice  byte // N2563 @2：ff=未确认，02=已确认（官服 chosen 帧值）
	target  uint32
	cleared [3]bool
	stage   int // 当前所在关（进图后指向本关）
	// stageClock[stage] 冻结本关倒计时起点（进图装载完成时刻，N1474 数据源，
	// 官服口径每关 60 分钟 = ForestStageLimits）。
	stageClock [3]time.Time
	// finalDone/storyFinished 镜像维纳斯终局状态机：CMD2046 终局应答后
	// 播放通关演出，演出恢复（CMD191 state=1）即 storyFinished，CMD72 退场
	// 作废 run。
	finalDone     bool
	storyFinished bool
	// hard 标记 Extreme（ForestOfAwakeningHard，内容 105）挑战：建队类型
	// 0x19 带入。Extreme 是**连战**：状态包走 N2565（64B 官服形状）、
	// 无选音符窗、第 1 关走 CMD2045，第 2/3 关由客户端 CMD2062 直进。
	hard bool
	// potionUsed[stage] 是本关已使用的消耗品次数（CMD44）：军团口径每关
	// 限 8 次（用户要求），进图时清零、超限拒绝（UseStackableRefused）。
	potionUsed [3]int
}

// isForestRequest 判别苏醒之森族请求：CMD2226/2227 为本族专属；
// CMD2043/2045/2046 与末世录/伊斯/维纳斯共用信封，由 body @13 的内容号
// 104（Normal，extreme 105 由开战分支与 run.hard 处理）区分。
func isForestRequest(id uint16, p []byte) bool {
	switch id {
	case legion.CmdForestOperationSelect, legion.CmdForestBuffSelect:
		return true
	}
	if !legion.Requests(id) || len(p) < legion.EnvelopeSize+4 {
		return false
	}
	return binary.LittleEndian.Uint32(p[legion.EnvelopeSize:]) == legion.ForestContentID
}

// isForestHardFamilyRequest 判别内容 105（Extreme）的家族命令
// CMD2045/2046。只在**本连接已经开过 Extreme run** 时接手，避免吞掉其它
// 内容的共用信封命令（内容号 105 在别的玩法里没有第二含义，但保守起见
// 仍加 run 门控）。
func (w *worldSession) isForestHardFamilyRequest(id uint16, p []byte) bool {
	if w.forest == nil || !w.forest.hard {
		return false
	}
	switch id {
	case legion.CmdEnterDungeon, legion.CmdRewardEnd:
	default:
		return false
	}
	if len(p) < legion.EnvelopeSize+4 {
		return false
	}
	return binary.LittleEndian.Uint32(p[legion.EnvelopeSize:]) == legion.ForestHardContentID
}

// handleForestRequest answers one forest family command (CMD2226/2045/2046;
// content 104 shared-envelope commands are discriminated by isForestRequest).
func (w *worldSession) handleForestRequest(p []byte, id uint16) ([]outboundPacket, []map[string]any, error) {
	switch id {
	case legion.CmdForestOperationSelect:
		return w.forestOperation(p)
	case legion.CmdEnterDungeon:
		return w.enterForestStage(p)
	case legion.CmdRewardEnd:
		return w.forestRewardEnd(p)
	}
	return nil, nil, fmt.Errorf("forest opcode %d is not implemented (content 104)", id)
}

// forestOperation handles CMD2226: action 1 is the window-ready ping (answer:
// the current stage's window state with state→2 + @116 清零), action 2
// confirms the melody target (answer: the current stage's window state with
// the choice flag, state 2 and this stage's melody record appended). 官服
// 三关 diff 规则见 ForestReadyInfo/ForestChosenInfo；stage = 已通关数（确认
// 的就是即将进入的这一关）。
func (w *worldSession) forestOperation(p []byte) ([]outboundPacket, []map[string]any, error) {
	req, err := legion.DecodeForestOperation(p)
	if err != nil {
		return nil, nil, err
	}
	if w.forest == nil {
		return nil, nil, fmt.Errorf("forest operation before start (no CMD2043 yet)")
	}
	if w.forest.hard {
		// 官服 Extreme 全程**没有** CMD2226（2026-10-08 抓包 s13 零帧）；
		// 真收到说明客户端还在 Normal 口径（多半是建队类型没回显 0x19），
		// 此时发 Normal 的 N2563 只会把状态机搅乱 —— 明确拒绝并落日志。
		return nil, nil, fmt.Errorf("forest operation (CMD2226) is a Normal-only step; Extreme has no melody selection")
	}
	if w.activeDungeon != nil {
		return nil, nil, fmt.Errorf("forest operation inside an active dungeon")
	}
	stage := 0
	for _, ok := range w.forest.cleared {
		if ok {
			stage++
		}
	}
	switch req.Action {
	case 1:
		info, err := legion.ForestReadyInfo(stage)
		if err != nil {
			return nil, nil, err
		}
		return []outboundPacket{
			{"forest_info_ready", 0, legion.NotiForestInfo, info},
			{"forest_operation_ready_ack", 1, legion.CmdForestOperationSelect, legion.ForestOperationReadyAck()},
		}, []map[string]any{{"kind": "forest_operation_ready", "character_id": w.role.ID, "stage": stage}}, nil
	case 2:
		if req.Value == 0 || req.Value == 0xffff || req.Value == 0xffffffff {
			return nil, nil, fmt.Errorf("forest melody target %d is not a selection", req.Value)
		}
		info, err := legion.ForestChosenInfo(stage, req.Value)
		if err != nil {
			return nil, nil, err
		}
		w.forest.choice = 0x02
		w.forest.target = req.Value
		return []outboundPacket{
			{"forest_info_chosen", 0, legion.NotiForestInfo, info},
			{"forest_operation_confirm_ack", 1, legion.CmdForestOperationSelect, legion.ForestOperationConfirmAck(req.Value)},
		}, []map[string]any{{"kind": "forest_operation_confirmed", "character_id": w.role.ID, "stage": stage, "target": req.Value}}, nil
	}
	return nil, nil, fmt.Errorf("forest operation action %d is not implemented", req.Action)
}

// enterForestStage handles CMD2045: load the stage dungeon and run the standard
// entry frame sequence. Extreme 只有**第一关**走这条命令（官服
// 22:05:41.255 CMD2045 内容 105 阶段 0）；第 2/3 关由客户端 CMD2062 直进
// （enterForestStageDirectMove）。阶段副本号按 run.hard 取两套（PVF
// hermitage_extreme_1/2/3.dgn）；苏醒之森副本无维纳斯式事件怪缺口，不注入
// 载体怪。Extreme 连战无选音符步骤，不要求 choice 确认。
func (w *worldSession) enterForestStage(p []byte) ([]outboundPacket, []map[string]any, error) {
	req, err := legion.DecodeForestEnter(p)
	if err != nil {
		return nil, nil, err
	}
	if w.activeDungeon != nil {
		return nil, nil, fmt.Errorf("forest enter while a dungeon is active")
	}
	run := w.forest
	if run == nil {
		return nil, nil, fmt.Errorf("forest enter before start (no CMD2043 yet)")
	}
	// Normal 必须先确认音符（CMD2226 action2）；Extreme 无此步骤。
	if !run.hard && run.choice != 0x02 {
		return nil, nil, fmt.Errorf("forest enter before a melody was confirmed (no CMD2226 action2)")
	}
	stage := int(req.Stage)
	if stage < 0 || stage > 2 {
		return nil, nil, fmt.Errorf("forest enter stage %d out of range", stage)
	}
	if run.cleared[stage] {
		return nil, nil, fmt.Errorf("forest stage %d already cleared", stage)
	}
	cleared := 0
	for _, ok := range run.cleared {
		if ok {
			cleared++
		}
	}
	if stage != cleared {
		return nil, nil, fmt.Errorf("forest enter stage %d, want %d (sequential)", stage, cleared)
	}
	dungeonID := legion.ForestStageDungeons[stage]
	if run.hard {
		dungeonID = legion.ForestHardStageDungeons[stage]
	}
	if w.dungeons == nil {
		return nil, nil, fmt.Errorf("forest stage %d entry unavailable: no dungeon catalog", stage)
	}
	sel := protocol.DungeonSelection{ID: dungeonID, Difficulty: 0, Party: 65535}
	s, err := dungeon.Select(*w.dungeons, sel, w.level, nil)
	if err != nil {
		return nil, nil, fmt.Errorf("forest stage %d entry unavailable: %w", stage, err)
	}
	// [FOREST-ARENA-BOSS] 苏醒之森与伊斯同款：单场 boss 战就发生在进图房间，
	// 置位后清房即通关。决定性差异：苏醒之森 boss 的多阶段（一二阶段）是
	// **客户端演出**，服务端目录里通常只有 1 只怪（18:04 会话 N29 monsters=1，
	// 每次死亡上报都是实体 4096），且部分关的客户端全程不发 CMD117——没有
	// bossCheck 落账 completionTarget，普通 boss 房完成路径永远不会触发。
	// ArenaBoss 完成路径（completion.go：ArenaBoss && Loaded && roomEnemiesDead
	// && reportableDisplayBoss）不依赖 CMD117，清房即结算。
	s.ArenaBoss = true
	noteMazeEntry(s)
	entryCtx, entryCancel := context.WithTimeout(context.Background(), 10*time.Second)
	var channelCtx *[2]byte
	if w.characters != nil {
		channelCtx = &w.characters.ChannelContext
	}
	frames, err := w.dungeonEntryPlanImpl(entryCtx, "forest_enter_ack", legion.CmdEnterDungeon, sel, s, channelCtx)
	entryCancel()
	if err != nil {
		return nil, nil, err
	}
	// 家族 CMD2045 应答是共享的 01+13B 结果块（1424FDC50 形，维纳斯同款），
	// 不是 dungeonEntryPlanImpl 默认的 1B 成功字节。Extreme 用**官服原文**
	// （24B：内容 105 + 阶段号 + 尾部 nonce，2026-10-08 抓包 s406/s465/s531）。
	enterAck := legion.EnterDungeonAck()
	if run.hard {
		official, ackErr := legion.ForestHardEnterAck(uint32(stage))
		if ackErr != nil {
			return nil, nil, ackErr
		}
		enterAck = official
	}
	frames[0].Name = "forest_enter_ack"
	frames[0].Payload = enterAck
	// N28 之前补 N3（角色状态 → 副本态）与 N27（选图上下文），维纳斯/
	// SemiRaid 军团进图序列同款。Extreme 用**官服无缝加载形态**：这一关之前
	// 客户端刚被 N2568 带进 `[SEAMLESS LOADING] … delay[4]`，官服在那里发的
	// NOTI27 头四字节是 `01 00 00 01`（22:05:45.580），冷进场形态（`00 00 00 01`）
	// 会让客户端按「新副本」处理 —— 与 N28 @30 必须写 05 是同一件事。
	actorState, se := protocol.UserState(w.role.WireID, protocol.UserStateDungeon)
	if se != nil {
		return nil, nil, se
	}
	selection := protocol.EnterDungeonSelection()
	if run.hard {
		selection = protocol.EnterDungeonSelectionSeamless()
	}
	inserted := false
	plan := make([]outboundPacket, 0, len(frames)+3)
	if run.hard {
		// 官服 22:05:41.244 / 22:05:45.580：进图前推该关作战窗（state2），
		// 客户端据此确认「这是连战的第 stage 关」。
		if window, wErr := legion.ForestHardWindowInfo(stage); wErr == nil {
			plan = append(plan, outboundPacket{"forest_hard_info_window", 0, legion.NotiForestHardInfo, window})
		}
	}
	for _, pkt := range frames {
		if pkt.ID == 28 && !inserted {
			plan = append(plan,
				outboundPacket{"forest_actor_state_dungeon", 0, 3, actorState},
				outboundPacket{"forest_dungeon_selection", 0, 27, selection},
			)
			inserted = true
		}
		plan = append(plan, pkt)
	}
	loyaltyCtx, loyaltyCancel := context.WithTimeout(context.Background(), 5*time.Second)
	loyaltyPackets, loyaltyErr := w.refreshCreatureLoyalty(loyaltyCtx, time.Now(), true)
	loyaltyCancel()
	if loyaltyErr != nil {
		return nil, nil, loyaltyErr
	}
	plan = append(plan, loyaltyPackets...)
	if run.hard {
		// 军团帧列（N26/781/782/476/629/465）：官服每关进图都发，伊斯进图一直
		// 发这一套（HUD 正常），本仓极难度此前**一条都没发**。
		plan, err = injectForestLegionFrames(plan)
		if err != nil {
			return nil, nil, err
		}
		// N23 + area=0xff：官服进图的**第一帧**（Normal/Extreme 两份抓包都是），
		// 告诉客户端「角色已经在副本区」。无缝加载（N2568）之后没有这一帧，
		// 客户端就一直停在城镇区域态 ⇒ 屏幕 UI 全丢（本轮实机的真因）。
		areaNotice, areaErr := w.forestDungeonAreaNotice()
		if areaErr != nil {
			return nil, nil, areaErr
		}
		plan = append([]outboundPacket{{"forest_dungeon_area_entered", 0, 23, areaNotice}}, plan...)
	}
	if run.hard {
		// ⚠️ Extreme 的进图是**两段式**（官服 2026-10-08 抓包，业主 2026-10-09 实机校准）：
		//
		//	22:05:41.255 c2s CMD2045（开始净化）
		//	22:05:41.514 s2c **N2568 PREPARE_LEGION_ENTER_DUNGEON** ← 净化开始横幅 + 演出
		//	22:05:45.580 s2c 作战窗 + N1584 + N28 + N29 + CMD2045 ACK ← 演出结束才进图
		//
		// 两帧之间固定 4.066s。一次连发（横幅与进图帧同一批）会让客户端在演出中途
		// 收到 N28/N29，演出永不收尾 —— 2026-10-09 实机症状正是「进第 1 关后屏幕
		// UI 全部消失、技能仍可用」。所以：横幅立即发，进图帧挂起到 4.066s 后由
		// client_connection 的秒 tick 下发（与 apocalypsePending 同款挂起-到期）。
		w.forestEntryPending = &forestStageEntryPending{
			session: s,
			stage:   stage,
			at:      time.Now().Add(forestPurifyBannerFallback),
			packets: plan,
		}
		return []outboundPacket{
				{"forest_hard_prepare_enter_dungeon", 0, legion.NotiPrepareEnterDungeon, legion.ForestHardPrepareEnterInfo()},
			}, []map[string]any{{
				"kind":         "forest_purify_banner_sent",
				"character_id": w.role.ID,
				"stage":        stage,
				"dungeon":      s.Definition.ID,
				"delay_ms":     forestPurifyBannerSeconds.Milliseconds(),
			}}, nil
	}
	w.beginForestStageEntry(s, stage)
	notes := []map[string]any{{
		"kind":         "forest_stage_entered",
		"character_id": w.role.ID,
		"stage":        stage,
		"target":       run.target,
		"dungeon":      s.Definition.ID,
		"maze":         s.Maze.Index,
		"map":          s.Room.Map,
		"monsters":     len(s.Monsters),
	}}
	return plan, notes, nil
}

// forestDungeonArea 是官服「角色已进入副本」的 N23（USER_AREA）里的区域号。
//
// 官服两份抓包（Normal 2026-10-02、Extreme 2026-10-08）**每次进图的第一个 s2c
// 帧都是 N23 + area=0xff**，例如：
//
//	官服 Normal  21:42:18.422  id=23  c401 c6000000 ff000000 b903 da00 05 …
//	官服 Extreme 22:05:45.515  id=23  4400 c6000000 ff000000 ee01 1f01 05 …
//
// 本仓此前**任何副本进图都不发这一帧**：非无缝路径下客户端自己会把区域切成
// 副本态（所以普通副本 HUD 正常）；但走 N2568 的无缝加载时，客户端**在等服务端
// 把区域改成 0xff**，我们不发 ⇒ 客户端认为仍在城镇 ⇒ 城镇界面收起、副本界面
// 不建 ⇒ 2026-10-09 业主连着四轮看到的「进图后屏幕 UI 全丢、技能仍可用」。
const forestDungeonArea uint32 = 0xff

// forestDungeonAreaNotice 构造「角色进入副本区」的 N23。位置沿用当前落点，
// 只把区域号换成 0xff（官服同形：town 不变、x/y 为进图前落点、第 5 字节 05）。
func (w *worldSession) forestDungeonAreaNotice() ([]byte, error) {
	p := w.state.Position
	return protocol.UserArea(p.Town, forestDungeonArea, protocol.AreaUser{
		ActorServerID: w.role.WireID,
		X:             p.X,
		Y:             p.Y,
		Flags:         w.flags,
	})
}

// forestLegionEntryFrames 是官服军团副本进图时随进图帧列一起下发的
// **军团帧列**，与伊斯 enterIspinsStage 用的是同一批原文字节
// （internal/legion/ispins_replay_frames.generated.go，出自官服 session_s4 抓包）。
//
// 为什么极难度必须补上（2026-10-09 业主三轮实机校准）：
//
//	官服 Extreme 每关进图批次（22:05:45.580 / 22:06:15.456 / 22:06:46.792）里
//	都有 N476（疲劳加速）、N629（连战关联副本信息）、N465（怪物移动系统）、
//	N26（UDP_HOST）、N781/N782（周无限难度）；本仓极难度进图**一条都没发**，
//	而伊斯进图一直发这一套（HUD 正常）——差别就在这里。
//	其中 N629 `linked_dungeon_info`（"连战"）与 N476 是与副本内界面最相关的两帧。
//
// 注入位置照官服/Ispins 的顺序：N26/781/782 在 N27 之前、N476 在 N1584 之前、
// N629 在 N28 之后、N465 在 N29 之后。
func forestLegionEntryFrames() (before27, before1584, after28, after29 []outboundPacket, err error) {
	pre, err := legion.IspinsReplayFrames("udp_host", "infinite_difficulty_user", "infinite_difficulty_charac")
	if err != nil {
		return nil, nil, nil, nil, err
	}
	before27 = ispinsReplayPackets("forest_legion_", pre)
	mid, err := legion.IspinsReplayFrames("fatigue_acceleration")
	if err != nil {
		return nil, nil, nil, nil, err
	}
	before1584 = ispinsReplayPackets("forest_legion_", mid)
	if linked, err := legion.IspinsReplayFrames("linked_dungeon_info"); err != nil {
		return nil, nil, nil, nil, err
	} else {
		after28 = ispinsReplayPackets("forest_legion_", linked)
	}
	if moves, err := legion.IspinsReplayFrames("monster_move_system"); err != nil {
		return nil, nil, nil, nil, err
	} else {
		after29 = ispinsReplayPackets("forest_legion_", moves)
	}
	return before27, before1584, after28, after29, nil
}

func ispinsReplayPackets(prefix string, frames []legion.IspinsReplayFrame) []outboundPacket {
	out := make([]outboundPacket, 0, len(frames))
	for _, f := range frames {
		out = append(out, outboundPacket{prefix + f.Name, 0, f.ID, f.Body})
	}
	return out
}

// injectForestLegionFrames 把上面的军团帧列按官服顺序插进进图计划。
// 找不到锚点（N27/N1584/N28/N29）时**报错而不是静默跳过** —— 静默少发一帧
// 正是这一轮丢 HUD 的成因。
func injectForestLegionFrames(plan []outboundPacket) ([]outboundPacket, error) {
	before27, before1584, after28, after29, err := forestLegionEntryFrames()
	if err != nil {
		return nil, err
	}
	anchors := map[string]bool{"27": false, "1584": false, "28": false, "29": false}
	out := make([]outboundPacket, 0, len(plan)+len(before27)+len(before1584)+len(after28)+len(after29))
	for _, pkt := range plan {
		if pkt.Kind == 0 {
			switch pkt.ID {
			case 27:
				if !anchors["27"] {
					out = append(out, before27...)
					anchors["27"] = true
				}
			case 1584:
				if !anchors["1584"] {
					out = append(out, before1584...)
					anchors["1584"] = true
				}
			}
		}
		out = append(out, pkt)
		if pkt.Kind != 0 {
			continue
		}
		switch pkt.ID {
		case 28:
			if !anchors["28"] {
				out = append(out, after28...)
				anchors["28"] = true
			}
		case 29:
			if !anchors["29"] {
				out = append(out, after29...)
				anchors["29"] = true
			}
		}
	}
	for name, seen := range anchors {
		if !seen {
			return nil, fmt.Errorf("forest legion entry plan lacks anchor NOTI%s（军团帧列必须按官服位置注入）", name)
		}
	}
	return out, nil
}

// forestPurifyBannerSeconds 是「净化开始」横幅 + 演出到真正进图的间隔。
//
// **它必须等于 N2568 正文 @8..11 的 delay 字段（官服 = 4 秒）**：客户端拿到这一帧
// 会打 `[SEAMLESS LOADING] PREPARE_LEGION_ENTER_DUNGEON start type[1] delay[4]`，
// 进图帧列必须落在它自己声明的窗口内，否则客户端退回非无缝路径、界面态卡在
// 「无缝加载中」—— 实机症状就是进图后屏幕 UI 全丢。官服 22:05:41.514（N2568）
// → 22:05:45.514（下一批）正好 4.000s。测试
// `TestForestHardPurifyDelayMatchesVector` 会把这两个数钉在一起。
const forestPurifyBannerSeconds = 4 * time.Second

// forestPurifyBannerFallback 是精确定时器失效时的兜底期限（1 秒 tick 才会用到）。
// 取 6 秒：宁可晚一点进图，也不要早于客户端声明的 4 秒窗口。
const forestPurifyBannerFallback = 6 * time.Second

// forestStageEntryPending 是「净化开始横幅已发、进图帧列待发」的挂起项。
type forestStageEntryPending struct {
	session *dungeon.Session
	stage   int
	at      time.Time
	packets []outboundPacket
}

// forestEntryDue 在演出结束后下发挂起的进图帧列。
//
// force=true 由 connection_session 的精确定时器驱动（按 N2568 的 delay 到期），
// 直接放行；force=false 是 1 秒 tick 的兜底路径，只在超过 fallback 期限时才发。
//
// 无论哪条路径都在主循环里执行，所以可以直接改 worldSession。
func (w *worldSession) forestEntryDue(now time.Time, force bool) ([]outboundPacket, []map[string]any) {
	pending := w.forestEntryPending
	if pending == nil {
		return nil, nil
	}
	if !force && now.Before(pending.at) {
		return nil, nil
	}
	w.forestEntryPending = nil
	if w.forest == nil || w.activeDungeon != nil {
		return nil, []map[string]any{{
			"kind":   "forest_purify_entry_aborted",
			"reason": "run finished or another dungeon is active",
		}}
	}
	w.beginForestStageEntry(pending.session, pending.stage)
	return pending.packets, []map[string]any{{
		"kind":         "forest_stage_entered",
		"character_id": w.role.ID,
		"stage":        pending.stage,
		"dungeon":      pending.session.Definition.ID,
		"maze":         pending.session.Maze.Index,
		"map":          pending.session.Room.Map,
		"monsters":     len(pending.session.Monsters),
		"after_banner": true,
	}}
}

// beginForestStageEntry 是进图真正生效那一刻的会话样板（Normal 立即调用，
// Extreme 在横幅演出结束后由 forestEntryDue 调用）。
func (w *worldSession) beginForestStageEntry(s *dungeon.Session, stage int) {
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
	if w.forest != nil {
		w.forest.stage = stage
		if stage >= 0 && stage < len(w.forest.potionUsed) {
			w.forest.potionUsed[stage] = 0
		}
	}
}

// completeForestStage 是清怪完成钩子（completeDungeon 的苏醒之森分支）。
// Normal（官服 21:43:02 清关链）：
//
//	N31 横幅 → N2252（token）→ N2 角色资料 → N2253 第二排 →
//	N9 队伍稳态 → N2563 cleared
//
// Extreme（连战，第 1/3 次尝试）：清怪投影 = N2565 状态推进（state 2、
// 下一阶段；终点关 state 3），镜像维纳斯的「清怪投影推进权威状态、客户端
// 自行直进」——硬模式无选音符/CMD46 环节的官服样本，投影值按家族同构取，
// 实机帧决定第 2/3 次取值。
func (w *worldSession) completeForestStage() ([]outboundPacket, error) {
	if !w.activeDungeon.Completed() || w.completionSent {
		return nil, nil
	}
	run := w.forest
	if run == nil {
		return nil, nil
	}
	stage, _, err := legion.ForestStageOfDungeonAny(w.activeDungeon.Definition.ID)
	if err != nil {
		return nil, err
	}
	// Extreme 连战（官服 2026-10-08 抓包口径，2026-10-09 改正）：第 1/2 关
	// 清关只发该关 N31 横幅（官服 22:06:06.828 / 22:06:44.364 **确实有横幅**，
	// 旧实现「非终点关不发横幅」是错的）；翻牌/材料链只在终点关出现。
	// 状态推进（N2565 清关 tick / 下一关作战窗）由 forestResult（CMD46 应答）
	// 下发，与官服同序：CMD39 → N31 → 客户端 CMD46 → tick + 窗态 → CMD2062。
	if run.hard && stage < len(legion.ForestStageDungeons)-1 {
		if run.cleared[stage] {
			return nil, nil // 一次性投影（venus 同款守卫）
		}
		banner, err := legion.ForestHardStageClearEnabled(stage)
		if err != nil {
			return nil, err
		}
		run.cleared[stage] = true
		return []outboundPacket{
			{"dungeon_clear_enabled", 0, 31, banner},
		}, nil
	}
	clearBanner, err := legion.ForestStageClearEnabledFor(stage, run.hard)
	if err != nil {
		return nil, err
	}
	clearedInfo, err := legion.ForestClearedInfo(stage)
	if err != nil {
		return nil, err
	}
	// 通关奖励表：Normal 终局 = N2252 四条（材料 + 3 件随机魔器/神器装备）+
	// N2253 五条；Extreme 终局 = 官服七行 N2252 + 一行 N2253
	//（2026-10-08 抓包解压后的真表，见 forest_rewards.go）；Normal 第 1/2 关
	// = N2252 token-only + N2253 三条。
	// 显示与发放同源（同一张表），全部经 Awarder 真实入库。
	var basicItems, additionalItems []legion.ForestRewardItem
	if run.hard {
		basicItems = legion.ForestHardFinalBasic()
		additionalItems = legion.ForestHardFinalAdditional()
	} else if stage == len(legion.ForestStageDungeons)-1 {
		basicItems = legion.ForestNormalFinalBasic()
		additionalItems = legion.ForestNormalFinalAdditional()
	} else {
		additionalItems = legion.ForestNormalStageRewards()
	}
	// 装备槽 roll：Normal 终局 3 件 115 级魔器/神器（维纳斯翻牌池子）。
	// Extreme 七行全是确定模板，没有装备槽。
	var gear []uint32
	for _, item := range basicItems {
		if item.Template == 0 {
			gear = append(gear, 0)
		}
	}
	if len(gear) != 0 {
		gear = legion.VenusRollFlipGear(venusFlipGearPool, len(gear))
	}
	flip, err := legion.ForestBasicClearReward(stage, basicItems, gear, run.hard)
	if err != nil {
		return nil, err
	}
	additional, err := legion.ForestAdditionalReward(additionalItems, run.hard)
	if err != nil {
		return nil, err
	}
	var plan []outboundPacket
	// 官服 Extreme 终局顺序（22:07:23.311→.323）：N31 → **N2566 过段 tick**
	// → N2252 → N2253。N2566 放最前，与官服同序。
	if run.hard {
		plan = append(plan, outboundPacket{"forest_hard_phase_clear_tick", 0, legion.NotiForestHardPhaseTick, legion.ForestHardPhaseClearTick()})
	}
	// N31 清关横幅：包名必须是 dungeon_clear_enabled（dispatch 发送监视器
	// 按它置 completionSent，维纳斯同款约定）。
	plan = append(plan, outboundPacket{"dungeon_clear_enabled", 0, 31, clearBanner})
	plan = append(plan, outboundPacket{"forest_basic_clear_reward", 0, legion.NotiIspinsBasicClearReward, flip})
	// N2 角色资料（队长资料 op=2 对；官服为单帧 USERINFO，实机复核点）。
	if w.characters != nil {
		basicInfo, e := w.characters.EntryBasicProbe(w.role, w.characters.ChannelContext)
		if e != nil {
			return nil, e
		}
		detailInfo, e := w.characters.EntryAddition(w.role)
		if e != nil {
			return nil, e
		}
		plan = append(plan,
			outboundPacket{"forest_settlement_character_info", 0, 2, basicInfo},
			outboundPacket{"forest_settlement_character_detail", 0, 2, detailInfo},
		)
	}
	// 入库：翻牌两排的每一条（含装备槽 roll 结果）经 Awarder 真实发放 +
	// N14 背包刷新（显示与发放同源）。
	var inventoryPlan []outboundPacket
	if w.loot != nil {
		grant := make([]legion.ForestRewardItem, 0, len(basicItems)+len(additionalItems))
		gearAt := 0
		for _, item := range basicItems {
			if item.Template == 0 {
				if gearAt < len(gear) {
					grant = append(grant, legion.ForestRewardItem{Template: gear[gearAt], Amount: 1})
				}
				gearAt++
				continue
			}
			grant = append(grant, item)
		}
		grant = append(grant, additionalItems...)
		awarder := &inventory.Awarder{
			Catalog:   w.loot.Catalog,
			Rules:     w.loot.BagRules,
			Equipment: w.loot.Equipment,
		}
		before, _ := inventory.ReadBag(w.role.State)
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		for _, item := range grant {
			updated, _, gErr := awarder.Grant(w.role.State, item.Template, item.Amount)
			if gErr != nil {
				log.Printf("forest flip award failed: template=%d amount=%d error=%v", item.Template, item.Amount, gErr)
				continue
			}
			w.role.State = updated
		}
		if w.store != nil {
			key := "forest-flip:" + w.activeDungeon.RunID
			_, _, _ = w.store.CommitCharacterEvent(ctx, w.account, w.role.ID, w.role.ConfigVersion,
				key, "forest-flip-v1", func(current database.Character) (json.RawMessage, json.RawMessage, error) {
					proof, _ := json.Marshal(map[string]any{"stage": stage, "hard": run.hard, "granted": grant})
					return w.role.State, proof, nil
				})
		}
		cancel()
		if after, rErr := inventory.ReadBag(w.role.State); rErr == nil {
			if updatePayload, uErr := protocol.InventoryUpdate(inventory.ChangedItemRows(before, after)); uErr == nil {
				inventoryPlan = append(inventoryPlan, outboundPacket{"forest_flip_inventory_updated", 0, 14, updatePayload})
				log.Printf("forest flip award: hard=%v stage=%d granted=%+v n14_bytes=%d", run.hard, stage, grant, len(updatePayload))
			}
		}
	}
	plan = append(plan, inventoryPlan...)
	plan = append(plan, outboundPacket{"forest_additional_clear_reward", 0, legion.NotiIspinsAdditionalClearReward, additional})
	party, err := protocol.SoloPartyInfo(w.role.WireID)
	if err != nil {
		return nil, err
	}
	plan = append(plan, outboundPacket{"forest_settlement_party_steady", 0, 9, party})
	if run.hard {
		// Extreme 终局不等 CMD46 的那一帧：官服 22:07:23.375 在 CMD46 之前
		// 已经把 N31/N2566/N2252/N2253 发完，最后那帧是 N2565 清关 tick
		//（state2、@10=2），由 forestResult 下发。
		run.cleared[stage] = true
		return plan, nil
	}
	plan = append(plan, outboundPacket{"forest_info_cleared", 0, legion.NotiForestInfo, clearedInfo})
	run.cleared[stage] = true
	return plan, nil
}

// forestStageTimer 在关卡装载完成后下发 N1474 关卡倒计时（60 分钟/关，官服
// 21:42:19.431 同款位置：CMD37 ack 之后、N30 之前由 venus 挂点同序）。N1474
// 只驱动客户端显示，失败判定是服务端职责（1474 规格文档）。
func (w *worldSession) forestStageTimer(now time.Time) []outboundPacket {
	if w.forest == nil || w.activeDungeon == nil || !legion.IsForestStageDungeonAny(w.activeDungeon.Definition.ID) {
		return nil
	}
	stage, _, err := legion.ForestStageOfDungeonAny(w.activeDungeon.Definition.ID)
	if err != nil || stage >= len(legion.ForestStageLimits) {
		return nil
	}
	if w.forest.stageClock[stage].IsZero() {
		w.forest.stageClock[stage] = now
	}
	body, err := protocol.LegionDungeonTimeout115(w.forest.stageClock[stage], time.Duration(legion.ForestStageLimits[stage])*time.Second)
	if err != nil {
		log.Printf("forest stage timer encode failed (stage %d): %v", stage, err)
		return nil
	}
	return []outboundPacket{{"forest_stage_timer_sync", 0, legion.NotiDungeonTimeoutTime, body}}
}

// forestStageTimeout 判定关卡倒计时到期（挑战失败，venusStageTimeout 同款
// 链路）：N33 原生超时失败开路（reason 100=timeout），42-ack 不发；失败演出
// 把角色置入死亡态，城镇序列末尾补原生复活；末尾回等待态 N2563 并整场复位
// （苏醒之森超时=本场作废，重新开团走 CMD2043）。
func (w *worldSession) forestStageTimeout(now time.Time, event func(map[string]any)) []outboundPacket {
	if w.forest == nil || w.activeDungeon == nil || w.activeDungeon.Completed() ||
		w.completionSent || w.resultSent ||
		!legion.IsForestStageDungeonAny(w.activeDungeon.Definition.ID) {
		return nil
	}
	stage, _, err := legion.ForestStageOfDungeonAny(w.activeDungeon.Definition.ID)
	if err != nil || stage >= len(legion.ForestStageLimits) {
		return nil
	}
	limit := time.Duration(legion.ForestStageLimits[stage]) * time.Second
	started := w.forest.stageClock[stage]
	if started.IsZero() || now.Sub(started) < limit {
		return nil
	}
	route, err := w.leaveDungeon()
	if err != nil {
		log.Printf("forest stage timeout leave failed (stage %d): %v", stage, err)
		return nil
	}
	hard := w.forest.hard
	w.activeDungeon = nil
	w.selectingDungeon = false
	w.approvedDungeonGate = 0
	w.drops = nil
	w.deathSent = nil
	w.completionSent = false
	w.resultSent = false
	w.resetCards()
	// 超时=本场作废：保留模式（Extreme 重开团仍走 N2565），只清进度。
	w.forest = &forestRun{choice: 0xff, hard: hard}
	if event != nil {
		event(map[string]any{"kind": "forest_stage_timeout", "character_id": w.role.ID,
			"stage": stage, "hard": hard, "started": started.Unix(), "limit": int(limit / time.Second)})
	}
	packets := []outboundPacket{{"forest_time_limit_failed", 0, 33, protocol.DungeonFailClear(100)}}
	for _, p := range route {
		if p.Name != "dungeon_leave_ack" {
			packets = append(packets, p)
		}
	}
	revive, err := protocol.PlayerDeathState(w.role.WireID)
	if err == nil {
		revive[2] = 1
		packets = append(packets, outboundPacket{"forest_timeout_actor_revived", 0, 32, revive})
	}
	// 超时=本场作废：右上角倒计时归零重来（业主 2026-10-09：撤退/判负出本后
	// 倒计时必须重置，不能停在被判负的那一档）。
	packets = append(packets, w.forestStageTimerReset(now, stage, "timeout")...)
	if hard {
		return append(packets, outboundPacket{"forest_hard_info_timeout_waiting", 0, legion.NotiForestHardInfo, legion.ForestHardWaitingInfo()})
	}
	return append(packets, outboundPacket{"forest_info_timeout_waiting", 0, legion.NotiForestInfo, legion.ForestWaitingInfo()})
}

// forestStageTimerReset 是「离开副本后把该关倒计时清零重来」的公共动作：
// 清掉冻结的起算时刻（下一次进图按满额 60 分钟重新起算）并补发一帧 N1474，
// 让客户端右上角立刻回到满额、而不是停在被中断的那一档。
//
// 业主 2026-10-09 实机 BUG：「点击撤退出去或者死亡强制退出，右上角的倒计时
// 没有刷新，正常情况应该重置」。以往只有**死亡请离**那条链给维纳斯清过时钟
// （player_death.go 的 venus_death_stage_timer_reset），苏醒之森既没清时钟、
// 也没补帧，于是被中断的剩余时间一直留着；再进图时因为 stageClock 仍是旧值，
// 客户端算出来的还是「剩下的时间」。
func (w *worldSession) forestStageTimerReset(now time.Time, stage int, reason string) []outboundPacket {
	if w.forest == nil || stage < 0 || stage >= len(w.forest.stageClock) {
		return nil
	}
	w.forest.stageClock[stage] = time.Time{}
	if stage < len(w.forest.potionUsed) {
		w.forest.potionUsed[stage] = 0
	}
	body, err := protocol.LegionDungeonTimeout115(now, time.Duration(legion.ForestStageLimits[stage])*time.Second)
	if err != nil {
		log.Printf("forest stage timer reset failed (stage %d): %v", stage, err)
		return nil
	}
	return []outboundPacket{{"forest_stage_timer_reset", 0, legion.NotiDungeonTimeoutTime, body}}
}

// forestStageOfActiveRun 取当前副本对应的苏醒之森关卡号（不是本内容/没有
// 活动副本时返回 -1）。
func (w *worldSession) forestStageOfActiveRun() int {
	if w.forest == nil || w.activeDungeon == nil {
		return -1
	}
	stage, _, err := legion.ForestStageOfDungeonAny(w.activeDungeon.Definition.ID)
	if err != nil {
		return -1
	}
	return stage
}

// forestResult 处理清关后的 CMD46（dungeonResult 的苏醒之森分支）：通用
// 结算（N34/N35/N261+8张牌）不适用。
//
//   - Normal：应答 = 下一关作战窗（state6，官服 21:43:02.340）；终局关无下一
//     窗，回 cleared 态（官服 21:45:25.262）。
//   - Extreme：应答 = N2565 清关 tick（官服 22:06:06.894 / 22:06:44.387 /
//     22:07:23.375）＋（非终点关）下一关作战窗（state2）。官服实测客户端
//     收到窗态后自行发 CMD2062 直进下一关（22:06:15.191 / 22:06:46.183）。
//
// 官服 CMD46 后客户端自行完成场景切换，无回城包链；服务端在此静默收尾
// 副本会话。
func (w *worldSession) forestResult(p []byte) ([]outboundPacket, error) {
	run := w.forest
	if run == nil {
		return nil, nil
	}
	// 关卡上下文：副本会话还在按副本号解析；连战下客户端可能在 2062 被拒后
	// 重发 CMD46（0231 会话），此时会话已被上一轮收尾——按 run.cleared 推断
	// 已到的关。cleared[i]=true 的个数 = 已完成关数，下一关 = 该数。
	cleared := 0
	for _, ok := range run.cleared {
		if ok {
			cleared++
		}
	}
	stage := cleared - 1
	if w.activeDungeon != nil {
		if s, _, err := legion.ForestStageOfDungeonAny(w.activeDungeon.Definition.ID); err == nil {
			stage = s
		}
	}
	if stage < 0 {
		return nil, nil
	}
	terminal := stage == len(legion.ForestStageDungeons)-1
	next := stage + 1
	var plan []outboundPacket
	switch {
	case run.hard:
		tick, err := legion.ForestHardClearTickInfo(stage)
		if err != nil {
			return nil, err
		}
		plan = append(plan, outboundPacket{"forest_hard_stage_clear_tick", 0, legion.NotiForestHardInfo, tick})
		if !terminal {
			window, err := legion.ForestHardWindowInfo(next)
			if err != nil {
				return nil, err
			}
			plan = append(plan, outboundPacket{"forest_hard_info_next_window", 0, legion.NotiForestHardInfo, window})
		}
	case terminal:
		clearedBody, err := legion.ForestClearedInfo(stage)
		if err != nil {
			return nil, err
		}
		plan = append(plan, outboundPacket{"forest_info_cleared", 0, legion.NotiForestInfo, clearedBody})
	default:
		info, err := legion.ForestWindowInfo(next)
		if err != nil {
			return nil, err
		}
		plan = append(plan, outboundPacket{"forest_info_next_window", 0, legion.NotiForestInfo, info})
	}
	// 静默收尾副本会话（无回城包链，官服口径）。
	w.leaveScene()
	w.deathSent = map[uint16]bool{}
	w.drops = nil
	w.completionSent = false
	w.completionErr = nil
	w.resultSent = false
	w.selectingDungeon = false
	w.approvedDungeonGate = 0
	w.pendingTownArrival = nil
	w.activeDungeon = nil
	return plan, nil
}

// forestRewardEnd handles the terminal CMD2046 (content 104/105, stage 2):
// ACK + 终局状态（Normal = N2563 state3，官服 21:45:39.092；Extreme =
// N2565 state3，第 1/3 次尝试值）。非终局或未清关拒绝。
func (w *worldSession) forestRewardEnd(p []byte) ([]outboundPacket, []map[string]any, error) {
	req, err := legion.DecodeForestEnter(p)
	if err != nil {
		return nil, nil, err
	}
	run := w.forest
	if run == nil {
		return nil, nil, fmt.Errorf("forest reward end before start (no CMD2043 yet)")
	}
	stage := int(req.Stage)
	if stage != len(legion.ForestStageDungeons)-1 {
		return nil, nil, fmt.Errorf("forest reward end stage %d is not the terminal stage", stage)
	}
	if !run.cleared[stage] {
		return nil, nil, fmt.Errorf("forest reward end before stage %d was cleared", stage)
	}
	if run.finalDone {
		return []outboundPacket{{"forest_reward_end_ack", 1, legion.CmdRewardEnd, legion.RewardEndAck()}}, nil, nil
	}
	run.finalDone = true
	if run.hard {
		// Extreme：官服 22:07:37.264/.329 先推 N2565 终局态（state3，触发通关
		// 演出）再回 ACK（32B 原文，内容 105 + 阶段 2）。
		return []outboundPacket{
				{"forest_hard_info_final", 0, legion.NotiForestHardInfo, legion.ForestHardFinalInfo()},
				{"forest_reward_end_ack", 1, legion.CmdRewardEnd, legion.ForestHardRewardEndAck()},
			}, []map[string]any{{
				"kind":         "forest_reward_end",
				"character_id": w.role.ID,
				"stage":        stage,
				"terminal":     true,
				"hard":         true,
			}}, nil
	}
	return []outboundPacket{
			{"forest_reward_end_ack", 1, legion.CmdRewardEnd, legion.RewardEndAck()},
			{"forest_info_final", 0, legion.NotiForestInfo, legion.ForestFinalInfo()},
		}, []map[string]any{{
			"kind":         "forest_reward_end",
			"character_id": w.role.ID,
			"stage":        stage,
			"terminal":     true,
		}}, nil
}

// enterForestStageDirectMove 接管 CMD2062 直进：两种触发——Normal 音符类型
// 0x04 的转关（确认音符后不回待机区走 CMD2045，官服 21:44:34.387 目标
// 100003881；私服 21:06 会话目标 100003880 同构）与 Extreme 连战的过段直进
// （镜像维纳斯清怪投影后客户端自行 2062）。Normal 上一关会话已在
// forestResult（CMD46）收尾、activeDungeon 为空；Extreme 的投影在清怪时已
// 推进（同维纳斯），directMoveDungeon 在通用 activeDungeon 门控之前接管。
// 应答照维纳斯转阶段的客户端已验证形状（dungeonSelectionHead + 通用进图
// 序列），装载完成后的 N1474 由 forestStageTimer 挂点补发。
func (w *worldSession) enterForestStageDirectMove(r protocol.DungeonDirectMove) (*dungeon.Session, []outboundPacket, error) {
	run := w.forest
	if run == nil {
		return nil, nil, fmt.Errorf("forest direct move without a run")
	}
	// 阶段号：优先按直进目标副本号解析；连战帧的 @13 可能为 0（客户端本地
	// 无该阶段副本号，0231 会话实证），此时按已通关数推断（直进的永远是
	// 下一关 = cleared 数）。
	stage, _, err := legion.ForestStageOfDungeonAny(r.ID)
	if err != nil {
		cleared := 0
		for _, ok := range run.cleared {
			if ok {
				cleared++
			}
		}
		if r.ID != 0 || cleared == 0 || cleared > 2 {
			return nil, nil, err
		}
		stage = cleared
	}
	if !run.hard && run.choice != 0x02 {
		return nil, nil, fmt.Errorf("forest direct move before a melody was confirmed (no CMD2226 action2)")
	}
	cleared := 0
	for _, ok := range run.cleared {
		if ok {
			cleared++
		}
	}
	if stage != cleared {
		return nil, nil, fmt.Errorf("forest direct move stage %d, want %d (sequential)", stage, cleared)
	}
	if w.dungeons == nil {
		return nil, nil, fmt.Errorf("forest stage %d direct move unavailable: no dungeon catalog", stage)
	}
	dungeonID := legion.ForestStageDungeons[stage]
	if run.hard {
		dungeonID = legion.ForestHardStageDungeons[stage]
	}
	sel := protocol.DungeonSelection{ID: dungeonID, Difficulty: 0, Party: 65535}
	s, err := dungeon.Select(*w.dungeons, sel, w.level, nil)
	if err != nil {
		return nil, nil, fmt.Errorf("forest stage %d direct move unavailable: %w", stage, err)
	}
	// 同 enterForestStage：多阶段 boss 是客户端演出，ArenaBoss 让清房即通关。
	s.ArenaBoss = true
	noteMazeEntry(s)
	entryCtx, entryCancel := context.WithTimeout(context.Background(), 10*time.Second)
	var channelCtx *[2]byte
	if w.characters != nil {
		channelCtx = &w.characters.ChannelContext
	}
	frames, err := w.dungeonEntryPlanImpl(entryCtx, "dungeon_select_ack", 16, sel, s, channelCtx)
	entryCancel()
	if err != nil {
		return nil, nil, err
	}
	// 官服 Extreme 过段（22:06:15.456）：先推下一关作战窗，再走 N28/N29，
	// 然后 CMD2045 ACK（内容 105 + 阶段）+ CMD2062 ACK。私服沿用维纳斯
	// 已验证的选图层形状（dungeonSelectionHead），只把 2045 应答换成官服原文，
	// 并补 CMD2062 的 01+nonce ACK（官服 s466/s532）。
	plan := dungeonSelectionHead()
	if run.hard {
		// Extreme 过段用官服「连战直进」形态的 NOTI27（头四字节 `00 01 01 01`，
		// 抓包 22:06:15.456 / 22:06:46.792），并把选图层压成官服那样只有 N27
		// —— 过段是同一场连战里的继续，不是新副本。
		plan = []outboundPacket{{"forest_dungeon_selection", 0, 27, protocol.EnterDungeonSelectionStageContinue()}}
		if window, wErr := legion.ForestHardWindowInfo(stage); wErr == nil {
			plan = append(plan, outboundPacket{"forest_hard_info_window", 0, legion.NotiForestHardInfo, window})
		}
	}
	plan = append(plan, frames...)
	if run.hard {
		// 过段直进同样补军团帧列（官服 22:06:15.456 / 22:06:46.792 第二三关
		// 的进图批次里 N476/N629/N465 一帧不少）+ N23 area=0xff（每关都发）。
		plan, err = injectForestLegionFrames(plan)
		if err != nil {
			return nil, nil, err
		}
		areaNotice, areaErr := w.forestDungeonAreaNotice()
		if areaErr != nil {
			return nil, nil, areaErr
		}
		plan = append([]outboundPacket{{"forest_dungeon_area_entered", 0, 23, areaNotice}}, plan...)
		ack, ackErr := legion.ForestHardEnterAck(uint32(stage))
		if ackErr != nil {
			return nil, nil, ackErr
		}
		plan = append(plan,
			outboundPacket{"forest_hard_enter_ack", 1, legion.CmdEnterDungeon, ack},
			outboundPacket{"forest_direct_move_ack", 1, 2062, protocol.DungeonDirectMoveAck()},
		)
	}
	run.stage = stage
	run.potionUsed[stage] = 0
	// 会话样板与主循环一致（dispatch 发送后把 pending 落位 activeDungeon，
	// 宠物忠诚由该路径统一刷新，这里不重复）。
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
	return s, plan, nil
}

// forestPotionGate 是苏醒之森副本内的消耗品限制（军团口径每关 8 次）：
// 命中返回拒绝应答；未命中把计数 +1 并返回 nil。计数在进图时清零。
const forestPotionLimit = 8

func (w *worldSession) forestPotionGate(r protocol.UseStackableRequest) []outboundPacket {
	if w.forest == nil || w.activeDungeon == nil {
		return nil
	}
	stage, _, err := legion.ForestStageOfDungeonAny(w.activeDungeon.Definition.ID)
	if err != nil || stage >= len(w.forest.potionUsed) {
		return nil
	}
	if w.forest.potionUsed[stage] >= forestPotionLimit {
		return []outboundPacket{{"forest_potion_refused", 1, 44, protocol.UseStackableRefused(r)}}
	}
	w.forest.potionUsed[stage]++
	return nil
}

// forestStoryPause 应答终局演出期间的 CMD191（剧情暂停/恢复，维纳斯同款）：
// 恢复（state=1）= 演出播完，发 leave 态（state5）关右上角面板。
func (w *worldSession) forestStoryPause(p []byte) ([]outboundPacket, error) {
	r, err := protocol.DecodeStoryPause(p)
	if err != nil {
		return nil, err
	}
	run := w.forest
	if run == nil || !run.finalDone {
		return nil, fmt.Errorf("forest story pause outside the finale")
	}
	notice, err := protocol.StoryPauseNotice(w.role.WireID, r)
	if err != nil {
		return nil, err
	}
	plan := []outboundPacket{{"forest_story_pause", 0, 170, notice}}
	if r.State == 1 && !run.storyFinished {
		run.storyFinished = true
		leaveNotice := outboundPacket{"forest_info_leave", 0, legion.NotiForestInfo, legion.ForestLeaveInfo()}
		if run.hard {
			// Extreme 的 leave 态走 N2565 state5（官服 22:08:33.372）。
			leaveNotice = outboundPacket{"forest_hard_info_leave", 0, legion.NotiForestHardInfo, legion.ForestHardLeaveInfo()}
		}
		plan = append(plan, leaveNotice)
		if run.hard {
			// Extreme **不自动回城**（业主 2026-10-09 实机口径）：leave 态之后
			// 客户端右上角出现「返回城镇」，玩家自己点 —— 官服同一时刻的帧列是
			// 22:08:33.372 N2565 state5 → 22:08:33.821 c2s CMD72 → CMD72 ACK。
			// 第十八轮的自动回城是「连战态没有返回按钮」时期的权宜（0254 会话），
			// 走对 Extreme 协议（队伍 0x19 + N2565）之后该按钮回来了，自动回城
			// 反而把玩家的操作抢掉。这里只留状态，落点交给 CMD72（card_flow.go
			// 的苏醒之森终局退场分支）。
			return plan, nil
		}
		// Normal 保持第十八轮实机确认过的自动回城：视频播完即回待机区。
		// leaveDungeon 的回城序列首帧是 dungeon_leave_ack（对从未发生的 CMD42
		// 的应答），跳过；会话样板在此自行清理（不走主循环 settlement_exit_ack 钩子）。
		route, e := w.leaveDungeon()
		if e != nil {
			return nil, e
		}
		w.selectingDungeon = false
		w.approvedDungeonGate = 0
		w.deathSent = map[uint16]bool{}
		w.drops = nil
		w.completionSent = false
		w.completionErr = nil
		w.resultSent = false
		w.pendingTownArrival = nil
		w.leaveScene()
		w.activeDungeon = nil
		w.forest = nil // 终局完成，run 作废
		for _, pkt := range route {
			if pkt.Name != "dungeon_leave_ack" {
				plan = append(plan, pkt)
			}
		}
	}
	return plan, nil
}

// dispatchForest 是苏醒之森族的统一派发层：待机区组队（CMD12/13）→
// 终局剧情 191 → 家族命令（CMD2226/2045/2046 + 共信封内容 104）→
// 开战（CMD2043 内容 104）。必须排在 dispatchLegion（末世录共用信封）
// 之前——实机 2026-10-05 18:04 会话实证：96 频道的 CMD2043 此前被末世录
// 处理器抢答（ACK + N2895 内容 107），客户端对苏醒之森毫无反应。
func (client *gameConnection) dispatchForest(requestData *clientRequest) dispatchAction {
	if requestData.frame.Type != 1 || !client.bootstrapped || client.worldState == nil {
		return dispatchNext
	}
	w := client.worldState
	if !requestData.verified {
		return dispatchNext
	}
	handled, packets, err := w.forestStandbyPartyHandle(requestData.frame.ID, requestData.plaintext)
	if handled {
		if err != nil {
			client.event(map[string]any{"kind": "forest_party_request_rejected", "id": requestData.frame.ID, "error": err.Error()})
			packets = []outboundPacket{{"苏醒之森待机区队伍请求拒绝应答", 1, requestData.frame.ID, protocol.Refusal(8)}}
		}
		if client.sendPlan(packets, client.logCharacterResponse) != nil {
			return dispatchClose
		}
		return dispatchHandled
	}
	// 终局通关演出：CMD191（剧情暂停/恢复）在 finalDone 期间由苏醒之森
	// 应答（维纳斯同款，必须先于通用 191 处理器）。
	if requestData.frame.ID == 191 && w.forest != nil && w.forest.finalDone {
		packets, err := w.forestStoryPause(requestData.plaintext)
		if err != nil {
			client.event(map[string]any{"kind": "forest_story_refused", "id": requestData.frame.ID, "error": err.Error()})
			return dispatchHandled
		}
		if client.sendPlan(packets, client.logWorldResponseBody) != nil {
			return dispatchClose
		}
		return dispatchHandled
	}
	// 开战分流：CMD2043 内容 104（Normal）/ 105（Extreme，2026-10-06 00:09
	// 会话 forest_hard_start_observed 实锤；2026-10-08 官服抓包确认
	// Extreme 队伍的开战内容号就是 105）。末世录/伊斯/维纳斯各持
	// 107/101/106，互不相认。
	if requestData.frame.ID == legion.CmdStart && len(requestData.plaintext) >= legion.EnvelopeSize+4 {
		content := binary.LittleEndian.Uint32(requestData.plaintext[legion.EnvelopeSize:])
		hard := w.forestPartyHard
		switch {
		case content == legion.ForestHardContentID:
			// 内容 105 = Extreme（官服 2026-10-08 抓包实锤）。**不依赖建队标记**：
			// 建队帧丢失/重连时也要有人应答，否则客户端停在「开始作战无反应」。
			return client.dispatchForestStart(requestData, true)
		case hard && content == legion.ForestContentID:
			// Extreme 队伍却按 Normal 内容号开战（旧客户端形态 / 回 0x18 时代）：
			// 仍按 hard 连战走，副本集与状态包都由 run.hard 决定。
			return client.dispatchForestStart(requestData, true)
		case content == legion.ForestContentID:
			return client.dispatchForestStart(requestData, false)
		default:
			// 不是苏醒之森的内容号（104/105）：**必须放行**给后面的派发层。
			//
			// 实机教训（2026-10-08 13:57 会话，末世录）：这里原本记一条事件并回
			// 共享 ACK，于是内容 107（末世录）的 CMD2043 被苏醒之森吃掉 ——
			// 客户端收到 ACK 以为开战成功，服务端却从未进入 channel（日志里
			// 只有 `forest_start_unmatched`，没有 `legion_entered_channel`），
			// 后续 CMD2354 因为「还没有 CMD2043」被拒，开团流程整条卡死。
			// 内容号是各内容互不相认的标签，不属于本内容的请求不能代答。
			client.event(map[string]any{"kind": "forest_start_foreign_content", "id": requestData.frame.ID,
				"content": content, "hard": hard, "request_hex": fmt.Sprintf("%x", requestData.plaintext)})
			return dispatchNext
		}
	}
	// 家族命令：CMD2226 就绪/确认音符、CMD2045 进图、CMD2046 终局。
	// 内容 105（Extreme）与 104 走同一个处理器，模式由 w.forest.hard 决定。
	if isForestRequest(requestData.frame.ID, requestData.plaintext) || w.isForestHardFamilyRequest(requestData.frame.ID, requestData.plaintext) {
		plan, notes, err := w.handleForestRequest(requestData.plaintext, requestData.frame.ID)
		if err != nil {
			client.event(map[string]any{"kind": "forest_refused", "id": requestData.frame.ID,
				"reason": err.Error(), "request_hex": fmt.Sprintf("%x", requestData.plaintext)})
			return dispatchHandled
		}
		for _, note := range notes {
			note["id"] = requestData.frame.ID
			client.event(note)
		}
		if client.sendPlan(plan, client.logWorldResponseBody) != nil {
			return dispatchClose
		}
		// Extreme 进图是两段式：上面这一批只含 N2568（净化开始横幅 + 演出），
		// 挂起的进图帧列按 N2568 自带的 delay（4s）**精确**到期下发 —— 晚到会
		// 让客户端退回非无缝路径、界面态卡住（实机：进图后 UI 全丢）。
		if w.forestEntryPending != nil && client.connection != nil {
			client.connection.scheduleForestBanner(forestPurifyBannerSeconds)
			client.event(map[string]any{
				"kind":     "forest_purify_entry_scheduled",
				"id":       requestData.frame.ID,
				"delay_ms": forestPurifyBannerSeconds.Milliseconds(),
			})
		}
		return dispatchHandled
	}
	return dispatchNext
}

// dispatchForestStart 执行开战应答。两种模式各自的官服形状：
//
//   - Normal（内容 104）：ACK + N2563 等待态，2.8s 后 N2563 作战窗
//     （官服 21:42:03.375→21:42:06.202）。
//   - Extreme（内容 105）：ACK + **N2565 等待态**（64B），2.8s 后 **N2565
//     作战窗**（state2，三关副本号；官服 22:05:38.308→22:05:41.244）。
//     Extreme **没有** CMD2226 选音符步骤，客户端收到窗态后自行发 CMD2045。
func (client *gameConnection) dispatchForestStart(requestData *clientRequest, hard bool) dispatchAction {
	w := client.worldState
	plan, notes, err := w.startForest(requestData.plaintext, hard)
	if err != nil {
		client.event(map[string]any{"kind": "forest_refused", "id": requestData.frame.ID,
			"reason": err.Error(), "request_hex": fmt.Sprintf("%x", requestData.plaintext)})
		return dispatchHandled
	}
	for _, note := range notes {
		note["id"] = requestData.frame.ID
		client.event(note)
	}
	if client.sendPlan(plan, client.logWorldResponseBody) != nil {
		return dispatchClose
	}
	window, infoID := legion.ForestOperationWindowInfo(), legion.NotiForestInfo
	if hard {
		hardWindow, wErr := legion.ForestHardWindowInfo(0)
		if wErr != nil {
			client.event(map[string]any{"kind": "forest_operation_window_missing", "hard": true, "error": wErr.Error()})
			return dispatchHandled
		}
		window, infoID = hardWindow, legion.NotiForestHardInfo
	}
	if len(window) == 0 {
		client.event(map[string]any{"kind": "forest_operation_window_missing", "hard": hard})
		return dispatchHandled
	}
	roleID := w.role.ID
	var done <-chan struct{}
	if client.connection != nil {
		done = client.connection.done
	}
	go func() {
		timer := time.NewTimer(2800 * time.Millisecond)
		defer timer.Stop()
		select {
		case <-timer.C:
		case <-done:
			return
		}
		if client.output.send(0, infoID, window) != nil {
			return
		}
		client.event(map[string]any{"kind": "forest_operation_window_pushed", "id": infoID, "character_id": roleID, "hard": hard})
	}()
	return dispatchHandled
}

// isForestStartRequest 判别苏醒之森开战命令：CMD2043 信封 @13 的内容号
// 104（末世录 107 / 伊斯 101 / 维纳斯 106 同信封分流）。
func isForestStartRequest(id uint16, p []byte) bool {
	if id != legion.CmdStart || len(p) < legion.EnvelopeSize+4 {
		return false
	}
	return binary.LittleEndian.Uint32(p[legion.EnvelopeSize:]) == legion.ForestContentID

}

// startForest handles CMD2043 (开始作战).
//
// Normal（内容 104）官服顺序（2026-10-02 21:42:03）：N2254 → N2563 waiting →
// CMD2043 ACK；共享 ACK 读取器只消费 01+4B，N2563 等待态才是内容对象渲染的
// 数据源，所以计划是「共享 ACK + 官服等待向量」，作战窗由 dispatchForestStart
// 的延迟推送补上。守卫同维纳斯：必须先有 CMD12 建队、且没有活动副本。
//
// Extreme（hard=true，内容 105）用**官服 2026-10-08 抓包**的原文：
// CMD2043 ACK（16B 官服形）+ N2565 等待态（64B）；窗态/清关 tick/终局态全部
// 走 N2565，奖励走官服七行 N2252。内容号以 105 为准，104 仍容忍（旧客户端
// 若按队伍类型发 104，不因此拒绝整场）。
func (w *worldSession) startForest(p []byte, hard bool) ([]outboundPacket, []map[string]any, error) {
	if len(p) < legion.EnvelopeSize+4 {
		return nil, nil, fmt.Errorf("forest start payload %d bytes, want at least %d", len(p), legion.EnvelopeSize+4)
	}
	content := binary.LittleEndian.Uint32(p[legion.EnvelopeSize:])
	if hard {
		if content != legion.ForestHardContentID && content != legion.ForestContentID {
			return nil, nil, fmt.Errorf("forest hard start content %d, want %d or %d", content, legion.ForestHardContentID, legion.ForestContentID)
		}
	} else if content != legion.ForestContentID {
		return nil, nil, fmt.Errorf("forest start content %d, want %d", content, legion.ForestContentID)
	}
	if w.activeDungeon != nil {
		return nil, nil, fmt.Errorf("forest start inside an active dungeon")
	}
	if !w.soloPartyReady {
		return nil, nil, fmt.Errorf("forest start before the standby party was created (no CMD12)")
	}
	// 每次开战都是一场新挑战（官服 21:42/21:43/21:44 三轮 CMD2043 语义）；
	// 重复开战覆盖旧 run（与维纳斯 startVenus 同口径）。挂起的进图帧列一并作废。
	w.forestEntryPending = nil
	w.forest = &forestRun{choice: 0xff, hard: hard}
	if hard {
		return []outboundPacket{
				{"forest_hard_start_ack", 1, legion.CmdStart, legion.ForestHardStartAck()},
				{"forest_hard_info_waiting", 0, legion.NotiForestHardInfo, legion.ForestHardWaitingInfo()},
			}, []map[string]any{{
				"kind":         "forest_started",
				"character_id": w.role.ID,
				"hard":         true,
				"content":      content,
			}}, nil
	}
	return []outboundPacket{
			{"forest_start_ack", 1, legion.CmdStart, legion.StartAck()},
			{"forest_info_waiting", 0, legion.NotiForestInfo, legion.ForestWaitingInfo()},
		}, []map[string]any{{
			"kind":         "forest_started",
			"character_id": w.role.ID,
			"hard":         false,
			"content":      content,
		}}, nil
}
