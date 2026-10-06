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
		// action3 清空八个成员槽。苏醒之森尚无伊斯式重复次数恢复包。
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
		w.soloPartyReady = false
		w.forestPartyHard = false
		return true, []outboundPacket{
			{"苏醒之森队伍解散", 0, 9, protocol.BlackPurgatoryPartyGone(w.characters.ChannelContext)},
		}, nil
	}
	request, err := protocol.DecodeForestStandbyParty(p)
	if err != nil {
		return fail(err)
	}
	// Extreme 建队应答仍用 Normal 队伍类型 0x18（第十四轮）：Extreme 类型
	// 0x19 的队伍会让客户端在进图确认时做「成员集结区」本地检查（Extreme
	// 集结区 = town198 area3，Normal = area2），单人 bootstrap 队伍的区域
	// 数据为空 → 永远弹「party members have not gathered」(21:30 会话实证)。
	// 回 0x18 让检查按 Normal 规则放行；run.hard 仍把进图副本换成 Extreme
	// 三关。代价：客户端队伍 UI 显示为普通军团队伍（纯外观）。
	party, err := protocol.ForestStandbyPartyReply(request.Name, w.role.WireID, w.characters.ChannelContext, false)
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
	// hard 标记 Extreme（ForestOfAwakeningHard）挑战：建队类型 0x19 带入。
	// 硬模式的开战内容号/状态 NOTI（2565）尚未取证，本轮仅承载标记。
	hard bool
	// potionUsed[stage] 是本关已使用的消耗品次数（CMD44）：军团口径每关
	// 限 8 次（用户要求），进图时清零、超限拒绝（UseStackableRefused）。
	potionUsed [3]int
}

// isForestRequest 判别苏醒之森族请求：CMD2226/2227 为本族专属；
// CMD2043/2045/2046 与末世录/伊斯/维纳斯共用信封，由 body @13 的内容号
// 104 区分。
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

// enterForestStage handles CMD2045 (content 104 Normal / 105 Extreme): load
// the stage dungeon and run the standard entry frame sequence. 阶段副本号按
// run.hard 取 Normal/Extreme 两套（官服 N2563 作战窗 @27/@39/@51 与 PVF
// hermitage_extreme_1/2/3.dgn）；苏醒之森副本无维纳斯式事件怪缺口，不注入
// 载体怪（实机若地图无怪再按 18:04 会话取证）。Extreme 连战无选音符步骤，
// 不要求 choice 确认。
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
	// 第 3/3 次尝试：Extreme run 也走 Normal 包链（CMD2045 内容 104），内容
	// 号不再作模式判定——进图副本只按 run.hard 选。
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
	// 不是 dungeonEntryPlanImpl 默认的 1B 成功字节。
	frames[0].Name = "forest_enter_ack"
	frames[0].Payload = legion.EnterDungeonAck()
	// N28 之前补 N3（角色状态 → 副本态）与 N27（选图上下文），维纳斯/
	// SemiRaid 军团进图序列同款。
	actorState, se := protocol.UserState(w.role.WireID, protocol.UserStateDungeon)
	if se != nil {
		return nil, nil, se
	}
	inserted := false
	plan := make([]outboundPacket, 0, len(frames)+3)
	for _, pkt := range frames {
		if pkt.ID == 28 && !inserted {
			plan = append(plan,
				outboundPacket{"forest_actor_state_dungeon", 0, 3, actorState},
				outboundPacket{"forest_dungeon_selection", 0, 27, protocol.EnterDungeonSelection()},
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
	// 会话样板与主循环/SemiRaid 门一致。
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
	run.stage = stage
	run.potionUsed[stage] = 0
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
	// Extreme 连战（第十六轮，用户按国服视频确认的口径）：非终点关打完不结算
	// （无横幅/翻牌/选音符），只发该关 cleared 状态——官服 Normal 实测客户端
	// 收到 cleared N2563 后 32ms 自动发 CMD46（21:43:02.294→.326，人来不及点
	// 翻牌 UI），forestResult 据此用「已选状态（类型 04 记录）」应答——那是
	// 官服实证的客户端自动 CMD2062 直进触发器（21:44:32.422→34.387）。
	// 终点关保持完整结算链（横幅/翻牌/视频/回城 = 用户要的终局形态）。
	if run.hard && stage < len(legion.ForestStageDungeons)-1 {
		if run.cleared[stage] {
			return nil, nil // 一次性投影（venus 同款守卫）
		}
		clearedInfo, err := legion.ForestClearedInfo(stage)
		if err != nil {
			return nil, err
		}
		run.cleared[stage] = true
		return []outboundPacket{
			{"forest_hard_stage_cleared", 0, legion.NotiForestInfo, clearedInfo},
		}, nil
	}
	clearBanner, err := legion.ForestStageClearEnabled(stage)
	if err != nil {
		return nil, err
	}
	clearedInfo, err := legion.ForestClearedInfo(stage)
	if err != nil {
		return nil, err
	}
	// 通关奖励表（第二十一轮，用户方案）：Normal 终局 = N2252 四条（材料 +
	// 3 件随机魔器/神器装备）+ N2253 五条；Extreme 终局 = N2252 十一条 +
	// N2253 一条；Normal 第 1/2 关 = N2252 token-only + N2253 三条。
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
	var gear []uint32
	for _, item := range basicItems {
		if item.Template == 0 {
			gear = append(gear, 0)
		}
	}
	if len(gear) != 0 {
		gear = legion.VenusRollFlipGear(venusFlipGearPool, len(gear))
	}
	flip, err := legion.ForestBasicClearReward(stage, basicItems, gear)
	if err != nil {
		return nil, err
	}
	additional, err := legion.ForestAdditionalReward(additionalItems)
	if err != nil {
		return nil, err
	}
	var plan []outboundPacket
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
	w.activeDungeon = nil
	w.selectingDungeon = false
	w.approvedDungeonGate = 0
	w.drops = nil
	w.deathSent = nil
	w.completionSent = false
	w.resultSent = false
	w.resetCards()
	w.forest = &forestRun{choice: 0xff}
	if event != nil {
		event(map[string]any{"kind": "forest_stage_timeout", "character_id": w.role.ID,
			"stage": stage, "started": started.Unix(), "limit": int(limit / time.Second)})
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
	return append(packets, outboundPacket{"forest_info_timeout_waiting", 0, legion.NotiForestInfo, legion.ForestWaitingInfo()})
}

// forestResult 处理清关后的 CMD46（dungeonResult 的苏醒之森分支）：通用
// 结算（N34/N35/N261+8张牌）不适用。Normal 应答 = 下一关作战窗（state6，
// 官服 21:43:02.340）；终局关无下一窗，回 cleared 态（官服 21:45:25.262）。
// 官服 CMD46 后客户端自行完成场景切换，无回城包链；服务端在此静默收尾
// 副本会话。Extreme 连战无 CMD46 环节样本（过段走清怪投影 + CMD2062），
// 若到达此分支按 N2565 状态推进（第 1/3 次尝试值）。
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
	var info []byte
	name := "forest_info_next_window"
	switch {
	case terminal:
		clearedBody, err := legion.ForestClearedInfo(stage)
		if err != nil {
			return nil, err
		}
		info, name = clearedBody, "forest_info_cleared"
	case run.hard:
		// 连战：应答「已选状态（下一关 + 类型 04 记录）」——客户端收到后自动
		// CMD2062 直进下一关（官服 21:44:32→34 实证的触发器），不回待机区、
		// 不弹选音符窗。2062 目标是 Normal 副本号（客户端按窗口记录解析），
		// enterForestStageDirectMove 按 run.hard 换成 Extreme 副本。
		chosen, cerr := legion.ForestChosenInfo(next, 6)
		if cerr != nil {
			return nil, cerr
		}
		info = chosen
		name = "forest_hard_info_direct_next"
	default:
		var err error
		info, err = legion.ForestWindowInfo(next)
		if err != nil {
			return nil, err
		}
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
	return []outboundPacket{{name, 0, legion.NotiForestInfo, info}}, nil
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
	return s, append(dungeonSelectionHead(), frames...), nil
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
		plan = append(plan, outboundPacket{"forest_info_leave", 0, legion.NotiForestInfo, legion.ForestLeaveInfo()})
		// 第十八轮：连战终局视频播完即**自动回城**（用户实测 Normal 式的
		// 「视频后玩家点返回城镇」在连战里不出现——客户端连战态没有该按钮，
		// 0254 会话实证；照维纳斯终局自动回城先例）。leaveDungeon 的回城
		// 序列首帧是 dungeon_leave_ack（对从未发生的 CMD42 的应答），跳过；
		// 会话样板在此自行清理（不走主循环 settlement_exit_ack 钩子）。
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
	// 会话 forest_hard_start_observed 实锤）。末世录/伊斯/维纳斯各持
	// 107/101/106，互不相认。
	if requestData.frame.ID == legion.CmdStart && len(requestData.plaintext) >= legion.EnvelopeSize+4 {
		content := binary.LittleEndian.Uint32(requestData.plaintext[legion.EnvelopeSize:])
		hard := w.forestPartyHard
		switch {
		case !hard && content == legion.ForestContentID:
			return client.dispatchForestStart(requestData, false)
		case hard && content == legion.ForestContentID:
			// Extreme 建队应答为 0x18 队伍（集结检查规避），开战随之以
			// Normal 内容号 104 到达——副本集仍按 run.hard 走 Extreme。
			return client.dispatchForestStart(requestData, true)
		case hard && content == legion.ForestHardContentID:
			// 若客户端以 105 开战（未来形状），同样按 hard 连战处理。
			return client.dispatchForestStart(requestData, true)
		default:
			// 观测：内容号与队伍模式不匹配（含 hard 的其它内容号）——记录后
			// 回共享 ACK，禁止把未知内容喂给任一流程。
			client.event(map[string]any{"kind": "forest_start_unmatched", "id": requestData.frame.ID,
				"content": content, "hard": hard, "request_hex": fmt.Sprintf("%x", requestData.plaintext)})
			if client.sendPlan([]outboundPacket{{"forest_start_unmatched_ack", 1, legion.CmdStart, legion.StartAck()}}, client.logWorldResponseBody) != nil {
				return dispatchClose
			}
			return dispatchHandled
		}
	}
	// 家族命令：CMD2226 就绪/确认音符、CMD2045 进图、CMD2046 终局。
	if isForestRequest(requestData.frame.ID, requestData.plaintext) {
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
		return dispatchHandled
	}
	return dispatchNext
}

// dispatchForestStart 执行开战应答。Extreme（hard）按连战口径：ACK +
// N2565 等待态（阶段未定）+ 2.8s 后 N2565 就绪态（state 2、阶段 0，镜像
// Normal 的两段节奏；硬模式无选音符步骤，客户端应自行发 CMD2045 内容 105
// 进第一关）。Normal 保持 1-8 轮的已验证流程。
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
	// 状态二段推（第 3/3 次尝试决策）：两种模式都走已实机验证的 Normal
	// 包链（等待态→窗态→CMD2226 确认→进图），只把进图副本按 run.hard 换成
	// Extreme 三关。N2565 家族同构两次尝试均被客户端无视（第 1/3：自构体；
	// 第 2/3：官服等待原文+win0 换副本），第 3 次不再猜硬模式专属形状，
	// 与官服的偏差（Extreme 多一个选音符窗、状态走 N2563）记录于日志。
	window, infoID := legion.ForestOperationWindowInfo(), legion.NotiForestInfo
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

// startForest handles CMD2043 (开始作战). Official order (2026-10-02
// 21:42:03): N2254 → N2563 waiting → CMD2043 ACK; the shared ACK reader
// consumes 01+4B and the N2563 waiting snapshot is what the forest content
// object renders, so the plan sends the shared 5B ACK followed by the
// official waiting vector. The operation window itself arrives via the
// delayed push in dispatchForestStart. Guards mirror venus: a standby party
// (CMD12) must exist and no dungeon may be active.
// Extreme（hard=true，内容 105）：第 3/3 次尝试——放弃 N2565 猜测，改用与
// Normal 完全相同的已验证包链（倒计时/横幅/选音符窗全部可用），差异只在
// 进图副本集（run.hard → hermitage_extreme_1/2/3）。与官服连战口径的偏差
// 见第十二轮日志。
func (w *worldSession) startForest(p []byte, hard bool) ([]outboundPacket, []map[string]any, error) {
	if len(p) < legion.EnvelopeSize+4 {
		return nil, nil, fmt.Errorf("forest start payload %d bytes, want at least %d", len(p), legion.EnvelopeSize+4)
	}
	content := binary.LittleEndian.Uint32(p[legion.EnvelopeSize:])
	// 第十五轮修正：Extreme 建队应答为 0x18 队伍（集结检查规避），开战内容号
	// 随之为 104；105 保留（若客户端以 hard 内容号开战）。两种内容号在 hard
	// run 下都放行，副本集由 run.hard 决定。
	if hard {
		if content != legion.ForestContentID && content != legion.ForestHardContentID {
			return nil, nil, fmt.Errorf("forest hard start content %d, want %d or %d", content, legion.ForestContentID, legion.ForestHardContentID)
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
	// 重复开战覆盖旧 run（与维纳斯 startVenus 同口径）。
	w.forest = &forestRun{choice: 0xff, hard: hard}
	return []outboundPacket{
			{"forest_start_ack", 1, legion.CmdStart, legion.StartAck()},
			{"forest_info_waiting", 0, legion.NotiForestInfo, legion.ForestWaitingInfo()},
		}, []map[string]any{{
			"kind":         "forest_started",
			"character_id": w.role.ID,
		}}, nil
}
