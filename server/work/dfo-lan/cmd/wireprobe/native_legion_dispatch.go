package main

import (
	"dfolan/internal/game/protocol"
	"dfolan/internal/legion"
	"encoding/binary"
	"fmt"
	"time"
)

func nativeLegionRequest(s *legionSession, id uint16, p []byte) (string, bool) {
	if id == legion.CmdDimCloisterOperationClear && s != nil && s.nativeRun != nil && s.nativeRun.Source.Name == "Dimension Cloister" {
		return "Dimension Cloister", true
	}
	if id == legion.CmdDimCloisterOperationSelect {
		return "Dimension Cloister", true
	}
	if id == legion.CmdDuskyOperationSelect {
		return "DuskyIsland", true
	}
	switch id {
	case legion.CmdStart, legion.CmdEnterDungeon, legion.CmdRewardEnd, legion.CmdFail:
		if len(p) < legion.EnvelopeSize+4 {
			return "", false
		}
		switch binary.LittleEndian.Uint32(p[legion.EnvelopeSize:]) {
		case 102:
			return "Dimension Cloister", true
		case legion.DuskyContentID:
			return "DuskyIsland", true
		}
	}
	return "", false
}

func (s *legionSession) handleNativeLegion(w *worldSession, p []byte, id uint16) (legionResult, bool, error) {
	if result, handled, err := s.nativeParty(w, p, id); handled {
		return result, true, err
	}
	name, owned := nativeLegionRequest(s, id, p)
	if !owned {
		return legionResult{}, false, nil
	}
	if id == legion.CmdStart {
		result, err := s.startNativeLegion(w, p, name)
		return result, true, err
	}
	if !w.boundNativeLegion() || w.legion != s || s.nativeRun.Source.Name != name {
		return legionResult{}, true, fmt.Errorf("native request has no matching owned run")
	}
	var result legionResult
	var err error
	switch id {
	case legion.CmdDimCloisterOperationSelect, legion.CmdDuskyOperationSelect:
		result, err = s.selectNativeLegion(w, p)
	case legion.CmdDimCloisterOperationClear:
		s.nativeRun.CloseSelection()
		result.Packets = []outboundPacket{{"native_legion_operation_close_ack", 1, id, legion.DimCloisterOperationClose()}}
	case legion.CmdEnterDungeon:
		result, err = s.enterNativeLegion(w, p)
	case legion.CmdRewardEnd:
		result, err = s.nativeRewardEnd(w, p, time.Now())
	case legion.CmdFail:
		// A reward-presentation failure must remain recoverable; it is not an
		// authorization to discard a cleared stage's frozen receipt.
		if s.nativeRun.State == legion.NativePhaseComplete {
			return legionResult{}, true, fmt.Errorf("cleared native stage must finish settlement")
		}
		_, notice, _, e := nativeLegionIdentity(name)
		if e != nil {
			return legionResult{}, true, e
		}
		closed, e := s.nativeRun.Info(5, 0)
		if e != nil {
			return legionResult{}, true, e
		}
		s.nativeRun.Abandon()
		s.dimCloisterEvents = nil
		result.Packets = []outboundPacket{{"native_legion_fail_ack", 1, legion.CmdFail, legion.FailAck()}, {"native_legion_abandoned", 0, notice, closed}}
	}
	return result, true, err
}

func (s *legionSession) nativeParty(w *worldSession, p []byte, id uint16) (legionResult, bool, error) {
	if id == 12 {
		if len(p) < 6 {
			return legionResult{}, false, nil
		}
		n := uint64(binary.LittleEndian.Uint32(p[2:]))
		if n+16 > uint64(len(p)) || p[15+n] != protocol.DuskyStandbyPartyType {
			return legionResult{}, false, nil
		}
		name, err := protocol.DecodeDuskyStandbyParty(p)
		if err != nil {
			return legionResult{}, true, err
		}
		if w == nil || w.role.ID == 0 || w.characters == nil || w.activeDungeon != nil || (s.nativeRun != nil && s.nativeRun.State != legion.NativeComplete && s.nativeRun.State != legion.NativeAbandoned) {
			return legionResult{}, true, fmt.Errorf("Dusky standby party unavailable during an owned run")
		}
		party, err := protocol.DuskyStandbyPartyReply(name, w.role.WireID, w.characters.ChannelContext)
		if err != nil {
			return legionResult{}, true, err
		}
		basic, err := w.characters.EntryBasicProbe(w.role, w.characters.ChannelContext)
		if err != nil {
			return legionResult{}, true, err
		}
		detail, err := w.characters.EntryAddition(w.role)
		if err != nil {
			return legionResult{}, true, err
		}
		s.nativeRun = nil
		s.nativeSettlement = nativeLegionSettlement{}
		s.nativeAcknowledged = [3]bool{}
		s.dimCloisterPartyActive = false
		s.nativePartyName = append([]byte(nil), name...)
		s.nativeOwnerID = w.role.ID
		s.world = w
		w.legion = s
		w.soloPartyReady = true
		return legionResult{Packets: []outboundPacket{{"dusky_party_leader", 0, 2, basic}, {"dusky_party_leader_detail", 0, 2, detail}, {"dusky_party_created", 0, 9, party}}}, true, nil
	}
	if id != 13 || w == nil || s.nativeOwnerID != w.role.ID || (len(s.nativePartyName) == 0 && !w.boundNativeLegion()) {
		return legionResult{}, false, nil
	}
	if w.activeDungeon != nil {
		return legionResult{}, true, fmt.Errorf("native party leave requires town")
	}
	if len(p) != 0 && len(p) != 8 {
		return legionResult{}, true, fmt.Errorf("invalid native leave length")
	}
	for _, b := range p {
		if b != 0 {
			return legionResult{}, true, fmt.Errorf("invalid native leave option")
		}
	}
	var result legionResult
	if s.dimCloisterPartyActive {
		var err error
		result, err = s.dimCloisterLeaveParty(w, p)
		if err != nil {
			return legionResult{}, true, err
		}
	} else {
		var channel [2]byte
		if w.characters != nil {
			channel = w.characters.ChannelContext
		}
		result.Packets = []outboundPacket{{"native_legion_party_gone", 0, 9, protocol.BlackPurgatoryPartyGone(channel)}}
	}
	if s.nativeRun != nil {
		s.nativeRun.Abandon()
	}
	s.nativeRun = nil
	s.nativeAcknowledged = [3]bool{}
	s.nativePartyName = nil
	s.dimCloisterEvents = nil
	s.nativeSettlement = nativeLegionSettlement{}
	w.soloPartyReady = false
	return result, true, nil
}

func (client *gameConnection) scheduleNativeLegion() {
	if client.connection == nil {
		return
	}
	if at := client.legionState.nativeNextEvent(); !at.IsZero() {
		client.connection.scheduleCloisterWindow(time.Until(at))
	}
}

func (client *gameConnection) dispatchNativeLegion(req *clientRequest) dispatchAction {
	if req.frame.Type != 1 || !req.verified || !client.bootstrapped || client.worldState == nil {
		return dispatchNext
	}
	result, handled, err := client.legionState.handleNativeLegion(client.worldState, req.plaintext, req.frame.ID)
	if !handled {
		return dispatchNext
	}
	if err != nil {
		client.event(map[string]any{"kind": "native_legion_refused", "id": req.frame.ID, "error": err.Error()})
		return dispatchHandled
	}
	for _, event := range result.Events {
		client.event(event)
	}
	if client.sendPlan(result.Packets, client.logWorldResponseBody) != nil {
		return dispatchClose
	}
	client.scheduleNativeLegion()
	return dispatchHandled
}

func (s *legionSession) nativeNextEvent() time.Time {
	if s.nativeRun == nil {
		return time.Time{}
	}
	r := s.nativeRun
	switch r.State {
	case legion.NativeCountdown:
		return r.ReadyAt
	case legion.NativeSelecting:
		return r.SelectionDeadline
	case legion.NativePhaseComplete:
		if !s.nativeSettlement.movieFinished {
			return s.nativeSettlement.movieUntil
		}
	}
	return time.Time{}
}

func (w *worldSession) nativeLegionTick(now time.Time) ([]outboundPacket, error) {
	if !w.ownsNativeLegion() {
		return nil, nil
	}
	s, r := w.legion, w.legion.nativeRun
	_, notice, opcode, err := nativeLegionIdentity(r.Source.Name)
	if err != nil {
		return nil, err
	}
	if r.Ready(now) {
		info, e := r.Info(6, 0)
		return []outboundPacket{{"native_legion_waiting", 0, notice, info}}, e
	}
	if r.Expire(now) {
		info, e := r.Info(6, 0)
		return []outboundPacket{{"native_legion_selection_expired", 0, notice, info}, {"native_legion_selection_closed", 1, opcode, legion.DuskySelectionAck(1, -1, true, 0, 0, int16(r.Reselections))}}, e
	}
	view := &s.nativeSettlement
	if r.State == legion.NativePhaseComplete && !view.movieFinished && !view.movieUntil.IsZero() && !now.Before(view.movieUntil) {
		info, e := r.Info(5, 3)
		if e != nil {
			return nil, e
		}
		view.movieFinished = true
		plan := []outboundPacket{{"native_legion_movie_finished", 0, notice, info}}
		if len(view.pendingExit) > 0 {
			_, exit, e := w.exitNativeLegion(view.pendingExit, now)
			if e != nil {
				view.movieFinished = false
				view.movieUntil = now.Add(time.Second)
				return nil, e
			}
			// Timer sends do not pass through dispatchDungeon's post-send cleanup.
			w.activeDungeon = nil
			w.drops = nil
			w.deathSent = nil
			w.completionSent = false
			w.resultSent = false
			w.resetCards()
			plan = append(plan, exit...)
		}
		return plan, nil
	}
	return nil, nil
}
