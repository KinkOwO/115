package main

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"dfolan/internal/catalog"
	"dfolan/internal/database"
	"dfolan/internal/dungeon"
	"dfolan/internal/game/protocol"
	"dfolan/internal/inventory"
	"dfolan/internal/legion"
	"dfolan/internal/loot"
)

// 维纳斯军团（频道 Type 99 / 内容 106）在重构后分发架构上的挂接层。
//
// 已实现：待机区队伍对话框（CMD12 建队 / CMD13 离队，2026-10-04 实机
// 验证通过）、开战流程（CMD2043 开始 → CMD2290 作战选择 → CMD2045 进图，
// 阶段推进）、放弃（CMD2044）与重选开窗（CMD2290 Action4，更改难度路径）。
// 副本为标准迷宫（Venus_1Phase..4Phase.dgn），走通用进图/战斗/结算机制；
// 第一阶段的三只圣物怪是事件怪，由 stageVenusPhaseCarriers 登记花名册、
// venusDynamicSpawnPackets 在 C37 加载完成后用 N2194 注册（源图无 [monster]
// 段，事件怪不能编进 N29 固定行）。副本内右上角「撤退」按钮走 CMD72，由
// settlementExit 的维纳斯分支应答（照伊斯大陆先例）；撤退/放弃把 run 复位
// 成全新未选状态并垫等待态 N2655，待机区可不经门弹窗直接 Open 重选重进。
// 每关的阶段倒计时由 venusStageTimer 以 NOTI1474（DUNGEON_TIMEOUT_TIME，
// 8B [时限秒, 冻结开始秒]）在加载完成应答里同步：第 1..3 关 600 秒、降临
// 第 4 关 900 秒（VenusPhaseLimits，不得超过客户端 DGN 源上限，见 venus.go
// 第四十四轮注释）。
//
// 仍显式拒绝（不回构造包）：CMD2293（降临中止领奖）、CMD2046 之后的终局
// 结算分支。家族命令必须在 dispatchLegion 之前拦截——CMD2043/2045/2046 与
// 末世录（内容 107）共用信封，放行会把内容 107 的状态包灌进频道 99 的
// 客户端（N2895/N2655 不通用）。
//
// 调用侧：client_dispatch.go（beforeClientTypeDispatch，dispatchIspins 之后）。

// venusFlipGearPool 是维纳斯终局翻牌第一排随机装备位池子
// （configs/venus-flip-gear.generated.json，115 级魔法/神器常规部位，
// bootstrap 装载）。nil/空 = 池子不可用，第一排降级为仅材料位。
var venusFlipGearPool []uint32

// venusRun 是一次维纳斯挑战的会话状态（会话级，不落存档，同 ispins）。
type venusRun struct {
	// choice 是 CMD2290 Action2 确认的作战难度：0/1 普通、2 降临；
	// 0xff = 尚未选择（匹配 4 上游已拒绝）。
	choice byte
	// stage 是当前所在（或下一个待进）的阶段号 0..3。
	stage int
	// cleared 记录已通关阶段；CMD2046 是权威的「该阶段结算完成」信号。
	cleared [4]bool
	// relicMask 是本次挑战的七圣物位集合（N2655 @77，bit0..6）；
	// CMD2291 报告首次领取时置位，跨阶段保留，不叠加。
	relicMask uint32
	// pending 是已进房间花名册、等客户端 C37 加载完成后用 N2194 注册的
	// 动态圣物怪（首关事件怪不能编进 N29 固定行，见 stageVenusPhaseCarriers）。
	pending []protocol.UnassignedMonster115
	// flipGear 是终局翻牌第一排随机装备位的本次 roll 结果（会话级）：
	// completeVenusStage 时 roll 一次，N2252 展示、Awarder 入库与 cardPlan
	// 三处共用，保证翻牌界面与背包一致。
	flipGear []uint32
	// potionUsed[stage] 是本关已使用的消耗品次数（CMD44）：军团口径每关
	// 限 8 次（用户要求，参考伊斯），进图时清零、超限拒绝不扣库存。
	potionUsed [4]int
	// stageClock 是各阶段倒计时的开始时刻（venusStageTimer 冻结）：阶段
	// 第一次真实加载完成时取当时服务器秒，同阶段重载/换房复用原值——1474
	// 规格文档「不能以当前发送时间覆盖阶段最初开始时间」，否则每次发包
	// 都在续时。
	stageClock [4]time.Time
	// windowDeadline 是难度选择窗的截止时刻（CMD2290 Action1/4 的 ACK 里
	// 下发的同一个截止秒）：归 0 后由 venusOperationClose 推原生 close ACK
	// 自动关窗（2290 ACK @5 close=1），清零表示窗已关、无需再推。
	windowDeadline time.Time
	// finalDone/storyFinished 标记终局流程：终局 CMD2046 置 finalDone（客户
	// 端随即播放通关视频），视频的 CMD191 恢复置 storyFinished（随后关右上
	// 角面板 + 回城 + 作废 run——通关后 Open 不得再进图）。
	finalDone     bool
	storyFinished bool
	// entered 标记本场挑战是否进过图（CMD2045/2062 任一成功入场）：难度锁
	// 的判据——进图前（确认了难度但还没进）变更提示里的「变更难度」仍可重
	// 选（action4 重开三卡窗，13:33 会话实机路径），进图后锁定（用户口径：
	// 已选择的难度无法在中途变更）。撤退/超时的进度保留路径置回 true。
	entered bool
}

// resetRun 把 run 复位成「已开战、未选难度」的全新状态（choice FF、stage 0、
// cleared 清空、待注册动态怪清空）。撤退/放弃走这里而不是作废 run：run 留着，
// 待机区的 Open/选难度/进图链路无需再按门弹窗就能继续，客户端由随后的等待态
// N2655 复位作战窗口（否则客户端带着已选难度残留，回城即自动弹难度窗）。
func (r *venusRun) resetRun() {
	r.choice = 0xff
	r.stage = 0
	r.cleared = [4]bool{}
	r.relicMask = 0
	r.pending = nil
	r.flipGear = nil
	r.stageClock = [4]time.Time{}
	r.windowDeadline = time.Time{}
	r.entered = false
}

// clearedCount 返回已通关阶段数，即下一个待进阶段的序号。
func (r *venusRun) clearedCount() int {
	count := 0
	for _, ok := range r.cleared {
		if ok {
			count++
		}
	}
	return count
}

// isVenusRequest 判别维纳斯族请求：CMD2290/2293 为本族专属；CMD2043/2045/
// 2046 与末世录/伊斯共用信封，由 body @13 的内容号 106 区分。
func isVenusRequest(id uint16, p []byte) bool {
	if legion.VenusRequests(id) {
		return true
	}
	if !legion.Requests(id) || len(p) < legion.EnvelopeSize+4 {
		return false
	}
	return binary.LittleEndian.Uint32(p[legion.EnvelopeSize:]) == legion.VenusContentID
}

// venusStandbyPartyHandle 实现维纳斯待机区队伍对话框：CMD12 建队、
// CMD13 离队。请求与伊斯待机区同构、仅队伍类型 0x22，应答照黑鸦/伊斯
// 先例：队长资料两个 op=2 先行 + 单帧 NOTI9。玩法边界同伊斯：单人
// bootstrap 队伍（Party==1 归一化 65535），建队后 arm soloPartyReady。
func (w *worldSession) venusStandbyPartyHandle(id uint16, p []byte) (bool, []outboundPacket, error) {
	if w.channelType != 99 || w.role.ID == 0 || (id != 12 && id != 13) {
		return false, nil, nil
	}
	fail := func(err error) (bool, []outboundPacket, error) { return true, nil, err }
	if w.characters == nil {
		return fail(fmt.Errorf("维纳斯待机区角色服务不可用"))
	}
	if id == 13 {
		// 离队与伊斯同款：原生 CMD13 body 为空或 8B 零填充；NOTI9
		// action3 清空八个成员槽。维纳斯没有伊斯式重复次数恢复包。
		if len(p) != 0 && len(p) != 8 {
			return fail(fmt.Errorf("维纳斯离队请求长度无效"))
		}
		for _, b := range p {
			if b != 0 {
				return fail(fmt.Errorf("不支持的维纳斯离队选项"))
			}
		}
		if w.activeDungeon != nil {
			return fail(fmt.Errorf("请先返回待机区再退出维纳斯队伍"))
		}
		w.soloPartyReady = false
		w.venus = nil
		return true, []outboundPacket{
			{"维纳斯队伍解散", 0, 9, protocol.BlackPurgatoryPartyGone(w.characters.ChannelContext)},
		}, nil
	}
	name, err := protocol.DecodeVenusStandbyParty(p)
	if err != nil {
		return fail(err)
	}
	party, err := protocol.VenusStandbyPartyReply(name, w.role.WireID, w.characters.ChannelContext)
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
	return true, []outboundPacket{
		{"维纳斯队长资料", 0, 2, basic},
		{"维纳斯队长详细资料", 0, 2, detail},
		{"维纳斯待机区队伍创建", 0, 9, party},
	}, nil
}

// handleVenusRequest answers one Venus family command (CMD2043/2045/2046
// content 106, CMD2290).
func (w *worldSession) handleVenusRequest(p []byte, id uint16) ([]outboundPacket, []map[string]any, error) {
	switch id {
	case legion.CmdStart:
		return w.startVenus(p)
	case legion.CmdVenusOperationSelect:
		return w.venusOperation(p)
	case legion.CmdEnterDungeon:
		return w.enterVenusStage(p)
	case legion.CmdRewardEnd:
		return w.venusRewardEnd(p)
	case legion.CmdFail:
		return w.venusFail(p)
	case legion.CmdVenusRelic:
		return w.venusRelic(p)
	}
	return nil, nil, fmt.Errorf("venus opcode %d is not implemented (content 106)", id)
}

// startVenus handles CMD2043 (开始作战). ACK = family shared 01+4B; the
// waiting-state N2655 (State2/ChoiceFF/Stage0/首目标0) is what puts the
// client into the Venus operation screen.
func (w *worldSession) startVenus(p []byte) ([]outboundPacket, []map[string]any, error) {
	if err := legion.DecodeVenusStart(p); err != nil {
		return nil, nil, err
	}
	if w.activeDungeon != nil {
		return nil, nil, fmt.Errorf("venus start inside an active dungeon")
	}
	if !w.soloPartyReady {
		return nil, nil, fmt.Errorf("venus start before the standby party was created (no CMD12)")
	}
	// BUG3（第二十二轮）：撤退/失败后重开保留 cleared/stage 进度（从撤退
	// 或失败的下一关继续）。第三十三轮起难度一并锁定（2290 规格：官网口径
	// 开局后不可改档；官服 C72 返回等待区保留原run、难度及遗物）：难度在锁
	// （choice≠FF，进图前置保证开战过）的进行中挑战保留 choice，waiting 向
	// 量带权威已选难度——客户端按钮意图 getter 按 Choice≠FF 直接进「变更提
	// 示」（已选择X。确定要进入吗？），不再弹三卡片重选窗；难度不在锁（未
	// 选过/预选未进图后主动放弃）才回未选重开。
	if w.venus == nil || w.venus.finalDone {
		w.venus = &venusRun{choice: 0xff}
	} else {
		w.venus.flipGear = nil
		w.venus.windowDeadline = time.Time{}
	}
	waiting := legion.VenusWaitingInfo()
	if w.venus.choice != 0xff {
		waiting = legion.VenusChosenInfo(w.venus.choice, w.venus.clearedCount())
	} else if kept := w.venus.clearedCount(); kept > 0 {
		waiting = legion.VenusReopenInfo(kept)
	}
	return []outboundPacket{
			{"venus_start_ack", 1, legion.CmdStart, legion.StartAck()},
			{"venus_info_waiting", 0, legion.NotiVenusInfo, waiting},
		}, []map[string]any{{
			"kind":         "venus_started",
			"character_id": w.role.ID,
		}}, nil
}

// venusOperation handles CMD2290: action 1 opens the difficulty window
// (deadline = now + venus.cos 60s), action 2 confirms the choice and must be
// preceded by the chosen-state N2655 so the native confirm callback enters
// the next stage with an authoritative selection.
func (w *worldSession) venusOperation(p []byte) ([]outboundPacket, []map[string]any, error) {
	req, err := legion.DecodeVenusOperation(p)
	if err != nil {
		return nil, nil, err
	}
	run := w.venus
	if run == nil {
		return nil, nil, fmt.Errorf("venus operation before start (no CMD2043 yet)")
	}
	if w.activeDungeon != nil {
		return nil, nil, fmt.Errorf("venus operation inside a dungeon")
	}
	switch req.Action {
	case 1:
		// 第三十三轮：进过图的挑战难度已锁（2290 规格：客户端只在未选择时
		// 才发 Action1 开三卡窗；已选时按钮意图直接走变更提示，正常流不会
		// 到这里）。万一收到也不得重开自由选窗——那会复现「重选低难度 →
		// 终点回退 → 越终点拒绝」类状态分裂（172342 会话实证），回原生
		// 公共负包（141C3D3D0 清 2290 待答、不触发开窗回调，UI 不卡死）。
		if run.entered {
			return []outboundPacket{{"venus_operation_locked_refused", 1, legion.CmdVenusOperationSelect, protocol.Refusal(4)}},
				[]map[string]any{{"kind": "venus_operation_locked", "character_id": w.role.ID, "action": req.Action}}, nil
		}
		deadline := uint32(time.Now().Unix()) + legion.VenusSelectionSeconds
		// 窗口截止时刻记进 run：归 0 后由 venusOperationClose 推原生 close ACK
		// 自动关窗（客户端自己对 0 不做任何事，实测 2026-10-05）。
		run.windowDeadline = time.Unix(int64(deadline), 0)
		return []outboundPacket{{
				"venus_operation_open_ack", 1, legion.CmdVenusOperationSelect,
				legion.VenusOperationAck(1, false, deadline, 0),
			}}, []map[string]any{{
				"kind":         "venus_operation_opened",
				"character_id": w.role.ID,
				"deadline":     deadline,
			}}, nil
	case 2:
		if req.Choice > 2 {
			return nil, nil, fmt.Errorf("venus choice %d is not a supported difficulty (0/1 normal, 2 descent)", req.Choice)
		}
		// 锁定期间的换档确认同样拒绝（同 Choice 的重复确认放行——那是变更
		// 提示里「进入地下城」的原生子模式路径）。
		if run.entered && req.Choice != run.choice {
			return []outboundPacket{{"venus_operation_locked_refused", 1, legion.CmdVenusOperationSelect, protocol.Refusal(4)}},
				[]map[string]any{{"kind": "venus_operation_locked", "character_id": w.role.ID,
					"action": req.Action, "locked": run.choice, "choice": req.Choice}}, nil
		}
		run.choice = req.Choice
		// 确认即原生关窗：清掉截止时刻，venusOperationClose 不再推 close。
		run.windowDeadline = time.Time{}
		// BUG3 回归：chosen 态携带下一个待进阶段（撤退/失败保留进度后
		// 重开，C2045 确认的不是第 0 关而是 clearedCount）。
		return []outboundPacket{
				{"venus_info_chosen", 0, legion.NotiVenusInfo, legion.VenusChosenInfo(req.Choice, run.clearedCount())},
				{"venus_operation_confirm_ack", 1, legion.CmdVenusOperationSelect, legion.VenusOperationAck(2, false, 0, 0)},
			}, []map[string]any{{
				"kind":         "venus_operation_confirmed",
				"character_id": w.role.ID,
				"choice":       req.Choice,
				"endpoint":     legion.VenusEndpoint(req.Choice),
			}}, nil
	case 4:
		// 重选开窗（更改难度 → 弹窗确定）：客户端发 action4+choiceFF。
		// 先回权威的未选择 N2655（清掉旧难度，客户端窗口回到三卡片选择态），
		// 再回带新截止时间的 action4 ACK——2026-10-04 13:33 会话实机：此前的
		// 静默拒绝让客户端作战窗口状态机卡死，卡片点不动、后续点 Open 无上行。
		if w.activeDungeon != nil {
			return nil, nil, fmt.Errorf("venus operation inside a dungeon")
		}
		// 第三十三轮：进过图的挑战难度锁定，变更提示里的「变更难度」按钮由
		// 客户端按权威状态置灰不给点；万一收到重选请求也只回负包（不清锁、
		// 不重开三卡窗——否则复现 172342 会话的难度/终点分裂）。
		if run.entered {
			return []outboundPacket{{"venus_operation_locked_refused", 1, legion.CmdVenusOperationSelect, protocol.Refusal(4)}},
				[]map[string]any{{"kind": "venus_operation_locked", "character_id": w.role.ID, "action": req.Action}}, nil
		}
		run.choice = 0xff
		deadline := uint32(time.Now().Unix()) + legion.VenusSelectionSeconds
		run.windowDeadline = time.Unix(int64(deadline), 0)
		return []outboundPacket{
				{"venus_info_reopened", 0, legion.NotiVenusInfo, legion.VenusReopenInfo(run.stage)},
				{"venus_operation_reopen_ack", 1, legion.CmdVenusOperationSelect, legion.VenusOperationAck(4, false, deadline, 0)},
			}, []map[string]any{{
				"kind":         "venus_operation_reopened",
				"character_id": w.role.ID,
				"stage":        run.stage,
				"deadline":     deadline,
			}}, nil
	}
	return nil, nil, fmt.Errorf("venus operation action %d unknown", req.Action)
}

// stageVenusPhaseCarriers 准备第一阶段的动态圣物怪注册。venus_1phase.map 完全
// 没有 [monster] 段（三只是事件怪：图内 [event monster position] 四槽、
// venus.cos 圣物表、2291 文档「首关三怪生成」），而 N29 的怪物行是固定 PVF
// 行——客户端按 SourceIndex 找不到源行就不会创建/落点（2026-10-04 13:52 实机：
// N29 带三行、客户端画面无怪；月湖先例 moon_room.go 同一结论：动态怪必须走
// N2194）。因此：花名册照常登记三只（死亡/清房/2291 校验用），N29 保持空单，
// 待注册行挂在 run.pending，由 venusDynamicSpawnPackets 在真实 C37 后注册。
// 坐标取源图事件位槽（四槽同在 373,372）；源图缺失事件位时不登记花名册，
// 退化为可通行空房（门开、可直接走到终点房）并返回 nil 由调用方记诊断。
func stageVenusPhaseCarriers(c catalog.DungeonCatalog, s *dungeon.Session, stage int) []protocol.UnassignedMonster115 {
	if s == nil || stage != 0 || len(s.Monsters) > 0 || !legion.IsVenusStageDungeon(s.Definition.ID) {
		return nil
	}
	script, err := c.MapScript(s.Room.Map)
	if err != nil {
		return nil
	}
	positions := venusEventPositions(script)
	if len(positions) == 0 {
		return nil
	}
	level := byte(s.Definition.BasisLevel)
	pending := make([]protocol.UnassignedMonster115, 0, len(legion.VenusPhase1Carriers))
	for i, template := range legion.VenusPhase1Carriers {
		if s.NextEntity == 0 || s.NextEntity >= 65535 {
			break
		}
		position := positions[i%len(positions)]
		pending = append(pending, protocol.UnassignedMonster115{
			Grid:     [2]byte{s.Room.X, s.Room.Y},
			Entity:   s.NextEntity,
			Template: template,
			X:        position[0],
			Y:        position[1],
		})
		s.Monsters = append(s.Monsters, protocol.DungeonMonster{
			Entity:   s.NextEntity,
			Level:    level,
			Template: template,
			Rank:     0,
			Team:     100,
		})
		s.NextEntity++
	}
	if len(pending) == 0 {
		return nil
	}
	s.Visited[s.Room.Map] = s.Monsters
	return pending
}

// venusEventPositions 读地图脚本的 [event monster position] 槽（x y z 三元组）。
func venusEventPositions(script catalog.ScriptRecord) [][2]int32 {
	var out [][2]int32
	active := false
	for i := 0; i < len(script.Cells); i++ {
		cell := script.Cells[i]
		if cell.Type == 3 {
			active = cell.Text == "[event monster position]"
			continue
		}
		if !active || cell.Type != 0 || i+2 >= len(script.Cells) {
			continue
		}
		y, z := script.Cells[i+1], script.Cells[i+2]
		if y.Type != 0 || z.Type != 0 {
			continue
		}
		out = append(out, [2]int32{cell.Value, y.Value})
		i += 2
	}
	return out
}

// venusDynamicSpawnPackets 在副本加载完成（CMD37）应答里注册待注册的动态圣物
// 怪：N2194 逐只携带房间格点与源图事件位坐标，客户端在房间加载后即时建怪。
// 只注册与当前房间匹配的行，注册即从 pending 清除。
func (w *worldSession) venusDynamicSpawnPackets() []outboundPacket {
	if w.venus == nil || w.activeDungeon == nil || !legion.IsVenusStageDungeon(w.activeDungeon.Definition.ID) {
		return nil
	}
	run := w.venus
	if len(run.pending) == 0 {
		return nil
	}
	grid := [2]byte{w.activeDungeon.Room.X, w.activeDungeon.Room.Y}
	rows := make([]protocol.UnassignedMonster115, 0, len(run.pending))
	remaining := make([]protocol.UnassignedMonster115, 0, len(run.pending))
	for _, row := range run.pending {
		if row.Grid == grid {
			rows = append(rows, row)
			continue
		}
		remaining = append(remaining, row)
	}
	if len(rows) == 0 {
		return nil
	}
	body, err := protocol.UnassignedMonsterAdd115(rows)
	if err != nil {
		log.Printf("venus dynamic carriers encode failed (%v)，放弃注册", err)
		return nil
	}
	run.pending = remaining
	return []outboundPacket{{"venus_carrier_dynamic_spawn", 0, 2194, body}}
}

// venusStageTimer 同步维纳斯阶段倒计时（NOTI1474 DUNGEON_TIMEOUT_TIME，挂在
// finishDungeonLoading 的 N30 加载应答之后）。正文 8B = [阶段时限秒,
// 阶段开始Unix秒]：频道 99 走秒分支（1452AEC10 第一/第二 u32；毫秒特例是
// 矿区频道 106，与维纳斯内部内容号 106 无关——1474 规格文档）。时限取
// VenusPhaseLimits（第 1..3 关 600 秒、降临第 4 关 900 秒），开始时间在该
// 阶段第一次真实加载完成时冻结进 run.stageClock，同阶段的房间重建/重载复用
// 原开始时间——用当前发送时间会错误延长本阶段倒计时（规格文档「已推翻」条）。
// 客户端 141C39AA0 按结束秒减服务器秒并用源上限钳制后驱动副本内倒计时 UI；
// 倒计时到期的服务端处置仍属遗留（不做伪失败判定）。
func (w *worldSession) venusStageTimer(now time.Time) []outboundPacket {
	if w.venus == nil || w.activeDungeon == nil || !legion.IsVenusStageDungeon(w.activeDungeon.Definition.ID) {
		return nil
	}
	stage, err := legion.VenusStageOfDungeon(w.activeDungeon.Definition.ID)
	if err != nil || stage >= len(legion.VenusPhaseLimits) {
		return nil
	}
	if w.venus.stageClock[stage].IsZero() {
		w.venus.stageClock[stage] = now
	}
	body, err := protocol.LegionDungeonTimeout115(w.venus.stageClock[stage], time.Duration(legion.VenusPhaseLimits[stage])*time.Second)
	if err != nil {
		log.Printf("venus stage timer encode failed (stage %d): %v", stage, err)
		return nil
	}
	return []outboundPacket{{"venus_stage_timer_sync", 0, legion.NotiDungeonTimeoutTime, body}}
}

// venusStageTimeout 判定阶段倒计时到期（挑战失败）：N1474 只驱动客户端显示，
// 失败判定是服务端职责（1474 规格文档「此包不代替服务器超时判定」）。到期 =
// 当前时间超过该阶段冻结开始时间 + VenusPhaseLimits[stage]。
//
// 退场用原生超时失败信号 **N33（FAIL_CLEAR_DUNGEON，reason 100=timeout）**
// 开路（141C3D3D0/1452af830：置 DUNGEON_STATE_FAIL_CLEAR、原生失败流程，客户
// 端自己驱动退场——2026-10-05 实测：直接推 42-ack+城镇外观包会让角色在退场
// 前 ~1 秒丢时装渲染成裸体，N33 让客户端走自己的失败演出避免这个窗口）。
// **不带 dungeon_leave_ack**：42-ack 是对从未发生的客户端 GIVEUP 请求的伪造
// 应答（同 ispinsTimeout 的既有口径）。后续城镇序列照 leaveDungeon 原样
// （N3/N23/N24/N2/N14/N105 + N32 原生复活——失败演出把角色置入死亡态，回
// 城后必须复活），末尾等待态 N2655 + run 复位，待机区可直接重新开团——源
// DGN [no giveup panalty] 语义下无惩罚。挂点在 connection ticker
// （client_connection.go，与矿区/伊斯到期检查同槽）；ticker 不经过 dispatch
// 的 dungeon_leave_ack 监视器，会话清理照 moonReturn 自行完成，run 复位后
// 到期条件自然失效、不会逐 tick 重发。结算期守卫：终点关投影 MarkCompleted
// 之后、翻牌链完成之前不拽人。
func (w *worldSession) venusStageTimeout(now time.Time, event func(map[string]any)) []outboundPacket {
	if w.venus == nil || w.activeDungeon == nil || w.activeDungeon.Completed() ||
		w.completionSent || w.resultSent ||
		!legion.IsVenusStageDungeon(w.activeDungeon.Definition.ID) {
		return nil
	}
	stage, err := legion.VenusStageOfDungeon(w.activeDungeon.Definition.ID)
	if err != nil || stage >= len(legion.VenusPhaseLimits) {
		return nil
	}
	started := w.venus.stageClock[stage]
	limit := time.Duration(legion.VenusPhaseLimits[stage]) * time.Second
	if started.IsZero() || now.Sub(started) < limit {
		return nil
	}
	choice := w.venus.choice
	relicMask := w.venus.relicMask
	kept := w.venus.clearedCount()
	route, err := w.leaveDungeon()
	if err != nil {
		log.Printf("venus stage timeout leave failed (stage %d): %v", stage, err)
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
	w.venus.resetRun()
	// BUG3（第二十二轮，第三十一/三十二轮补丁错乱中丢失、本轮随难度锁一并
	// 补回）：超时失败不清进度——保留 cleared/难度/遗物，回待机区从失败关
	// 继续（DGN [no giveup panalty]，无惩罚语义）。
	w.venus.choice = choice
	w.venus.relicMask = relicMask
	w.venus.entered = true
	for i := 0; i < kept; i++ {
		w.venus.cleared[i] = true
	}
	if event != nil {
		event(map[string]any{"kind": "venus_stage_timeout", "character_id": w.role.ID,
			"stage": stage, "choice": choice, "started": started.Unix(), "limit": int(limit / time.Second)})
	}
	// N33 原生超时失败开路；42-ack 不发（见上）；失败演出把角色置入死亡态，
	// 城镇序列末尾补原生复活（revive[2]=1 满血满蓝解除死亡态）。
	packets := []outboundPacket{{"venus_time_limit_failed", 0, 33, protocol.DungeonFailClear(100)}}
	for _, p := range route {
		if p.Name != "dungeon_leave_ack" {
			packets = append(packets, p)
		}
	}
	revive, err := protocol.PlayerDeathState(w.role.WireID)
	if err == nil {
		revive[2] = 1
		packets = append(packets, outboundPacket{"venus_timeout_actor_revived", 0, 32, revive})
	}
	// 序列末尾的 N2655：难度在锁时带权威已选难度（变更提示直出，不给三卡
	// 重选——与撤退路径同一语义）；无锁（理论不可达，防御）回未选择等待态。
	trailing := legion.VenusWaitingInfo()
	if choice != 0xff {
		trailing = legion.VenusChosenInfo(choice, kept)
	}
	return append(packets, outboundPacket{"venus_info_timeout_waiting", 0, legion.NotiVenusInfo, trailing})
}

// venusOperationClose 在难度选择窗倒计时归 0 时推原生 close ACK 自动关窗
// （2290 ACK @5 close=1：原生 reader 对 Action1 close=1 转关闭分支）。客户端
// 对归 0 自己不做任何事（2026-10-05 实测：窗口停在 0），官服由服务端在截止
// 时刻关闭；残留的开口状态还会让超时/撤退回待机区后难度窗自动重弹。守卫：
// 仅在窗口确实开着（未选难度、不在副本中、截止时刻已设且已过）时推一次。
func (w *worldSession) venusOperationClose(now time.Time, event func(map[string]any)) []outboundPacket {
	if w.venus == nil || w.activeDungeon != nil || w.venus.choice != 0xff ||
		w.venus.windowDeadline.IsZero() || now.Before(w.venus.windowDeadline) {
		return nil
	}
	deadline := w.venus.windowDeadline
	w.venus.windowDeadline = time.Time{}
	if event != nil {
		event(map[string]any{"kind": "venus_operation_window_closed", "character_id": w.role.ID,
			"deadline": deadline.Unix()})
	}
	return []outboundPacket{{
		"venus_operation_close", 1, legion.CmdVenusOperationSelect, legion.VenusOperationAck(1, true, 0, 0),
	}}
}

// venusStageEntryShared 是两条进图路径（CMD2045 与转阶段 CMD2062）的公共
// 部分：顺序校验 → 载入阶段副本 → 通用进图序列。返回的 frames[0] 是
// ackName/ackID 的 1B 成功应答，由调用方按各自命令语义替换或保留。
func (w *worldSession) venusStageEntryShared(stage int, ackName string, ackID uint16) (*dungeon.Session, []outboundPacket, error) {
	run := w.venus
	if run == nil {
		return nil, nil, fmt.Errorf("venus enter before start (no CMD2043 yet)")
	}
	if run.choice == 0xff {
		return nil, nil, fmt.Errorf("venus enter before a difficulty was confirmed (no CMD2290 action2)")
	}
	if stage < 0 || stage > 3 {
		return nil, nil, fmt.Errorf("venus enter stage %d out of range", stage)
	}
	if run.cleared[stage] {
		return nil, nil, fmt.Errorf("venus stage %d already cleared", stage)
	}
	if stage != run.clearedCount() {
		return nil, nil, fmt.Errorf("venus enter stage %d, want %d (sequential)", stage, run.clearedCount())
	}
	if stage > legion.VenusEndpoint(run.choice) {
		return nil, nil, fmt.Errorf("venus stage %d beyond the endpoint for choice %d", stage, run.choice)
	}
	sel := protocol.DungeonSelection{ID: legion.VenusStageDungeons[stage], Difficulty: 0, Party: 65535}
	s, err := dungeon.Select(*w.dungeons, sel, w.level, nil)
	if err != nil {
		return nil, nil, fmt.Errorf("venus stage %d entry unavailable: %w", stage, err)
	}
	noteMazeEntry(s)
	entryCtx, entryCancel := context.WithTimeout(context.Background(), 10*time.Second)
	var channelCtx *[2]byte
	if w.characters != nil {
		channelCtx = &w.characters.ChannelContext
	}
	frames, err := w.dungeonEntryPlanImpl(entryCtx, ackName, ackID, sel, s, channelCtx)
	entryCancel()
	if err != nil {
		return nil, nil, err
	}
	// 入场成功即锁定本场难度（CMD2045 与转阶段 2062 共用此处）。
	run.entered = true
	return s, frames, nil
}

// enterVenusStage handles CMD2045: load the stage dungeon and run the
// standard entry frame sequence. Venus stages are ordinary two-room mazes
// (start room + boss room per the DGN [maze info]), so unlike Ispins there
// is no arena-boss override: the generic room/boss/completion machinery
// drives the stage.
func (w *worldSession) enterVenusStage(p []byte) ([]outboundPacket, []map[string]any, error) {
	req, err := legion.DecodeVenusEnter(p)
	if err != nil {
		return nil, nil, err
	}
	if w.activeDungeon != nil {
		return nil, nil, fmt.Errorf("venus enter while a dungeon is active")
	}
	stage := int(req.Stage)
	s, frames, err := w.venusStageEntryShared(stage, "venus_enter_ack", legion.CmdEnterDungeon)
	if err != nil {
		return nil, nil, err
	}
	run := w.venus
	// 家族 CMD2045 应答是共享的 01+13B 结果块（1424FDC50 形），不是
	// dungeonEntryPlanImpl 默认的 1B 成功字节。
	frames[0].Name = "venus_enter_ack"
	frames[0].Payload = legion.EnterDungeonAck()
	// NOTI28 之前补 N3（角色状态 → 副本态）与 N27（选图上下文），与
	// SemiRaid 红门/月湖的军团进图序列同款。
	actorState, se := protocol.UserState(w.role.WireID, protocol.UserStateDungeon)
	if se != nil {
		return nil, nil, se
	}
	inserted := false
	plan := make([]outboundPacket, 0, len(frames)+3)
	for _, pkt := range frames {
		if pkt.ID == 28 && !inserted {
			plan = append(plan,
				outboundPacket{"venus_actor_state_dungeon", 0, 3, actorState},
				outboundPacket{"venus_dungeon_selection", 0, 27, protocol.EnterDungeonSelection()},
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
	// 首关三只圣物怪：N29 已按空单编码（进图序列在 dungeonEntryPlanImpl 里
	// 生成），这里把动态怪登记进花名册并挂到 run.pending，等 CMD37 加载完成
	// 后由 venusDynamicSpawnPackets 用 N2194 注册。
	run.pending = stageVenusPhaseCarriers(*w.dungeons, s, stage)
	notes := []map[string]any{{
		"kind":         "venus_stage_entered",
		"character_id": w.role.ID,
		"stage":        stage,
		"choice":       run.choice,
		"dungeon":      s.Definition.ID,
		"maze":         s.Maze.Index,
		"map":          s.Room.Map,
		"monsters":     len(s.Monsters),
	}}
	if stage == 0 && len(s.Monsters) == 0 {
		notes = append(notes, map[string]any{
			"kind":         "venus_carrier_placement_missing",
			"character_id": w.role.ID,
			"map":          s.Room.Map,
		})
	}
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
	return plan, notes, nil
}

// venusRewardEnd handles CMD2046 (content 106): the client's per-stage
// "reward done" signal. The shared family ACK is 01+13B; the reply also
// publishes the next waiting state so the native confirm path takes the next
// stage from manager+6B. Clearing the endpoint stage ends the run state
// client-side; the final settlement branch stays unimplemented and refuses.
// 阶段推进的权威信号已前移到清怪投影（venusStageProjection，2026-10-04 14:41
// 实机：转阶段走 CMD2062，客户端不发 2046）——2046 若对已通关阶段补报，
// 幂等回 ACK 不再重复推进。
func (w *worldSession) venusRewardEnd(p []byte) ([]outboundPacket, []map[string]any, error) {
	req, err := legion.DecodeVenusEnter(p)
	if err != nil {
		return nil, nil, err
	}
	run := w.venus
	if run == nil {
		return nil, nil, fmt.Errorf("venus reward end before start (no CMD2043 yet)")
	}
	stage := int(req.Stage)
	if stage < 0 || stage > 3 {
		return nil, nil, fmt.Errorf("venus reward end stage %d out of range", stage)
	}
	// 终局翻牌结束（终点关已在 boss 死亡时记账+翻牌链发出）：官方口径（国服
	// 视频 2026-10-05）——是否继续面板出现、通关过场动画播放、动画结束后右上
	// 角面板与遗物消失、玩家点返回城镇。这里回 **N2655 final 态**（State3/
	// Outcome1，家族约定镜像伊斯 N2255 "final"——它就是伊斯通关视频的触发）
	// 并置 finalDone：客户端留在副本里播放视频，视频的 CMD191 由
	// venusStoryPause 应答，恢复时发 leave 态关面板；返回城镇走 CMD72
	// finalDone 分支（退场 + run 作废）。通关视频的触发值无官方维纳斯样本，
	// 取家族同构映射（State3/Outcome1）。
	if stage == legion.VenusEndpoint(run.choice) && run.cleared[stage] && !run.finalDone {
		run.finalDone = true
		return []outboundPacket{
				{"venus_reward_end_ack", 1, legion.CmdRewardEnd, legion.RewardEndAck()},
				{"venus_info_final", 0, legion.NotiVenusInfo, legion.VenusFinalInfo(run.choice, stage, run.relicMask)},
			}, []map[string]any{{
				"kind":         "venus_reward_end",
				"character_id": w.role.ID,
				"stage":        stage,
				"terminal":     true,
			}}, nil
	}
	if run.cleared[stage] {
		// 清怪投影已记过通关：幂等应答，不重复发布状态。
		return []outboundPacket{{
				"venus_reward_end_ack", 1, legion.CmdRewardEnd, legion.RewardEndAck(),
			}}, []map[string]any{{
				"kind":         "venus_reward_end",
				"character_id": w.role.ID,
				"stage":        stage,
				"idempotent":   true,
			}}, nil
	}
	if stage != run.stage {
		return nil, nil, fmt.Errorf("venus reward end stage %d does not match the active stage %d", stage, run.stage)
	}
	run.cleared[stage] = true
	next := stage + 1
	note := map[string]any{
		"kind":         "venus_reward_end",
		"character_id": w.role.ID,
		"stage":        stage,
		"cleared":      run.clearedCount(),
		"endpoint":     legion.VenusEndpoint(run.choice),
	}
	if next > legion.VenusEndpoint(run.choice) {
		// 终局（最后一关结算完成）：翻牌结束后客户端发 CMD2046，服务端应答
		// ACK2046 + 回城序列（仿 CMD72 维纳斯分支），实现"翻牌完毕过几秒自动
		// 返回城镇"。包名设为 settlement_exit_ack 触发主循环清 activeDungeon。
		ack := outboundPacket{"settlement_exit_ack", 1, legion.CmdRewardEnd, legion.RewardEndAck()}
		route, e := w.leaveDungeon()
		if e != nil {
			return nil, nil, e
		}
		w.selectingDungeon = false
		run.resetRun()
		plan := append(append([]outboundPacket{ack}, route[1:]...),
			outboundPacket{"venus_info_waiting", 0, legion.NotiVenusInfo, legion.VenusWaitingInfo()})
		return plan, []map[string]any{note}, nil
	}
	return []outboundPacket{
		{"venus_reward_end_ack", 1, legion.CmdRewardEnd, legion.RewardEndAck()},
		{"venus_info_next_stage", 0, legion.NotiVenusInfo, legion.VenusStageAdvancedInfo(run.choice, next)},
	}, []map[string]any{note}, nil
}

// venusPotionGate 是维纳斯副本内的消耗品限制（军团口径每关 8 次，同伊斯
// N1584 上限）：命中返回拒绝应答；未命中计数 +1 并返回 nil。计数在
// venusStageEntryShared 进图时清零。
const venusPotionLimit = 8

func (w *worldSession) venusPotionGate(r protocol.UseStackableRequest) []outboundPacket {
	if w.venus == nil || w.activeDungeon == nil {
		return nil
	}
	stage, err := legion.VenusStageOfDungeon(w.activeDungeon.Definition.ID)
	if err != nil || stage >= len(w.venus.potionUsed) {
		return nil
	}
	if w.venus.potionUsed[stage] >= venusPotionLimit {
		return []outboundPacket{{"venus_potion_refused", 1, 44, protocol.UseStackableRefused(r)}}
	}
	w.venus.potionUsed[stage]++
	return nil
}

// venusStoryPause 应答终局通关视频期间的 CMD191（剧情暂停/恢复）：暂停/恢复
// 都答原生 N170（StoryPauseNotice）；**恢复（state=1）= 视频播完**——发
// leave 态（N2655 State5）关右上角面板与遗物显示。之后玩家点返回城镇
// （CMD72 finalDone 分支）退场并作废 run——通关后 Open 不得再进图
// （2026-10-05 实机 BUG：通关后面板残留、Open 还能重新进图）。
func (w *worldSession) venusStoryPause(p []byte) ([]outboundPacket, error) {
	r, err := protocol.DecodeStoryPause(p)
	if err != nil {
		return nil, err
	}
	run := w.venus
	if run == nil || !run.finalDone {
		return nil, fmt.Errorf("venus story pause outside the finale")
	}
	notice, err := protocol.StoryPauseNotice(w.role.WireID, r)
	if err != nil {
		return nil, err
	}
	plan := []outboundPacket{{"venus_story_pause", 0, 170, notice}}
	if r.State == 1 && !run.storyFinished {
		run.storyFinished = true
		// BUG4：视频播完即挂起「待重置」标记——此后同连接切角色时由
		// dispatchVenus 的 CMD35 分支补发 mask=0 重置（遗物 UI 归零）。
		w.pendingRelicReset = true
		plan = append(plan, outboundPacket{"venus_info_leave", 0, legion.NotiVenusInfo, legion.VenusLeaveInfo(run.choice, run.stage, run.relicMask)})
	}
	return plan, nil
}

// venusStageProjection 是 2655 文档「boss death projects to the next Stage/
// Target」的服务端实现：阶段战斗怪的最后一次死亡确认后，本阶段记为已通关、
// 权威 N2655 推进到下一阶段。客户端收到推进后的状态才会退出副本模块并按新
// 阶段记录直进（CMD2062）下一阶段副本。2026-10-04 14:41 会话实机：没有这份
// 投影时 N2655 永远停在 Stage0/记录0，客户端每 0.5 秒重选第 1 关形成无限
// 进图循环。终点阶段的推进属于终局结算分支（未实现），这里只标记通关。
func (w *worldSession) venusStageProjection(event func(map[string]any)) []outboundPacket {
	if w.venus == nil || w.activeDungeon == nil || !legion.IsVenusStageDungeon(w.activeDungeon.Definition.ID) {
		return nil
	}
	run := w.venus
	// 阶段号取自当前会话的副本号（不是 run.stage——投影推进后 run.stage 已
	// 指向下一阶段，同一会话的后续死亡上报不能再二次投影）。
	stage, err := legion.VenusStageOfDungeon(w.activeDungeon.Definition.ID)
	if err != nil || run.cleared[stage] || len(w.activeDungeon.LivingMonsters()) != 0 {
		return nil
	}
	run.cleared[stage] = true
	note := map[string]any{"kind": "venus_stage_cleared", "character_id": w.role.ID, "stage": stage}
	next := stage + 1
	if next > legion.VenusEndpoint(run.choice) {
		// 终点关清完：只标记会话完成，终局翻牌链由随后的 completeDungeon →
		// completeVenusStage 发出（N31 带阶段token → N2252 基础奖励翻牌 →
		// N2253 追加奖励，仿伊斯实机验证过的家族形状）。
		// 绝不能在这里发通用 N31（protocol.DungeonClearEnabled）——那会触发
		// CMD46 通用结算链（N34/N35/N261+8张牌翻牌，图5/图6），团本里根本
		// 不存在那种通用结算面板。
		w.activeDungeon.MarkCompleted()
		note["endpoint"] = true
		event(note)
		return nil
	}
	if run.stage == stage {
		run.stage = next
	}
	note["next"] = next
	event(note)
	return []outboundPacket{{
		"venus_info_stage_cleared", 0, legion.NotiVenusInfo, legion.VenusStageAdvancedInfo(run.choice, next),
	}}
}

// completeVenusStage is the Venus terminal settlement chain, entered from
// completeDungeon when the endpoint stage is cleared. 形状仿照伊斯
// completeIspinsStage（军团家族共享 N2252/N2253 结构，伊斯实机验证过），
// 但只发维纳斯终局需要的核心包：
//
//	N14（物品台账刷新）→ N31（通关横幅，带阶段token）→
//	N2252（基础奖励翻牌界面，第一段发放）→ N2（角色信息）→
//	N2253（追加奖励，第二段发放）→ N9（队伍稳态）
//
// 物品在发链前通过 Awarder 实际入库（2026-10-04 用户方案：第一排 5 随机
// 装备 + 3 难度材料 + 第二排 5 固定材料，共 13 项），N2252/N2253 只负责
// 展示。绝不走通用结算链（CMD46→N34/N35/N261+8张牌），
// 那是普通副本的结算面板（图5/图6），团本里不存在。
func (w *worldSession) completeVenusStage() ([]outboundPacket, error) {
	if !w.activeDungeon.Completed() || w.completionSent {
		return nil, nil
	}
	run := w.venus
	if run == nil {
		return nil, nil
	}
	stage, err := legion.VenusStageOfDungeon(w.activeDungeon.Definition.ID)
	if err != nil {
		return nil, err
	}
	// 只在终点关触发终局翻牌链；非终点关走转阶段（CMD2062），不结算。
	if stage != legion.VenusEndpoint(run.choice) {
		return nil, nil
	}

	var plan []outboundPacket

	// 第一排随机装备位：从池子 roll 5 件（会话级，展示/入库/cardPlan 共用）。
	run.flipGear = legion.VenusRollFlipGear(venusFlipGearPool, legion.VenusFlipGearCount)
	if len(run.flipGear) != legion.VenusFlipGearCount {
		log.Printf("venus flip gear pool unavailable: rolled %d of %d slots", len(run.flipGear), legion.VenusFlipGearCount)
	}

	// 物品入库：用 Awarder 把 13 项翻牌奖励（第一排 5 随机装备 + 3 难度
	// 材料 + 第二排 5 固定材料）发进背包。
	if w.loot != nil {
		awarder := &inventory.Awarder{
			Catalog:   w.loot.Catalog,
			Rules:     w.loot.BagRules,
			Equipment: w.loot.Equipment,
		}
		before, _ := inventory.ReadBag(w.role.State)
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		for _, item := range venusRewardItems(run.choice, run.flipGear) {
			updated, _, gErr := awarder.Grant(w.role.State, item.Template, item.Amount)
			if gErr != nil {
				log.Printf("venus terminal award failed: template=%d amount=%d error=%v", item.Template, item.Amount, gErr)
				continue
			}
			w.role.State = updated
		}
		// 持久化到角色存档（幂等事件键）。
		if w.store != nil {
			key := "venus-terminal:" + w.activeDungeon.RunID
			_, _, _ = w.store.CommitCharacterEvent(ctx, w.account, w.role.ID, w.role.ConfigVersion,
				key, "venus-terminal-v2", func(current database.Character) (json.RawMessage, json.RawMessage, error) {
					proof, _ := json.Marshal(map[string]any{"choice": run.choice, "stage": stage, "flip_gear": run.flipGear})
					return w.role.State, proof, nil
				})
		}
		cancel()
		// N14 物品台账刷新：让客户端即时看到背包里的奖励盒子。
		if after, rErr := inventory.ReadBag(w.role.State); rErr == nil {
			if updatePayload, uErr := protocol.InventoryUpdate(inventory.ChangedItemRows(before, after)); uErr == nil {
				plan = append(plan, outboundPacket{"venus_terminal_inventory_updated", 0, 14, updatePayload})
			}
		}
	}

	// N31 通关横幅（图1）：包名必须是 dungeon_clear_enabled，dispatch 的发送
	// 监视器按它置 completionSent。阶段 token 与 N2252 尾 @7760 呼应。
	plan = append(plan, outboundPacket{
		"dungeon_clear_enabled", 0, 31, legion.VenusDungeonClearEnabled(stage),
	})

	// N2252 基础奖励翻牌界面（第一排）：7772B 原生布局，@1600 起第一排
	// 8 条（5 随机装备 + 3 难度材料），@7760 阶段token。
	basic, err := legion.VenusBasicClearReward(run.choice, stage, run.flipGear)
	if err != nil {
		return nil, err
	}
	plan = append(plan, outboundPacket{
		"venus_basic_clear_reward", 0, legion.NotiIspinsBasicClearReward, basic,
	})

	// N2 角色信息：结算翻牌期间客户端需要角色资料刷新（照伊斯链序，在
	// N2252 之后、N2253 之前）。
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
			outboundPacket{"venus_settlement_character_info", 0, 2, basicInfo},
			outboundPacket{"venus_settlement_character_detail", 0, 2, detailInfo},
		)
	}

	// N2253 追加奖励第二排发放：2405B，40B 记录步长，固定 5 件材料各 1
	// （融合石礼盒/竞拍券/眼泪×2/花瓣），全部置展示位（flag=01）。
	additional, err := legion.VenusAdditionalClearReward()
	if err != nil {
		return nil, err
	}
	plan = append(plan, outboundPacket{
		"venus_additional_clear_reward", 0, legion.NotiIspinsAdditionalClearReward, additional,
	})

	// N9 队伍稳态：翻牌结束后重申作战中队伍状态（照伊斯链序）。
	party, err := protocol.SoloPartyInfo(w.role.WireID)
	if err != nil {
		return nil, err
	}
	plan = append(plan, outboundPacket{"venus_settlement_party_steady", 0, 9, party})

	run.cleared[stage] = true
	return plan, nil
}

// enterVenusStageDirectMove 接管清怪后的原生转阶段：客户端退出副本模块后
// 直接 CMD2062 直进下一阶段副本（目标号解析自 N2655 推进后的阶段记录）。
// 应答照普通 CMD2062 形状（选择 UI 头 gate_ack15 + N27 + 1B select_ack +
// 进图序列），2026-10-04 14:41 会话里客户端对这一形状照常加载。终点关已
// 通关后的重选用回城序列终止（终局结算分支未实现，遗留事项1）。
//
// 同关守卫（2026-10-04 16:03 会话实机）：战斗房里的门在未通关时被玩家触发，
// 客户端撞门（CMD38）约 7 秒无果后 fallback 发 CMD2062 直进**当前关**——
// 不拒绝的话服务端重建会话、玩家重载后仍站在门位，0.5 秒一轮重载循环
// （与玩家是否攻击 boss 无关）。合法转阶段的 2062 目标永远是下一关
// （旧关已清、activeDungeon 还停在旧关），按「目标 == 当前所在关且未清」
// 识别并静默拒绝（CMD2062 无拒绝应答形状，dispatch 层既有约定是静默）。
func (w *worldSession) enterVenusStageDirectMove(r protocol.DungeonDirectMove) (*dungeon.Session, []outboundPacket, error) {
	stage, err := legion.VenusStageOfDungeon(r.ID)
	if err != nil {
		return nil, nil, err
	}
	run := w.venus
	if run == nil {
		return nil, nil, fmt.Errorf("venus direct move without a run")
	}
	if w.activeDungeon != nil && legion.IsVenusStageDungeon(w.activeDungeon.Definition.ID) {
		if current, err := legion.VenusStageOfDungeon(w.activeDungeon.Definition.ID); err == nil &&
			current == stage && !run.cleared[stage] {
			// 同关直进：未清 boss 时玩家走进/站在过关门矩形内，客户端每 5 秒
			// 强制取消当前动作并发本 2062（113830 会话实证：拒绝期间 60 秒内
			// 重发 12 次，取消落在技能动画中间=技能被吞进冷却；第 1 关战斗区
			// 不在门矩形内所以从未复现）。官服对副本内直进请求一律回 16B 成功
			// ACK（巴卡尔抓包 21:44:34 战斗中同款帧：01 b2ce8d5940 00000000
			// 00000000），客户端收到后自行完成过门、停止重发——房间移动是客
			// 户端本地行为，boss/阶段状态不变。拒绝（任何错误码）都会复现
			// 5 秒节拍的技能吞噬，第四十一轮改回官服成功形状。
			return nil, []outboundPacket{{
				"venus_direct_move_ack", 1, 2062, protocol.VenusDirectMoveAck(),
			}}, nil
		}
	}
	if run.cleared[stage] && stage == legion.VenusEndpoint(run.choice) {
		route, err := w.leaveDungeon()
		if err != nil {
			return nil, nil, err
		}
		w.venus.resetRun()
		// 包名带 terminal 字样，events.jsonl 可直接检索这次兜底回城。
		return nil, append(append([]outboundPacket{}, route...),
			outboundPacket{"venus_info_terminal_waiting", 0, legion.NotiVenusInfo, legion.VenusWaitingInfo()}), nil
	}
	s, frames, err := w.venusStageEntryShared(stage, "dungeon_select_ack", 16)
	if err != nil {
		return nil, nil, err
	}
	// 首关动态圣物怪：转阶段正常不回首关，统一走同一登记以防万一（花名册
	// 非空或非首关时是空操作）。
	run.pending = stageVenusPhaseCarriers(*w.dungeons, s, stage)
	run.stage = stage
	return s, append(dungeonSelectionHead(), frames...), nil
}

// venusPhaseShift 处理维纳斯阶段本内的 CMD2329（MONSTER_HISTORY_LOG）：
// 文本 `Venus_N_Phase_Shift, Monster HP : 0.00` 是 boss 行为脚本的技能日志
// （形态转换行为组——圈圈免伤、抓取投掷、变身预备都走它），**不是击杀信号**
// （2026-10-04 16:17 会话实机：玩家完全未攻击时进图 4-5 秒也会上报；此前把它
// 当击杀处理，boss 每次放技能就被服务端误杀，N38/N2655 打断客户端演出——
// 抓取被弹到半空、角色状态机错乱抽搐、技能进冷却不释放、二阶段变身永不发生）。
//
// 维纳斯 boss 的正确流转全在客户端本地：一阶段血量打空 → 变身演出（行为日志
// 上报 2329/2059，无需应答）→ boss actor 切二阶段（血条恢复，模板不变）→
// 二阶段血量打空 → 客户端发真正的 CMD39 死亡上报 → 服务端普通死亡路径
// （ConfirmDeath → N38 → 清怪投影推进 N2655）。服务端在这里只记诊断日志。
func (w *worldSession) venusPhaseShift(p []byte, event func(map[string]any)) ([]outboundPacket, error) {
	if w.activeDungeon == nil || !legion.IsVenusStageDungeon(w.activeDungeon.Definition.ID) {
		return nil, nil
	}
	const textOffset, textLen = 4, 256
	end := textOffset + textLen
	if end > len(p) {
		end = len(p)
	}
	text := p[textOffset:end]
	if i := bytes.IndexByte(text, 0); i >= 0 {
		text = text[:i]
	}
	if !bytes.Contains(text, []byte("Phase_Shift")) {
		return nil, nil
	}
	if event != nil {
		event(map[string]any{"kind": "venus_phase_shift_log", "character_id": w.role.ID,
			"text": string(text)})
	}
	return nil, nil
}

// venusPhaseRevive handles CMD2059 (ENUM_CMDPACKET_PLAYER_REVIVE_WHEN_PHASE_
// CHANGE，2026-10-04 opcode 表取证)：维纳斯 boss 的形态转换行为（圈圈灼烧、
// 抓取摔落）把玩家 phase-change 击杀时，客户端发本命令请求免费复活——对应
// 源 DGN 的 `[player revive when phase change tag max count] 1`（每关一次）。
// 此前静默，客户端复活流程挂起：角色卡在死亡/复活中间态（半空站起来、走
// 路抽搐、技能进冷却不释放、无法跳跃），直到玩家退出副本。
// 应答回放伊斯官服 ACK2059 的 16B 常量形状（legion.IspinsReviveAck，整场
// 八帧恒 94f5c28b3c）；复活次数账本未实现，当前每次请求都放行。
// 载荷三种形态（07/27 开头的计数+浮点、带 nonce 的 306d9638…）不参与判定。
func (w *worldSession) venusPhaseRevive(p []byte) []outboundPacket {
	if w.activeDungeon == nil || !legion.IsVenusStageDungeon(w.activeDungeon.Definition.ID) {
		return nil
	}
	return []outboundPacket{{
		"venus_phase_revive_ack", 1, legion.CmdVenusPhaseRevive, legion.VenusPhaseReviveAck(),
	}}
}

// venusRewardItems 组装本次翻牌的全部入库奖励（2026-10-04 用户方案，共
// 13 项）：第一排 = flipGear（池子 roll 的随机装备，各 1）+ 3 难度材料
// （VenusBasicRewardMaterials），第二排 = 5 固定材料（VenusAdditionalRewardItems）。
func venusRewardItems(choice byte, gear []uint32) []loot.Award {
	if len(gear) > legion.VenusFlipGearCount {
		gear = gear[:legion.VenusFlipGearCount]
	}
	items := make([]loot.Award, 0, legion.VenusFlipGearCount+3+5)
	for _, t := range gear {
		items = append(items, loot.Award{Template: t, Amount: 1})
	}
	for _, m := range legion.VenusBasicRewardMaterials(choice) {
		items = append(items, loot.Award{Template: m.Template, Amount: m.Amount})
	}
	for _, m := range legion.VenusAdditionalRewardItems() {
		items = append(items, loot.Award{Template: m.Template, Amount: m.Amount})
	}
	return items
}

// venusCardPlan 组装终局翻牌计划（纯函数，便于测试）：Items[8] 正好容纳
// 第一排（5 随机装备 + 3 难度材料）；第二排 5 件不进卡组（CMD71 领取链
// 在维纳斯不可达，cardPlan 仅作防御一致性）。
func venusCardPlan(choice byte, gear []uint32, runID, source string, level byte) loot.CardPlan {
	plan := loot.CardPlan{Run: runID, Source: source, Model: "venus-terminal-v2", Level: level}
	copy(plan.Items[:], venusRewardItems(choice, gear))
	return plan
}

// freezeVenusCards 冻结终局翻牌计划：内容 = 第一排翻牌奖励，
// 沿用 FreezeCards 的存储契约（"cardplan:"+RunID 事件 + 回执校验），使 CMD71
// 的领取（PickCard → pickFrozenCard）与展示（N35 卡组）完全一致——不能用
// 通用 PlanCards 生成普通地下城的金币卡组（用户实证那是错误的翻牌外观）。
func (w *worldSession) freezeVenusCards(ctx context.Context) error {
	if w.loot == nil || w.store == nil || w.venus == nil || w.activeDungeon == nil {
		return fmt.Errorf("venus card freeze requires loot and store")
	}
	plan := venusCardPlan(w.venus.choice, w.venus.flipGear, w.activeDungeon.RunID,
		w.loot.Catalog.Source.SaveIdentity(), byte(w.activeDungeon.Definition.BasisLevel))
	commit := func(current database.Character) (json.RawMessage, json.RawMessage, error) {
		b, e := json.Marshal(plan)
		return current.State, b, e
	}
	if _, _, err := w.store.CommitCharacterEvent(ctx, w.role.AccountID, w.role.ID, plan.Source,
		"cardplan:"+plan.Run, plan.Model, commit); err != nil {
		return err
	}
	b, err := w.store.CharacterEventReceipt(ctx, w.role.AccountID, w.role.ID, "cardplan:"+plan.Run)
	if err != nil {
		return err
	}
	var stored loot.CardPlan
	if err := json.Unmarshal(b, &stored); err != nil {
		return err
	}
	if stored != plan {
		return fmt.Errorf("venus card plan round-trip mismatch")
	}
	w.cardPlan = &plan
	return nil
}

// venusFail handles CMD2044 (LEGION_FAIL, content 106): the client's abandon
// path inside a Venus dungeon. Family ACK is the 1+8B FailAck; the run state is
// reset to a fresh unchosen run (resetRun，不是作废) and the standard leave
// sequence (route carries dungeon_leave_ack, which drives the main loop's
// dungeon cleanup) returns the actor to the Venus standby area, followed by the
// waiting N2655 so the client's operation window restarts clean. 撤退不算通关：
// cleared 清空、重进从第 0 关开始——源 DGN 声明 [no giveup panalty]，无惩罚
// 语义只影响原服次数账本（未实现）。
func (w *worldSession) venusFail(p []byte) ([]outboundPacket, []map[string]any, error) {
	if _, err := legion.DecodeFail(p); err != nil {
		return nil, nil, err
	}
	if w.activeDungeon == nil || !legion.IsVenusStageDungeon(w.activeDungeon.Definition.ID) {
		return nil, nil, fmt.Errorf("venus fail outside a venus dungeon")
	}
	stage, choice := 0, byte(0xff)
	if run := w.venus; run != nil {
		stage, choice = run.stage, run.choice
	}
	route, err := w.leaveDungeon()
	if err != nil {
		return nil, nil, err
	}
	if w.venus == nil {
		w.venus = &venusRun{}
	}
	w.venus.resetRun()
	// 路由保留完整 leaveDungeon 序列（dungeon_leave_ack 驱动主循环清
	// activeDungeon）；等待态 N2655 垫在序列末尾，客户端回到待机区后作战
	// 窗口从「未选难度」重新开始。
	plan := append([]outboundPacket{
		{"venus_fail_ack", 1, legion.CmdFail, legion.FailAck()},
	}, route...)
	plan = append(plan, outboundPacket{"venus_info_waiting", 0, legion.NotiVenusInfo, legion.VenusWaitingInfo()})
	return plan, []map[string]any{{
		"kind":         "venus_retreated",
		"character_id": w.role.ID,
		"stage":        stage,
		"choice":       choice,
		"path":         "cmd2044",
	}}, nil
}

// venusRelic handles CMD2291 (GET_VENUS_RELIC): the client's relic-catch
// report. Wire contract per the 2291 source table: request 13B envelope +
// u32 template + u8 relic id; success 01+5B, refusal 00+u16 0 (refusals still
// clear the client's pending registration, so they must be sent — silence is
// what dead-ends the UI). Server checks are the source pairing (template,id)
// and the template actually registered in the current room roster — the
// carriers die right before the catch, so a dead-but-registered row is
// accepted. First catch sets the N2655 relic bit and republishes the state
// BEFORE the ACK (2655 doc: 请求者先收权威 N2655 再收 ACK).
func (w *worldSession) venusRelic(p []byte) ([]outboundPacket, []map[string]any, error) {
	template, relic, err := legion.DecodeVenusRelic(p)
	if err != nil {
		return nil, nil, err
	}
	run := w.venus
	if run == nil || w.activeDungeon == nil || !legion.IsVenusStageDungeon(w.activeDungeon.Definition.ID) {
		return nil, nil, fmt.Errorf("venus relic report outside a venus dungeon")
	}
	refusal := []outboundPacket{{"venus_relic_refused", 1, legion.CmdVenusRelic, legion.VenusRelicRefused()}}
	if legion.VenusRelicSources[relic] != template {
		return refusal, []map[string]any{{"kind": "venus_relic_rejected", "character_id": w.role.ID,
			"reason": "pairing", "template": template, "relic": relic}}, nil
	}
	if run.relicMask&(1<<relic) != 0 {
		return refusal, []map[string]any{{"kind": "venus_relic_rejected", "character_id": w.role.ID,
			"reason": "already owned", "template": template, "relic": relic}}, nil
	}
	registered := false
	for _, m := range w.activeDungeon.Monsters {
		if m.Template == template {
			registered = true
			break
		}
	}
	if !registered {
		return refusal, []map[string]any{{"kind": "venus_relic_rejected", "character_id": w.role.ID,
			"reason": "carrier absent from the current room", "template": template, "relic": relic}}, nil
	}
	run.relicMask |= 1 << relic
	return []outboundPacket{
			{"venus_info_relic", 0, legion.NotiVenusInfo, legion.VenusRelicInfo(run.choice, run.stage, run.relicMask)},
			{"venus_relic_ack", 1, legion.CmdVenusRelic, legion.VenusRelicAck(template, relic)},
		}, []map[string]any{{
			"kind":         "venus_relic_collected",
			"character_id": w.role.ID,
			"template":     template,
			"relic":        relic,
			"mask":         run.relicMask,
			"stage":        run.stage,
		}}, nil
}

// dispatchVenus 是维纳斯族的统一派发层：入场遗物重置（CMD35 角色变化）→
// 待机区组队（CMD12/13）→ 家族命令。必须排在 dispatchLegion（末世录共用
// 信封）之前。
func (client *gameConnection) dispatchVenus(requestData *clientRequest) dispatchAction {
	if requestData.frame.Type != 1 || !client.bootstrapped || client.worldState == nil {
		return dispatchNext
	}
	w := client.worldState
	if !requestData.verified {
		return dispatchNext
	}
	// BUG4：同一频道连接内切换角色后，客户端的军团内容对象仍缓存上一个
	// 角色的遗物收集位（N2655 mask 只置位不清）。
	// 第二十三轮回归修正：第二十二轮把触发条件写成「角色变化后的首个
	// CMD35」，但进维纳斯频道本身就会发 CMD35——首次进频道也命中，待机区
	// 凭空出现等待态 UI（虚假「继续」，Open 无反应，015424 会话实证）。
	// 改为显式两段式：终局视频播完（storyFinished）置 pendingRelicReset，
	// 其后同连接内角色变化的首个 CMD35 才补发 mask=0 重置；首次进频道与
	// 未打过本场挑战的连接永不触发。
	// BUG2（第三十三轮重做）：返回选择角色（CMD7，菜单动作、不可取消）的
	// 过场里，客户端先 Close 右上角面板再按 manager 里最后一次 N2655 状态
	// 重建 VENUS_MAIN_INFO_WINDOW（171948 会话 trace：Close 3789 → Open
	// 3789 → focus POPUP_WINDOW_TYPE_VENUS_MAIN_INFO_WINDOW）——选角界面
	// 因此残留面板。在过场请求应答前发 State0 关闭态（此时客户端仍在完整
	// 世界语境，State0 的关面板语义已实机验证）。
	// 第三十四轮（用户口径）：退出队伍 = 放弃攻坚——不管经返回选角还是
	// 其他途径，只要队伍没了，run 一律整体作废（难度/进度/遗物全清零，重进
	// 走 CMD12 重建队伍 + CMD2043 全新开团），不再保留关卡续打；保留进度
	// 只属于「撤退回待机区」（CMD72，队伍还在）。关闭态用 VenusClosedInfo
	// （ChoiceFF/掩码0）——run 已作废，权威状态同步归零，客户端遗物显示
	// 一并清掉。终局已完成的 run 同样在此作废（leave 态早已关面板，重复
	// State0 是无害收尾）。
	// 注意不能在退出爆发帧（1566）期间补发 N2655——上一轮就是这么做的：
	// 1566 实际伴随「退出游戏」爆发（紧邻 CMD3，连接随即关闭），且拆屏期
	// 间的 N2655 到达只会把窗口重建打开（172342 会话 trace：Close 3789 →
	// RECV N2655 → Open 3789），残留依旧，该钩子已删除。
	if requestData.frame.ID == 7 && w.channelType == 99 && w.venus != nil {
		run := w.venus
		// 副本内直接返回选角（ESC 菜单）时通用 CMD7 处理器只清
		// activeDungeon/role，战斗期残留字段在此一并收尾。
		w.selectingDungeon = false
		w.approvedDungeonGate = 0
		w.deathSent = map[uint16]bool{}
		w.drops = nil
		w.completionSent = false
		w.completionErr = nil
		w.resultSent = false
		w.pendingTownArrival = nil
		w.resetCards()
		w.venus = nil
		if client.output.send(0, legion.NotiVenusInfo, legion.VenusClosedInfo()) != nil {
			return dispatchClose
		}
		client.event(map[string]any{"kind": "venus_exit_to_select", "character_id": client.selectedCharacterID,
			"choice": run.choice, "stage": run.stage, "kept": run.clearedCount(), "abandoned": true})
	}
	if requestData.frame.ID == 35 && w.channelType == 99 && w.venus == nil &&
		w.pendingRelicReset && client.selectedCharacterID != 0 &&
		w.lastVenusResetCharacter != client.selectedCharacterID {
		w.pendingRelicReset = false
		w.lastVenusResetCharacter = client.selectedCharacterID
		// 第三十五轮：重置包必须是 State0 关闭态而不是 State2 等待态——
		// 客户端在城镇里周期性发 CMD35（位置同步，191905 会话 11:26:23 实
		// 证：通关 CMD72 回城 1 秒后周期 CMD35 命中本钩子），等待态会把右
		// 上角面板重新顶起来；本钩子只负责清客户端缓存的遗物掩码，State0
		// 掩码照样归零且面板保持关闭。
		if client.output.send(0, legion.NotiVenusInfo, legion.VenusClosedInfo()) != nil {
			return dispatchClose
		}
		client.event(map[string]any{"kind": "venus_relic_ui_reset", "character_id": client.selectedCharacterID})
	}
	handled, packets, err := w.venusStandbyPartyHandle(requestData.frame.ID, requestData.plaintext)
	if handled {
		if err != nil {
			client.event(map[string]any{"kind": "venus_party_request_rejected", "id": requestData.frame.ID, "error": err.Error()})
			packets = []outboundPacket{{"维纳斯待机区队伍请求拒绝应答", 1, requestData.frame.ID, protocol.Refusal(8)}}
		}
		if client.sendPlan(packets, client.logCharacterResponse) != nil {
			return dispatchClose
		}
		return dispatchHandled
	}
	// 终局通关视频：CMD191（剧情暂停/恢复）在 finalDone 期间由维纳斯应答——
	// 恢复时发 leave 态关右上角面板。必须先于通用 191 处理器
	// （dispatchWorldAndQuests 只答 N170，不会驱动维纳斯的收尾序列）。
	if requestData.frame.ID == 191 && w.venus != nil && w.venus.finalDone {
		packets, err := w.venusStoryPause(requestData.plaintext)
		if err != nil {
			client.event(map[string]any{"kind": "venus_story_refused", "id": requestData.frame.ID, "error": err.Error()})
			return dispatchHandled
		}
		if client.sendPlan(packets, client.logWorldResponseBody) != nil {
			return dispatchClose
		}
		return dispatchHandled
	}
	if isVenusRequest(requestData.frame.ID, requestData.plaintext) {
		plan, notes, err := w.handleVenusRequest(requestData.plaintext, requestData.frame.ID)
		if err != nil {
			client.event(map[string]any{"kind": "venus_refused", "id": requestData.frame.ID,
				"reason": err.Error(), "request_hex": hex.EncodeToString(requestData.plaintext)})
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
