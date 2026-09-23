package main

import (
	"dfolan/internal/catalog"
	"dfolan/internal/legion"
	"fmt"
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
//
// Refusing a packet on the wrong side keeps a mid-run request from silently
// interleaving with the town screens, and — because the reason is logged — makes
// a real misclassification visible instead of looking like a dropped packet.
func sideOf(id uint16) string {
	switch id {
	case legion.CmdRoleSelect:
		return "dungeon"
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
	switch sideOf(id) {
	case "dungeon":
		if !inDungeon {
			return legionResult{}, fmt.Errorf("legion opcode %d is sent inside the dungeon (world state 3), not in town", id)
		}
	default:
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
		if s.session == nil || s.session.CharacterID != w.role.ID {
			s.session = legion.NewSession(w.role.ID)
		}
		s.session.Begin(w.role.ID, request.Argument)
		// StartAck is exactly the four bytes the client reads; the handler
		// discards them and rebuilds its own screen state.
		return legionResult{Packets: []outboundPacket{{
			Name:    "legion_start_ack",
			Kind:    1,
			ID:      legion.CmdStart,
			Payload: legion.StartAck(),
		}}}, nil
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
		plan, err := s.runPlan(request.Operation)
		if err != nil {
			return legionResult{}, err
		}
		s.session.EnterDungeon(request.Operation)
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
				"operation":    request.Operation,
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
	default:
		return legionResult{}, fmt.Errorf("legion opcode %d is not implemented yet", id)
	}
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
