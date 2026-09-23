package main

import (
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
// P1 covers CMD2043 (LEGION_START) only. The rest of the family is routed to
// this handler as well, but refused explicitly: an unimplemented packet must
// show up in the log as a refusal rather than be silently swallowed, and the
// server must not answer with invented data just to clear the client's
// pending-response latch.
type legionSession struct {
	session *legion.Session
}

// handle answers one legion command. It returns the packets to send; a nil,
// nil result means "recognised but intentionally not answered".
func (s *legionSession) handle(w *worldSession, p []byte, id uint16) ([]outboundPacket, error) {
	if w == nil || w.role.ID == 0 {
		return nil, fmt.Errorf("legion requires a selected character")
	}
	// Town only: the client's own gate (0x1406afcc0) accepts the request only
	// while world state == 1, and a dungeon run must not interleave with the
	// operation.
	if w.activeDungeon != nil {
		return nil, fmt.Errorf("legion is unavailable inside a dungeon")
	}

	switch id {
	case legion.CmdStart:
		request, err := legion.DecodeStart(p)
		if err != nil {
			return nil, err
		}
		if s.session == nil || s.session.CharacterID != w.role.ID {
			s.session = legion.NewSession(w.role.ID)
		}
		s.session.Begin(w.role.ID, request.Argument)
		// StartAck is exactly the four bytes the client reads; the handler
		// discards them and rebuilds its own screen state.
		return []outboundPacket{{
			Name:    "legion_start_ack",
			Kind:    1,
			ID:      legion.CmdStart,
			Payload: legion.StartAck(),
		}}, nil
	case legion.CmdOperationSelect:
		request, err := legion.DecodeOperationSelect(p)
		if err != nil {
			return nil, err
		}
		// The channel code is a tag both ends agree on; a mismatch means the
		// request is not the packet we think it is, so refuse instead of
		// answering something the client cannot consume.
		if request.Channel != legion.OperationChannelCode {
			return nil, fmt.Errorf("legion operation channel %d, want %d", request.Channel, legion.OperationChannelCode)
		}
		// CMD2354 only exists after the operation screen was opened, which
		// requires CMD2043 first. Requiring the session keeps that ordering
		// visible in the log rather than papering over it.
		if s.session == nil {
			return nil, fmt.Errorf("legion operation selected before entering (no CMD2043 yet)")
		}
		s.session.SelectOperation(request.Action, request.Auxiliary)
		return []outboundPacket{{
			Name:    "legion_operation_ack",
			Kind:    1,
			ID:      legion.CmdOperationSelect,
			Payload: legion.OperationAck(request.Action),
		}}, nil
	case legion.CmdEnterDungeon:
		request, err := legion.DecodeEnterDungeon(p)
		if err != nil {
			return nil, err
		}
		if request.Channel != legion.OperationChannelCode {
			return nil, fmt.Errorf("legion enter channel %d, want %d", request.Channel, legion.OperationChannelCode)
		}
		if s.session == nil {
			return nil, fmt.Errorf("legion enter before entering the channel (no CMD2043 yet)")
		}
		s.session.EnterDungeon(request.Operation)
		// A zero result code is the client's "accepted": its handler only
		// reacts to 252 and 380, which are its two failure messages.
		return []outboundPacket{{
			Name:    "legion_enter_ack",
			Kind:    1,
			ID:      legion.CmdEnterDungeon,
			Payload: legion.EnterDungeonAck(),
		}}, nil
	case legion.CmdRoleSelect:
		request, err := legion.DecodeRoleSelect(p)
		if err != nil {
			return nil, err
		}
		if request.Channel != legion.OperationChannelCode {
			return nil, fmt.Errorf("legion role channel %d, want %d", request.Channel, legion.OperationChannelCode)
		}
		if s.session == nil {
			return nil, fmt.Errorf("legion role selected before entering the channel (no CMD2043 yet)")
		}
		s.session.SelectRole(request.Role)
		// The client handler forwards to the screen object without reading the
		// body, so only the opcode matters.
		return []outboundPacket{{
			Name:    "legion_role_ack",
			Kind:    1,
			ID:      legion.CmdRoleSelect,
			Payload: legion.RoleSelectAck(),
		}}, nil
	default:
		return nil, fmt.Errorf("legion opcode %d is not implemented yet", id)
	}
}

// legionRequestBody renders the raw request for the log. The first live run
// settles X1 (next64 §6.2): whether the length the caller appended already
// contains the 13-byte envelope. Every packet in this family is short (17-22
// bytes), so the whole body is logged rather than a prefix, and the length is
// taken before any formatting so the two can be compared directly.
func legionRequestBody(p []byte) (int, string) {
	return len(p), fmt.Sprintf("%x", p)
}
