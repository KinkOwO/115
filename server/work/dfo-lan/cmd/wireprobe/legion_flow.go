package main

import (
	"dfolan/internal/catalog"
	"dfolan/internal/legion"
	"fmt"
	"time"
)

// legionSession carries the per-connection legion state.
//
// The state itself lives in internal/legion; the two standing decisions behind
// it (solo party, no persistence) and every compromise they cost are recorded
// in analysis/tasks/next64-legion-apocalypse-plan.md §6.1 and §6.2. Read those
// before adding behaviour here.
//
// Implemented so far: CMD2043 (enter the channel), CMD2354 (operation screen),
// CMD2045 (confirm an operation), CMD2355 (announce a role change). The rest of
// the family is routed here as well but refused explicitly: an unimplemented
// packet must show up in the log as a refusal rather than be silently swallowed,
// and the server must not answer with invented data just to clear the client's
// pending-response latch.
type legionSession struct {
	session *legion.Session

	// catalog is the compiled apocalypse table (configs/apocalypse.generated.json).
	// It is optional only so a server started without the config still serves
	// the rest of the family; when it is missing every confirmation logs an
	// explicit `legion_catalog_missing` event instead of pretending the
	// operation was validated.
	catalog *catalog.ApocalypseCatalog
	clock   *legion.ApocalypseClock

	// plan is the compiled description of the confirmed operation, kept so the
	// end-of-run packets (P5) can report the reward tuple the table declares
	// next to what the client announced. It is nil when no operation was
	// confirmed or the catalog is not loaded.
	plan *legion.RunPlan

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

	// dimCloisterStage / dimCloisterParty 记次元回廊（频道 Type 50）当前那一关。
	// 该族的 CMD2043 自带阶段下标与队伍字节，服务端在应答里回显，所以只在这里留
	// 一份记录供 CMD2045 确认帧与日志引用；真正的副本状态在 world.activeDungeon。
	dimCloisterStage       int
	dimCloisterParty       byte
	dimCloisterPartyName   string
	dimCloisterPartyActive bool
	// dimCloisterPartyNameReply 是建队请求里那份**逐字节验证过的名长字节**
	// （`DecodeEvildomParty` 的 NameReplyBytes）。@3=06 之后要重推一帧队伍稳态
	// （官服 #759），重推时直接用这份，不自己拼名长 —— 这一族的读法本身踩过坑。
	dimCloisterPartyNameReply []byte
	// dimCloisterCleared 是本场已经打完的界数（0..3）。用来选 N2314 的进度变体
	// （官服三界各有一帧，见 internal/legion/dimension_cloister_info.generated.go），
	// 以及判断 CMD2046 是「还有下一界」还是终局。
	dimCloisterCleared int
	// dimCloisterStages 是本场抽定的关卡顺序（业主 2026-10-11 定调：**恢复随机关卡**）。
	//
	// 池子取 `legion.DimCloisterStageDungeons`（**有官服进图帧列的那 3 个界**：
	// TheMan/Abyss/Charon）。`DimCloisterAllStageDungeons` 的 5 个界里，
	// Moros(100003180) / LightWoman(100003200) **没有任何官服帧**，
	// 只随机副本号却回放别的界的怪物/状态帧会让三处打架（业主 2026-10-10 截图），
	// 所以要扩到"5 选 3"必须先补那两界的官方帧列。
	dimCloisterStages []uint32
	// dimCloisterWindowDeadline 是难度选择窗的截止时刻；到点由
	// worldSession.dimCloisterWindowClose 推原生 close ACK 自动关窗（同族维纳斯/末世录口径）。
	dimCloisterWindowDeadline time.Time
	// dimCloisterWindowAcked 记「本界已经发过开窗 A 帧」。同族（伊斯/维纳斯/末世录）是
	// A=开窗、B=已选两条应答，本仓据此把客户端的第二次 CMD2080 当成"玩家的选择"。
	dimCloisterWindowAcked bool
	// dimCloisterEvents 是次元回廊的待发事件（开窗状态帧、到期关窗）。
	// 用一条有序队列而不是多个定时器：开窗帧与关窗帧会在同一秒到期，顺序必须确定。
	dimCloisterEvents []dimCloisterPendingEvent

	// world is the connection's world session. The apocalypse run pointer is
	// mirrored onto it because CMD2062 DUNGEON_DIRECT_MOVE is dispatched by the
	// dungeon layer, which has no legionSession of its own.
	world *worldSession
}

// legionResult is one handled command: the packets to send plus the events that
// describe what the compiled table says about the request.
type legionResult struct {
	Packets []outboundPacket
	Events  []map[string]any
}

// side reports which side of the dungeon boundary an opcode belongs to. The
// client's own call sites decide this:
//
//   - CMD2043 / CMD2354 are town-side: the entry gate (0x1406afcc0) accepts the
//     request only while world state == 1, and the operation screen is a town
//     screen. CMD2045 is sent from that screen, before the run starts.
//   - CMD2355 is dungeon-side. Its only caller (sub_14069B4E0) returns early
//     unless world state == 3, and the packet is how a role change is announced
//     while the run is in progress.
//   - CMD2044 / CMD2046 are end-of-run signals and their call sites have not
//     been read yet, so their side stays unestablished. They are accepted on
//     either side and the arriving side is logged, which turns the first live
//     run into the evidence instead of encoding a guess as a guard.
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
		if s.session == nil || s.session.CharacterID != w.role.ID {
			s.session = legion.NewSession(w.role.ID)
		}
		s.session.Begin(w.role.ID, request.Argument)
		// StartAck is exactly the four bytes the client reads; the handler
		// discards them and rebuilds its own screen state.
		//
		// NOTI2895 follows it, and that is new: the client's entry flow reads
		// this packet into the content object it has just raised, and the server
		// had never sent one, so the object only ever held the values the
		// client's own constructor wrote. The body is that constructor's triple
		// verbatim (see internal/legion/legion_info.go for the byte map and for
		// why the three variable fields must not be invented): @3/@7/@11 are
		// handed to the content object and their meaning is still unresolved, so
		// a packet built from this server's guesses would be indistinguishable
		// from real data in the log. Order and lengths are pinned by the flow
		// test, because a short body kills the client's reader.
		return legionResult{
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
					Payload: legion.LegionInfo(legion.ApocalypseContent, legion.WaitingLegionInfoState()),
				},
			},
			Events: []map[string]any{{
				"kind":         "legion_entered_channel",
				"character_id": w.role.ID,
				"argument":     request.Argument,
				"channel_type": s.channelType,
				"info_values":  "client initializer defaults; @3/@7/@11 semantics unresolved",
			}},
		}, nil
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
		if s.session == nil {
			return legionResult{}, fmt.Errorf("legion operation selected before entering (no CMD2043 yet)")
		}
		s.session.SelectOperation(request.Action, request.Auxiliary)
		return legionResult{Packets: []outboundPacket{{
			Name:    "legion_operation_ack",
			Kind:    1,
			ID:      legion.CmdOperationSelect,
			Payload: legion.OperationAck(request.Action),
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
		// The operation id is read from the client's own screen state when it
		// sends the confirmation, so it is the first place the server learns
		// which of the four declared operations the player picked. Validate it
		// against the compiled table rather than accepting any number.
		plan, err := s.runPlan(request.Stage)
		if err != nil {
			return legionResult{}, err
		}
		s.session.EnterDungeon(request.Stage)
		s.plan = plan
		result := legionResult{Packets: []outboundPacket{{
			Name: "legion_enter_ack",
			// A zero result code is the client's "accepted": its handler only
			// reacts to 252 and 380, which are its two failure messages.
			Kind:    1,
			ID:      legion.CmdEnterDungeon,
			Payload: legion.EnterDungeonAck(),
		}}}
		if plan == nil {
			result.Events = append(result.Events, map[string]any{
				"kind":         "legion_catalog_missing",
				"operation":    request.Stage,
				"reason":       "apocalypse catalog is not loaded; the operation was not validated",
				"character_id": w.role.ID,
			})
			return result, nil
		}
		result.Events = append(result.Events, map[string]any{
			"kind":               "legion_run_planned",
			"character_id":       w.role.ID,
			"operation":          plan.OperationID,
			"operation_row":      plan.Row,
			"phase_order":        plan.PhaseOrder,
			"phase_seconds":      plan.PhaseSeconds,
			"total_seconds":      plan.TotalSeconds,
			"allow_coin":         plan.AllowCoinConfigured,
			"allow_coin_values":  plan.AllowCoin,
			"gate_schedule":      plan.GateSchedule,
			"gate_flow":          plan.GateFlow,
			"reward_label":       plan.RewardLabel,
			"reward_values":      plan.RewardValues,
			"recommend_fame":     plan.RecommendFame,
			"card_symbol_index":  plan.CardSymbolIndex,
			"member_limit_class": plan.MemberLimitClass,
		})
		return result, nil
	case legion.CmdRoleSelect:
		request, err := legion.DecodeRoleSelect(p)
		if err != nil {
			return legionResult{}, err
		}
		if request.Channel != legion.OperationChannelCode {
			return legionResult{}, fmt.Errorf("legion role channel %d, want %d", request.Channel, legion.OperationChannelCode)
		}
		if s.session == nil {
			return legionResult{}, fmt.Errorf("legion role change before entering the channel (no CMD2043 yet)")
		}
		s.session.SelectRole(request.Role)
		// The client handler forwards to the screen object without reading the
		// body, so only the opcode matters.
		return legionResult{
			Packets: []outboundPacket{{
				Name:    "legion_role_ack",
				Kind:    1,
				ID:      legion.CmdRoleSelect,
				Payload: legion.RoleSelectAck(),
			}},
			Events: []map[string]any{{
				"kind":         "legion_role_recorded",
				"character_id": w.role.ID,
				"role":         request.Role,
			}},
		}, nil
	case legion.CmdFail:
		request, err := legion.DecodeFail(p)
		if err != nil {
			return legionResult{}, err
		}
		if s.session == nil {
			return legionResult{}, fmt.Errorf("legion fail before entering the channel (no CMD2043 yet)")
		}
		s.session.Fail(request.Argument)
		// 8 bytes is what the client reads; its handler discards them.
		return resultWithEvents(legionResult{Packets: []outboundPacket{{
			Name:    "legion_fail_ack",
			Kind:    1,
			ID:      legion.CmdFail,
			Payload: legion.FailAck(),
		}}}, map[string]any{
			"kind":          "legion_run_failed",
			"character_id":  w.role.ID,
			"argument":      request.Argument,
			"arriving_side": arrivingSide,
			"operation":     s.session.Operation,
		}), nil
	case legion.CmdRewardEnd:
		request, err := legion.DecodeRewardEnd(p)
		if err != nil {
			return legionResult{}, err
		}
		if s.session == nil {
			return legionResult{}, fmt.Errorf("legion reward end before entering the channel (no CMD2043 yet)")
		}
		note := map[string]any{
			"kind":          "legion_run_reward_end",
			"character_id":  w.role.ID,
			"first":         request.First,
			"second":        request.Second,
			"flag":          request.Flag,
			"arriving_side": arrivingSide,
			"operation":     s.session.Operation,
			"failed":        s.session.Failed,
		}
		// The table's own reward tuple is reported next to the client's signal
		// so a live run shows both sides at once. It is not turned into granted
		// items yet: the reward notification's payload layout (40x40 + 140x44
		// bytes) is unresolved, which is registered as T5 in next64 §6.2.
		if s.plan != nil {
			note["table_reward_label"] = s.plan.RewardLabel
			note["table_reward_values"] = s.plan.RewardValues
			note["table_phase_seconds"] = s.plan.PhaseSeconds
			note["table_total_seconds"] = s.plan.TotalSeconds
		} else {
			note["table_reward_label"] = nil
			note["table_reward_values"] = nil
		}
		s.session.EndReward()
		s.plan = nil
		return resultWithEvents(legionResult{Packets: []outboundPacket{{
			Name:    "legion_reward_end_ack",
			Kind:    1,
			ID:      legion.CmdRewardEnd,
			Payload: legion.RewardEndAck(),
		}}}, note), nil
	default:
		return legionResult{}, fmt.Errorf("legion opcode %d is not implemented yet", id)
	}
}

// resultWithEvents attaches one event to a result. It exists so the packet
// cases above read as "ack plus one note" instead of repeating the slice
// literal.
func resultWithEvents(result legionResult, note map[string]any) legionResult {
	result.Events = append(result.Events, note)
	return result
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
	return note, true
}

// runPlan describes the confirmed operation from the compiled table. A nil plan
// with a nil error means the catalog is not loaded, which the caller reports as
// an explicit event instead of silently skipping validation.
func (s *legionSession) runPlan(operation uint32) (*legion.RunPlan, error) {
	if s.catalog == nil || s.clock == nil {
		return nil, nil
	}
	plan, err := legion.BuildRunPlan(s.catalog, s.clock, operation)
	if err != nil {
		return nil, fmt.Errorf("legion operation refused: %w", err)
	}
	return plan, nil
}

// legionRequestBody renders the raw request for the log. The first live run
// settles X1 (next64 §6.2): whether the length the caller appended already
// contains the 13-byte envelope. Every packet in this family is short (17-22
// bytes), so the whole body is logged rather than a prefix, and the length is
// taken before any formatting so the two can be compared directly.
func legionRequestBody(p []byte) (int, string) {
	return len(p), fmt.Sprintf("%x", p)
}
