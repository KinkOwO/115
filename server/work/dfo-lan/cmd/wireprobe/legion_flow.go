package main

import (
	"dfolan/internal/catalog"
	"dfolan/internal/dungeon"
	"dfolan/internal/legion"
	"fmt"
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
		return "dungeon"
	case legion.CmdFail, legion.CmdRewardEnd:
		return "either"
	default:
		return "town"
	}
}

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
