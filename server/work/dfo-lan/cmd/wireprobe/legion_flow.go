package main

import (
	"dfolan/internal/catalog"
	"dfolan/internal/dungeon"
	"dfolan/internal/legion"
	"fmt"
	"time"
)

// legionSession tracks the Apocalypse UI, validated difficulty and local party.
// Navigation map ownership is adopted by worldSession after entry delivery;
// Source combat progression, retreat and normal full-clear settlement use the
// owned worldSession run; failure/continue still need their own executor.
type legionSession struct {
	session  *legion.Session
	contents catalog.LegionContents
	party    *apocalypseParty

	// catalog/clock come from the prepared native PVF source. Missing rules
	// refuse difficulty confirmation instead of falling back to UI defaults.
	catalog *catalog.ApocalypseCatalog
	clock   *legion.ApocalypseClock

	// plan describes the source-confirmed operation. It does not establish a
	// server-owned run or authorize failure/reward completion.
	plan *legion.RunPlan

	// Last completely delivered native N2895 projection. Incremental updates
	// preserve every unrelated field instead of rebuilding constructor defaults.
	projection []byte

	// channelType is the Type of the directory row this connection arrived on,
	// or 0 when no channel directory is configured. The game connection never
	// carries a channel number - only the port a client dialled does - so this
	// is how a handler can say which channel a request came from. It is
	// recorded, never enforced: refusing a legion packet on a channel the
	// client accepts would look like a dropped packet rather than a decision.
	channelType uint32

	// apocalypse is the 末世录 (content 107 / channel Type 119) run tracker. It
	// is separate from session because the two families share the envelope but
	// not the meaning: the capture shows the apocalypse family driving its own
	// difficulty byte, stage marks and role counter through NOTI2895, while
	// Venus drives NOTI2655. See apocalypse_run.go for the field evidence.
	apocalypse *legion.ApocalypseRunState

	// world is the connection's world session. The apocalypse run pointer is
	// mirrored onto it because CMD2062 DUNGEON_DIRECT_MOVE is dispatched by the
	// dungeon layer, which has no legionSession of its own.
	world *worldSession

	// 以下是次元回廊（dimension cloister）的运行态，由 cmd/wireprobe/dimension_cloister_flow.go 驱动。
	// 该族覆盖两个频道：Type 84「Hall of Dimensions」（内容 0x0d）与 Type 50（Evildom），
	// 判据见 dimension_cloister_flow2_test.go；真正的副本状态在 world.activeDungeon。
	//
	// dimCloisterStage 是当前界序号，撤退休退时置 -1 表示"整场作废重开"。
	// dimCloisterCleared 是已清界数，用来决定记录/终局态与奖励。
	// dimCloisterPartyActive 标记待机区是否已建队（CMD12/CMD13 的判据）。
	// dimCloisterWindowDeadline / dimCloisterEvents 是操作窗的待发事件队列：
	// 开窗帧与「停留若干秒后自动确认难度」的兜底帧都排在这里，由
	// connectionSession.cloisterWindow 的精确定时器到期后统一下发。
	dimCloisterStage          int
	dimCloisterCleared        int
	dimCloisterPartyActive    bool
	// dimCloisterPartyName 是建队请求（CMD12）里的队名；dimCloisterPartyNameReply
	// 是那份请求自带的"名长字节"（`DecodeEvildomParty` 的 NameReplyBytes，逐字节
	// 验证过）；队伍稳态刷新帧（N9/官服 #759）要复用同一份字节，否则客户端会当成
	// 新队伍把界面刷回初始态。
	dimCloisterPartyName      string
	dimCloisterPartyNameReply []byte
	// dimCloisterWindowAcked 标记操作窗是否已经开过（开窗 A 帧已发）。
	// 再收到一次选择请求时说明玩家已经选了 → 回"已选" B 帧并撤掉兜底关窗。
	dimCloisterWindowAcked    bool
	dimCloisterWindowDeadline time.Time
	dimCloisterEvents         []dimCloisterPendingEvent
	// dimCloisterParty 是 CMD2045 进图确认帧里的队伍字节（原样留存，供后续帧复用）。
	// dimCloisterStages 是本场三关的**抽定顺序**（dimCloisterRollStages 洗牌后写入，
	// 业主 2026-10-11 定调：恢复随机关卡）；池子取 legion.DimCloisterStageDungeons
	// （有官服进图帧列的那 3 个界：TheMan/Abyss/Charon），为空时退回固定顺序；
	// DimCloisterAllStageDungeons 的 5 个界里 Moros/LightWoman 没有任何官服帧，
	// 要扩到"5 选 3"必须先补那两界的官方帧列。
	dimCloisterParty  byte
	dimCloisterStages []uint32
}

// legionResult is one handled command: the packets to send plus the events that
// describe what the compiled table says about the request.
type legionResult struct {
	Packets       []outboundPacket
	Events        []map[string]any
	Navigation    *dungeon.Session // adopted only after the complete entry plan is delivered
	Combat        *dungeon.Session
	Route         *apocalypseRoute
	ReturnWaiting bool
}

// side reports which side of the dungeon boundary an opcode belongs to. The
// client's own call sites decide this:
//
//   - CMD2043 / CMD2354 are town-side: the entry gate (0x1406afcc0) accepts the
//     request only while world state == 1, and the operation screen is a town
//     screen. CMD2045 is sent from that screen, before the run starts.
//   - CMD2355 is dungeon-side. Current callers 14069B4E0 and140697C30 return early
//     unless world state == 3, and the packet is how a role change is announced
//     while the run is in progress.
//   - CMD2044 retains an unestablished side and no failure executor.
//     CMD2046's Apocalypse delegate is closed, but acceptance requires the
//     owned delivered full-clear result and durable reward receipts.
//
// Refusing a packet on the wrong side keeps a mid-run request from silently
// interleaving with the town screens, and — because the reason is logged — makes
// a real misclassification visible instead of looking like a dropped packet.
func sideOf(id uint16) string {
	switch id {
	case legion.CmdRoleSelect:
		// 末世录（内容107）的 CMD2355 与其它军团不同：抓包 20261008-105227 里
		// 它在攻坚房间（CMD2062 载入的副本）内由客户端在进图后立刻发出两次，
		// 而那个时点服务端的 activeDungeon 尚未建立（CMD2045 只负责确认作战，
		// 不建会话）。把这一族判成 dungeon-only 会让它必被拒绝。角色分配本身
		// 与「当前在城里还是在副本里」无关，故放宽为 either，由处理函数按
		// 本场是否已 CMD2045 入场来把关。
		return "either"
	case legion.CmdFail, legion.CmdRewardEnd:
		return "either"
	default:
		return "town"
	}
}

// apocalypseChannelType is the channel Type of the 末世录 directory row
// (configs/channel.local35.json: Apocalypse, area [apocalypse], Type 119). The
// apocalypse family shares its envelope with Venus but not its state, so the
// connection's channel type is what selects the right tracker.
const apocalypseChannelType uint32 = 119

// isApocalypse reports whether this connection speaks the 末世录 flow: either it
// arrived on the Type 119 directory row, or a run was already opened on this
// connection (the flag keeps the run working if a future channel layout moves
// the content onto another row).
func (s *legionSession) isApocalypse() bool {
	return s.channelType == apocalypseChannelType || s.apocalypse != nil
}

// apocalypseRan reports whether this connection ever opened an apocalypse run.
func (s *legionSession) apocalypseRan() bool { return s.apocalypse != nil }

// handle answers one legion command. A nil packet list with no events means
// "recognised but intentionally not answered".
func (s *legionSession) handle(w *worldSession, p []byte, id uint16) (legionResult, error) {
	if w == nil || w.role.ID == 0 {
		return legionResult{}, fmt.Errorf("legion requires a selected character")
	}
	inDungeon := w.activeDungeon != nil
	side := sideOf(id)
	switch side {
	case "dungeon":
		if !inDungeon {
			return legionResult{}, fmt.Errorf("legion opcode %d is sent inside the dungeon (world state 3), not in town", id)
		}
	case "town":
		if inDungeon {
			return legionResult{}, fmt.Errorf("legion opcode %d is town-side and unavailable inside a dungeon", id)
		}
	}

	// arrivingSide is reported by the end-of-run packets, whose side is not
	// established yet: the first live run's log is what pins it down.
	arrivingSide := "town"
	if inDungeon {
		arrivingSide = "dungeon"
	}

	// 次元回廊（频道 Type 50 / Evildom）在集结区里直接选关进图：CMD2043 自带阶段
	// 下标，服务端在那一刻就把整套进图帧列发完（官服 2026-10-09 抓包 s30 实测，
	// 客户端 24 秒后的 CMD2045 只是确认帧）。不走末世录那条状态机。
	if result, handled, err := s.handleDimCloister(w, p, id); handled {
		return result, err
	}

	// 末世录（内容107/频道 Type119）走自己的玩法层：同一套信封，但难度、
	// 阶段与角色状态都落在 NOTI2895 上，与维纳斯的 NOTI2655 不通用。
	if s.isApocalypse() {
		if result, handled, err := s.handleApocalypse(w, p, id, arrivingSide); handled {
			return result, err
		}
	}

	switch id {
	case legion.CmdStart:
		request, err := legion.DecodeStart(p)
		if err != nil {
			return legionResult{}, err
		}
		if request.Argument != uint32(legion.ApocalypseContent) {
			return legionResult{}, fmt.Errorf("legion content %d has no bound executor; apocalypse content is %d", request.Argument, legion.ApocalypseContent)
		}
		if w.apocalypseRun != nil && w.apocalypseRun.Paused {
			if _, err := s.validateApocalypsePaused(w); err != nil {
				return legionResult{}, err
			}
			return legionResult{Packets: []outboundPacket{{"apocalypse_continue_ui_ack", 1, legion.CmdStart, legion.StartAck()}, {"apocalypse_paused_route_restored", 0, legion.NotiLegionInfo, append([]byte(nil), w.apocalypseRun.ResumeProjection...)}}}, nil
		}
		if s.session == nil || s.session.CharacterID != w.role.ID {
			s.session = legion.NewSession(w.role.ID)
			s.plan = nil
			s.projection = nil
		}
		s.session.Begin(w.role.ID, request.Argument)
		recovery, recoveryEvents := w.recoverApocalypseRewards()
		state := legion.WaitingLegionInfoState()
		if s.plan != nil {
			state.OperationChoice = byte(s.plan.OperationID - 1)
			state.Stage = uint32(s.session.Phase)
		}
		// FD900 reads the four-byte result after the generic success byte;
		// FDDB0 then reads content+204B and publishes the waiting state.
		result := legionResult{
			Packets: []outboundPacket{
				{
					Name:    "legion_start_ack",
					Kind:    1,
					ID:      legion.CmdStart,
					Payload: legion.StartAck(),
				},
				{
					Name: "legion_info",
					Kind: 0,
					ID:   legion.NotiLegionInfo,
					// 补丁：起始状态改发「等待区」状态（State2/Outcome0/Stage0），不是客户端构造初值 14。
					Payload: legion.LegionInfo(legion.ApocalypseContent, state),
				},
			},
			Events: []map[string]any{{
				"kind":         "legion_entered_channel",
				"character_id": w.role.ID,
				"argument":     request.Argument,
				"channel_type": s.channelType,
				"info_values":  state,
			}},
		}
		result.Packets = append(result.Packets[:1], append(recovery, result.Packets[1:]...)...)
		result.Events = append(result.Events, recoveryEvents...)
		return result, nil
	case legion.CmdOperationSelect:
		request, err := legion.DecodeOperationSelect(p)
		if err != nil {
			return legionResult{}, err
		}
		// The channel code is a tag both ends agree on; a mismatch means the
		// request is not the packet we think it is, so refuse instead of
		// answering something the client cannot consume.
		if request.Channel != legion.OperationChannelCode {
			return legionResult{}, fmt.Errorf("legion operation channel %d, want %d", request.Channel, legion.OperationChannelCode)
		}
		// CMD2354 only exists after the operation screen was opened, which
		// requires CMD2043 first. Requiring the session keeps that ordering
		// visible in the log rather than papering over it.
		if s.session == nil || !s.session.Entered || s.session.CharacterID != w.role.ID {
			return legionResult{}, fmt.Errorf("legion operation selected before entering (no CMD2043 yet)")
		}
		if w.apocalypseRun != nil && w.apocalypseRun.Paused {
			return legionResult{}, fmt.Errorf("paused apocalypse difficulty is locked; abandon before selecting another operation")
		}
		switch request.Action {
		case 1:
			// 14069B580 refreshes with FF; this is not a difficulty choice.
			if request.Auxiliary != 255 {
				return legionResult{}, fmt.Errorf("legion refresh requires unset choice")
			}
		case 2:
			plan, err := legion.PlanForChoice(s.catalog, s.clock, request.Auxiliary)
			if err != nil {
				return legionResult{}, fmt.Errorf("legion difficulty refused: %w", err)
			}
			s.plan = plan
			s.session.Operation = plan.OperationID
		default:
			return legionResult{}, fmt.Errorf("legion operation action %d is not supported", request.Action)
		}
		s.session.SelectOperation(request.Action, request.Auxiliary)
		packets := []outboundPacket{}
		if request.Action == 2 {
			// FDDB0 -> A4D0 copies body+2 to manager+6A. Publish before the
			// action-2 ACK, whose A360 consumer sends CMD2045 from manager+73.
			state := legion.WaitingLegionInfoState()
			state.OperationChoice = request.Auxiliary
			state.Stage = uint32(s.session.Phase)
			packets = append(packets, outboundPacket{"legion_difficulty_selected", 0, legion.NotiLegionInfo, legion.LegionInfo(legion.ApocalypseContent, state)})
		}
		packets = append(packets, outboundPacket{
			Name:    "legion_operation_ack",
			Kind:    1,
			ID:      legion.CmdOperationSelect,
			Payload: legion.OperationAck(request.Action),
		})
		return legionResult{Packets: packets, Events: []map[string]any{{
			"kind": "legion_operation_recorded", "action": request.Action,
			"choice": request.Auxiliary, "operation": s.session.Operation,
			"character_id": w.role.ID,
		}}}, nil
	case legion.CmdEnterDungeon:
		request, err := legion.DecodeEnterDungeon(p)
		if err != nil {
			return legionResult{}, err
		}
		if request.Channel != legion.OperationChannelCode {
			return legionResult{}, fmt.Errorf("legion enter channel %d, want %d", request.Channel, legion.OperationChannelCode)
		}
		if s.session == nil {
			return legionResult{}, fmt.Errorf("legion enter before entering the channel (no CMD2043 yet)")
		}
		// CMD2045 carries a stage; difficulty was confirmed by CMD2354.
		if !s.session.Entered || s.session.CharacterID != w.role.ID || s.plan == nil || s.session.Operation != s.plan.OperationID {
			return legionResult{}, fmt.Errorf("legion enter requires a confirmed source difficulty")
		}
		if s.clock == nil {
			return legionResult{}, fmt.Errorf("legion phase clock unavailable")
		}
		if _, ok := s.clock.Seconds(int64(request.Stage)); !ok {
			return legionResult{}, fmt.Errorf("legion stage %d is not declared by the source clock", request.Stage)
		}
		if request.Stage != uint32(s.session.Phase) {
			return legionResult{}, fmt.Errorf("legion stage %d is not the active stage %d", request.Stage, s.session.Phase)
		}
		// FD160 consumes its 13B ACK but success does not initiate loading.
		// Prepare a source navigation map, not an ACK-only UI transition.
		if w.apocalypseRun != nil && w.apocalypseRun.Paused {
			return s.prepareApocalypseResume(w, request.Stage)
		}
		return s.prepareNavigationEntry(w, request.Stage)

	case legion.CmdRoleSelect:
		request, err := legion.DecodeRoleSelect(p)
		if err != nil {
			return legionResult{}, err
		}
		if request.Channel != legion.OperationChannelCode {
			return legionResult{}, fmt.Errorf("legion role channel %d, want %d", request.Channel, legion.OperationChannelCode)
		}
		if s.session == nil || !s.session.Entered || s.session.Failed || s.session.CharacterID != w.role.ID || s.plan == nil {
			return legionResult{}, fmt.Errorf("legion role requires a confirmed apocalypse difficulty and active tracker for the selected character")
		}
		if request.Role > 2 {
			return legionResult{}, fmt.Errorf("apocalypse role %d has no native executor", request.Role)
		}
		limits, err := s.catalog.CompileRoleLimits()
		if err != nil {
			return legionResult{}, err
		}
		return s.prepareNavigationRole(w, request.Role, limits)
	case legion.CmdFail:
		request, err := legion.DecodeFail(p)
		if err != nil {
			return legionResult{}, err
		}
		if s.session == nil || !s.session.Entered || s.session.CharacterID != w.role.ID {
			return legionResult{}, fmt.Errorf("legion fail requires the current character's open UI session")
		}
		return s.prepareApocalypseAbandon(w, request.Argument)
	case legion.CmdRewardEnd:
		request, err := legion.DecodeRewardEnd(p)
		if err != nil {
			return legionResult{}, err
		}
		if s.session == nil || !s.session.Entered || s.session.CharacterID != w.role.ID {
			return legionResult{}, fmt.Errorf("legion reward end requires the current character's open UI session")
		}
		return s.prepareApocalypseRewardEnd(w, request, w.apocalypseRewardService())
	default:
		return legionResult{}, fmt.Errorf("legion opcode %d is not implemented yet", id)
	}
}

// abandonOnLeave closes a legion run when the player leaves the dungeon it was
// running in (P6).
//
// It is a backstop, not the main path: the client normally announces the end of
// a run itself through CMD2044 (fail) or CMD2046 (reward end). But a player who
// simply walks out of the dungeon sends neither, and a session left marked as
// entered would make the *next* CMD2354 look like it arrived before CMD2043 —
// a confusing log line rather than a wrong answer, but exactly the kind of
// thing that wastes a live session. Returns false when there is nothing to
// close, so the caller does not emit an event about a run that never started.
func (s *legionSession) abandonOnLeave(reason string, characterID int64) (map[string]any, bool) {
	if s.session == nil || !s.session.Entered {
		return nil, false
	}
	note := map[string]any{
		"kind":         "legion_run_abandoned",
		"character_id": characterID,
		"reason":       reason,
		"operation":    s.session.Operation,
		"failed":       s.session.Failed,
	}
	s.session.Reset()
	s.plan = nil
	s.projection = nil
	return note, true
}

// legionRequestBody renders the raw request for the log. The first live run
// settles X1 (next64 §6.2): whether the length the caller appended already
// contains the 13-byte envelope. Every packet in this family is short (17-22
// bytes), so the whole body is logged rather than a prefix, and the length is
// taken before any formatting so the two can be compared directly.
func legionRequestBody(p []byte) (int, string) {
	return len(p), fmt.Sprintf("%x", p)
}

// Keep upstream event annotations on the shared legion result.
func resultWithEvents(result legionResult, note map[string]any) legionResult {
	result.Events = append(result.Events, note)
	return result
}
