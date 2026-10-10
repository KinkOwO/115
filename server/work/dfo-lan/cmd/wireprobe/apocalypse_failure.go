package main

import (
	"bytes"
	"dfolan/internal/dungeon"
	"dfolan/internal/game/protocol"
	"dfolan/internal/legion"
	"fmt"
	"reflect"
)

// Death returns to the COS waiting area through the already verified town
// transport, without ordinary dungeon cards/clear or a fabricated C72 ACK.
// A combat retry retains the whole navigation run and prior source clears.
func (s *legionSession) prepareApocalypseDeathReturn(w *worldSession, run string, sequence uint32) (legionResult, error) {
	if err := s.validateApocalypseReturn(w); err != nil {
		return legionResult{}, err
	}
	if w.pilotDeath == nil || !w.pilotDeath.Dead || w.pilotDeath.Run != run || w.pilotDeath.Sequence != sequence || w.activeDungeon.RunID != run || w.apocalypseRun.RewardShown {
		return legionResult{}, fmt.Errorf("apocalypse death owner or sequence changed")
	}
	town, err := s.apocalypseWaitingTransport(w)
	if err != nil {
		return legionResult{}, err
	}
	route := *w.apocalypseRun
	route.Paused = true
	route.ResumeProjection = append([]byte(nil), s.projection...)
	packets := []outboundPacket{{"apocalypse_death_waiting_roles_cleared", 0, legion.NotiLegionInfo, pausedApocalypseProjection(s.projection)}}
	return legionResult{Route: &route, Packets: append(packets, town[1:]...)}, nil
}

func (s *legionSession) validateApocalypsePaused(w *worldSession) (*apocalypseRoute, error) {
	if s == nil || w == nil || w.activeDungeon != nil || w.selectingDungeon || w.apocalypseNavigation == nil || w.apocalypseRun == nil || !w.apocalypseRun.Paused {
		return nil, fmt.Errorf("owned paused apocalypse run required")
	}
	r := w.apocalypseRun
	if r.RewardShown || len(r.ResumeProjection) != legion.LegionInfoSize || (!bytes.Equal(s.projection, pausedApocalypseProjection(r.ResumeProjection)) && !bytes.Equal(s.projection, r.ResumeProjection)) || w.state.Position.Town != r.Entry.Waiting.Town || w.state.Position.Area != r.Entry.Waiting.Area {
		return nil, fmt.Errorf("paused apocalypse waiting projection changed")
	}
	shadow, tracker := *w, *s
	shadow.activeDungeon = r.Active
	tracker.projection = r.ResumeProjection
	// Return validation also accepts a navigation-room death before a role
	// was chosen. A combat resume additionally uses the source role validator.
	if err := tracker.validateApocalypseReturn(&shadow); err != nil {
		return nil, err
	}
	current, err := legion.BuildRunPlan(s.catalog, s.clock, r.Operation)
	if err != nil || !reflect.DeepEqual(current, s.plan) {
		return nil, fmt.Errorf("paused apocalypse rules changed")
	}
	return r, nil
}

func pausedApocalypseProjection(projection []byte) []byte {
	if len(projection) != legion.LegionInfoSize {
		return nil
	}
	// Keep the ongoing mode state and source stage; State0 would turn the
	// native continuation callback into a new-operation admission. Only the
	// dungeon-specific equipment effect is withdrawn while waiting.
	paused := append([]byte(nil), projection...)
	clear(paused[109:125])
	return paused
}

// Native continuation uses C2045(content107,current stage); the destination
// comes from the delivered source row, never an arbitrary client dungeon ID.
func (s *legionSession) prepareApocalypseResume(w *worldSession, stage uint32) (legionResult, error) {
	r, err := s.validateApocalypsePaused(w)
	if err != nil {
		return legionResult{}, fmt.Errorf("apocalypse continue stage or owner mismatch: %v", err)
	}
	if stage != r.Stage {
		return legionResult{}, fmt.Errorf("apocalypse continue stage mismatch")
	}
	if r.PhaseKey == 0 && r.Stage == 0 {
		result, err := s.prepareNavigationEntry(w, 0)
		if err != nil {
			return result, err
		}
		result.Navigation.RunID = w.apocalypseNavigation.RunID
		result.Navigation.StartedAt = w.apocalypseNavigation.StartedAt
		return result, nil
	}
	shadow, tracker := *w, *s
	shadow.activeDungeon = r.Active
	tracker.projection = r.ResumeProjection
	if _, err := tracker.validateRoute(&shadow); err != nil {
		return legionResult{}, err
	}
	phase := r.ResumeProjection[2+27+12*int(stage)]
	if int(phase) >= len(r.Entry.Destinations) || phase == 0 || phase != r.PhaseKey && phase != r.PendingPhase && !(r.PhaseKey == 0 && phase == r.First) {
		return legionResult{}, fmt.Errorf("paused source destination mismatch")
	}
	dgn := r.Entry.Destinations[phase]
	sel := protocol.DungeonSelection{ID: dgn, Difficulty: r.Active.Difficulty, Party: 65535}
	selected, err := dungeon.Select(*w.dungeons, sel, w.level, nil)
	if err != nil {
		return legionResult{}, err
	}
	request := protocol.LegionPortal115{Dungeon: dgn, Difficulty: uint32(sel.Difficulty), TargetGrid: [2]int32{int32(selected.Maze.Start[0]), int32(selected.Maze.Start[1])}}
	result, err := tracker.prepareCombatLoad(w, r, request, phase, stage, r.ResumeProjection, "apocalypse_combat_resumed")
	if err != nil {
		return result, err
	}
	result.Route.Paused, result.Route.ResumeProjection = false, nil
	result.Packets = append([]outboundPacket{{"apocalypse_continue_ack", 1, legion.CmdEnterDungeon, legion.EnterDungeonAck()}}, result.Packets...)
	return result, nil
}

// 14250A8E0's confirmation callback sends C2044 argument0. In the owned
// solo waiting context this abandons the operation, including paused progress.
// No weekly-count refund is invented; the user's test policy has no count cap.
func (s *legionSession) prepareApocalypseAbandon(w *worldSession, argument uint32) (legionResult, error) {
	if s == nil || argument != 0 || w == nil || w.activeDungeon != nil || w.dungeons == nil || s.party == nil || s.plan == nil || s.session == nil || !s.session.Entered || s.party.CharacterID != w.role.ID || s.party.Actor != w.role.WireID || w.characters == nil || s.party.Context != w.characters.ChannelContext {
		return legionResult{}, fmt.Errorf("apocalypse abandon requires actual solo waiting owner")
	}
	if w.apocalypseRun != nil {
		if _, err := s.validateApocalypsePaused(w); err != nil {
			return legionResult{}, err
		}
	} else {
		entry, err := legion.BuildEntryPlan(s.contents, s.catalog, *w.dungeons, dungeon.ValidateSelection)
		if err != nil || w.channelType != 119 || w.service == nil || entry.Source != w.service.Catalog.Source.Checksum || w.state.Position.Town != entry.Waiting.Town || w.state.Position.Area != entry.Waiting.Area || len(s.projection) != legion.LegionInfoSize {
			return legionResult{}, fmt.Errorf("apocalypse abandon source waiting area mismatch")
		}
	}
	reset := legion.WaitingLegionInfoState()
	reset.State = 0
	return legionResult{ReturnWaiting: true, Packets: []outboundPacket{{"apocalypse_abandon_ack", 1, legion.CmdFail, legion.FailAck()}, {"apocalypse_abandoned_progress", 0, legion.NotiLegionInfo, legion.LegionInfo(legion.ApocalypseContent, reset)}}}, nil
}

func (client *gameConnection) finishApocalypseDeathReturn(result legionResult) {
	w := client.worldState
	w.activeDungeon, w.pendingTownArrival = nil, nil
	w.apocalypseRun = result.Route
	w.selectingDungeon, w.completionSent, w.resultSent = false, false, false
	w.approvedDungeonGate = 0
	w.completionErr, w.deathSent, w.drops = nil, nil, nil
	w.resetCards()
	if w.pilotDeath != nil {
		w.pilotDeath.Dead = false
	}
	client.legionState.adoptProjection(result.Packets)
	w.enterArea()
	if err := w.announceSelf(client.event); err != nil {
		client.event(map[string]any{"kind": "area_presence_error", "error": err.Error()})
	}
	client.event(map[string]any{"kind": "apocalypse_death_waiting_completed", "stage": result.Route.Stage, "operation": result.Route.Operation, "run": w.apocalypseNavigation.RunID, "attempt": "1/3", "client_acceptance": "pending"})
}
