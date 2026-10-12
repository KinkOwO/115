package main

import (
	"dfolan/internal/game/protocol"
	"dfolan/internal/legion"
	"fmt"
	"time"
)

// Master's four physical branches come from its CTP gate schedule/flow. The
// first boss's server clock freezes availability; no client time is trusted.
func (w *worldSession) apocalypseMasterClear(stage int, event func(map[string]any)) []outboundPacket {
	r := w.apocalypse
	if stage < 1 || stage > 5 || w.legion.plan == nil {
		return nil
	}
	started := r.StageClock[stage]
	if started.IsZero() {
		started = w.activeDungeon.StartedAt
	}
	elapsed := time.Since(started)
	if started.IsZero() || elapsed < 0 {
		event(map[string]any{"kind": "apocalypse_master_clock_refused", "stage": stage})
		w.apocalypseStageCleared[stage] = false
		return nil
	}
	if stage == 1 {
		board, err := w.legion.plan.Gates.AtElapsed(elapsed)
		if err != nil {
			event(map[string]any{"kind": "apocalypse_master_gate_refused", "error": err.Error()})
			w.apocalypseStageCleared[stage] = false
			return nil
		}
		r.MasterGateBoard = board
		r.MasterAwaitingGate = true
	}
	r.TargetCleared[stage] = true
	r.ClearElapsedMS[stage] = uint64(elapsed / time.Millisecond)
	r.Cleared = stage
	r.PhaseCleared = stage + 1
	r.Stage = stage + 1
	if stage == legion.ApocalypseEndpoint(r.Choice) {
		w.activeDungeon.MarkCompleted()
		return nil
	}
	if stage > 1 && len(r.MasterRoute) > 0 {
		w.armApocalypseAdvance(int(r.MasterRoute[0]), apocalypseStageAdvanceDelay)
	}
	event(map[string]any{"kind": "apocalypse_master_phase_cleared", "stage": stage, "gate_board": r.MasterGateBoard, "elapsed_ms": r.ClearElapsedMS[stage]})
	return []outboundPacket{{"apocalypse_master_clear_state", 0, legion.NotiLegionInfo, w.legion.apocalypseInfo()}}
}

func (s *legionSession) apocalypseMasterMove(w *worldSession, id uint32) (legionResult, error) {
	r := s.apocalypseRun()
	stage := legion.ApocalypseStageOfDungeon(id)
	if r.Choice != 2 || !r.Entered || r.Ended || r.Rewarded || s.plan == nil || w.activeDungeon == nil || !w.activeDungeon.Loaded || len(w.activeDungeon.LivingMonsters()) != 0 {
		return legionResult{}, fmt.Errorf("master branch requires its defeated owned encounter")
	}
	current := legion.ApocalypseStageOfDungeon(w.activeDungeon.Definition.ID)
	if current < 1 || r.Stage != current+1 || !r.TargetCleared[current] {
		return legionResult{}, fmt.Errorf("master branch lacks server clear proof")
	}
	route := append([]byte(nil), r.MasterRoute...)
	if r.MasterAwaitingGate {
		var err error
		route, err = s.plan.Gates.PhysicalDoorPath(r.MasterGateBoard, byte(stage))
		if err != nil {
			return legionResult{}, err
		}
	}
	if len(route) == 0 || int(route[0]) != stage || r.TargetCleared[stage] {
		return legionResult{}, fmt.Errorf("master destination is outside the frozen route")
	}
	_, frames, err := w.apocalypseStageEntry(protocol.DungeonSelection{ID: id, Party: 65535})
	if err != nil {
		return legionResult{}, err
	}
	w.cancelApocalypseAdvance()
	r.MasterRoute = route[1:]
	r.MasterAwaitingGate = false
	r.Stage = stage + 1
	r.PhaseCleared = stage + 1
	plan := append([]outboundPacket{{"apocalypse_master_direct_move_ack", 1, apocalypseDirectMoveID, []byte{1}}}, frames...)
	return legionResult{Packets: plan, Events: []map[string]any{{"kind": "apocalypse_master_route_entered", "target": stage, "remaining": r.MasterRoute}}}, nil
}
