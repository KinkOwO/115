package main

import (
	"context"
	"dfolan/internal/catalog"
	"dfolan/internal/dungeon"
	"dfolan/internal/game/protocol"
	"dfolan/internal/legion"
	"encoding/binary"
	"fmt"
)

// Stage is the native row ordinal; PhaseKey is its source clock/destination key.
// A portal never supplies authority to skip a source phase.
type apocalypseRoute struct {
	Entry            legion.EntryPlan
	Active           *dungeon.Session
	Character        int64
	Operation        uint32
	Stage            uint32
	First            byte
	PhaseKey         byte
	PendingPhase     byte // next destination published after a native direct-move boss death
	CatalogHash      string
	Cleared          [6]bool // delivered source boss deaths of this whole content run
	RewardShown      bool    // only adopted after the complete settlement plan is sent
	Paused           bool    // combat owner retained while physically in the waiting area
	ResumeProjection []byte  // frozen delivered source route, including its actual role
}

func (w *worldSession) ownsApocalypseDungeon() bool {
	return w != nil && w.activeDungeon != nil && (w.activeDungeon == w.apocalypseNavigation || w.apocalypseRun != nil && w.activeDungeon == w.apocalypseRun.Active)
}

func (s *legionSession) validateRoute(w *worldSession) (*apocalypseRoute, error) {
	if w == nil || s == nil {
		return nil, fmt.Errorf("apocalypse route owner unavailable")
	}
	r := w.apocalypseRun
	if r == nil || r.First == 0 || int(r.First) >= len(r.Entry.Destinations) || int(r.PhaseKey) >= len(r.Entry.Destinations) || r.Active == nil || r.Active != w.activeDungeon || !r.Active.Loaded || w.channelType != 119 || w.characters == nil || w.dungeons == nil || w.service == nil || s.party == nil || s.plan == nil || s.session == nil || s.session.Failed || r.Character != w.role.ID || r.Operation != s.plan.OperationID || s.party.CharacterID != w.role.ID || s.party.Actor != w.role.WireID || s.party.Context != w.characters.ChannelContext {
		return nil, fmt.Errorf("owned loaded apocalypse route required")
	}
	entry, err := legion.BuildEntryPlan(s.contents, s.catalog, *w.dungeons, dungeon.ValidateSelection)
	if err != nil || entry != r.Entry || s.catalog.Source != catalog.ApocalypseSource || s.catalog.SHA256 != r.CatalogHash || s.session.CharacterID != w.role.ID || !s.session.Entered || entry.Source != w.service.Catalog.Source.Checksum || r.Active.Definition.ID != entry.Destinations[r.PhaseKey] || len(s.projection) != legion.LegionInfoSize || binary.LittleEndian.Uint16(s.projection) != legion.ApocalypseContent || s.projection[4] != byte(r.Operation-1) || binary.LittleEndian.Uint32(s.projection[13:]) != r.Stage {
		return nil, fmt.Errorf("apocalypse route source or delivered projection changed")
	}
	role := binary.LittleEndian.Uint32(s.projection[109:])
	limits, err := s.catalog.CompileRoleLimits()
	quota, ok := limits.Quota(1, role)
	if err != nil || role == 0 || !ok || quota < 1 {
		return nil, fmt.Errorf("source-authorized actual party role required")
	}
	for seat := 1; seat < 4; seat++ {
		if binary.LittleEndian.Uint32(s.projection[109+4*seat:]) != 0 {
			return nil, fmt.Errorf("unowned apocalypse party seat")
		}
	}
	return r, nil
}

func (s *legionSession) validateInitialRoute(w *worldSession) (*apocalypseRoute, error) {
	r, err := s.validateRoute(w)
	if err != nil {
		return nil, err
	}
	if r.Active != w.apocalypseNavigation || r.PhaseKey != 0 {
		return nil, fmt.Errorf("owned apocalypse navigation required")
	}
	return r, nil
}

func (s *legionSession) prepareNavigationAdvance(w *worldSession, p []byte) (legionResult, error) {
	r, err := s.validateInitialRoute(w)
	if err != nil {
		return legionResult{}, err
	}
	request, err := protocol.DecodeBossCheck(p)
	if err != nil || request.Actor != w.role.WireID || request.Check != 0 || r.Stage != 0 || len(r.Active.Monsters) != 1 {
		return legionResult{}, fmt.Errorf("navigation completion identity or stage mismatch")
	}
	m := r.Active.Monsters[0]
	// MAP [monster] is the navigation gate actor. Its source ACT switches to
	// die action when all role-bearing members approach; C39 then C117 were
	// observed live. Never waive death or create ordinary dungeon completion.
	if m.Entity != request.Target || m.Rank != 3 || !r.Active.Dead[m.Entity] || !w.deathSent[m.Entity] {
		return legionResult{}, fmt.Errorf("navigation gate death has not been confirmed")
	}
	if s.projection[2+27+12] != r.First {
		return legionResult{}, fmt.Errorf("first destination was not delivered before progression")
	}
	next := append([]byte(nil), s.projection...)
	binary.LittleEndian.PutUint32(next[13:], 1)
	route := *r
	route.Stage = 1
	return legionResult{Route: &route, Packets: []outboundPacket{{"apocalypse_navigation_advanced", 0, legion.NotiLegionInfo, next}}, Events: []map[string]any{{"kind": "apocalypse_navigation_gate_completed", "target": m.Entity, "destination": r.First, "dungeon": r.Entry.Destinations[r.First], "source": r.Entry.Source, "attempt": "1/3"}}}, nil
}

func (s *legionSession) prepareFirstCombat(w *worldSession, p []byte) (legionResult, error) {
	r, err := s.validateInitialRoute(w)
	if err != nil {
		return legionResult{}, err
	}
	request, err := protocol.DecodeLegionPortal115(p)
	if err != nil {
		return legionResult{}, err
	}
	if r.Stage != 1 || request.Dungeon != r.Entry.Destinations[r.First] || request.Difficulty != uint32(r.Active.Difficulty) || request.Mode != 0 || s.projection[2+27+12] != r.First {
		return legionResult{}, fmt.Errorf("portal does not match the owned first combat route")
	}
	return s.prepareCombatLoad(w, r, request, r.First, r.Stage, nil, "apocalypse_first_combat_prepared")
}

func (s *legionSession) prepareCombatPortal(w *worldSession, p []byte) (legionResult, error) {
	if w != nil && w.activeDungeon == w.apocalypseNavigation {
		return s.prepareFirstCombat(w, p)
	}
	r, err := s.validateRoute(w)
	if err != nil {
		return legionResult{}, err
	}
	if s.clock == nil || r.PhaseKey == 0 || r.Stage >= uint32(len(r.Entry.Destinations)) {
		return legionResult{}, fmt.Errorf("combat phase does not match delivered destination")
	}
	target := r.PhaseKey
	if r.PendingPhase != 0 {
		target = r.PendingPhase
	}
	if int(target) >= len(r.Entry.Destinations) || s.projection[2+27+12*int(r.Stage)] != target {
		return legionResult{}, fmt.Errorf("combat phase does not match delivered destination")
	}
	order := s.clock.Order()
	ordinal := r.Stage
	if r.PendingPhase != 0 {
		if ordinal == 0 {
			return legionResult{}, fmt.Errorf("pending phase lacks previous combat ordinal")
		}
		ordinal--
	}
	if int(ordinal) >= len(order) || order[ordinal] != int64(r.PhaseKey) {
		return legionResult{}, fmt.Errorf("combat ordinal does not match source clock")
	}
	next, ok, err := s.nextSourceCombat(r.PhaseKey)
	if err != nil {
		return legionResult{}, err
	}
	stage := r.Stage + 1
	if r.PendingPhase != 0 {
		if int64(r.PendingPhase) != next || int(r.Stage) >= len(order) || order[r.Stage] != next {
			return legionResult{}, fmt.Errorf("pending phase differs from source next combat")
		}
		stage = r.Stage
	}
	if !ok || next <= 0 || next >= int64(len(r.Entry.Destinations)) || stage >= uint32(len(r.Entry.Destinations)) {
		return legionResult{}, fmt.Errorf("no following source combat phase; settlement executor required")
	}
	request, err := protocol.DecodeLegionPortal115(p)
	if err != nil {
		return legionResult{}, err
	}
	if request.Dungeon != r.Entry.Destinations[next] || request.Difficulty != uint32(r.Active.Difficulty) || request.Mode != 0 {
		return legionResult{}, fmt.Errorf("portal does not match following source combat phase")
	}
	// Reuse the existing DGN [clear condition]/[hunt boss] reader. Ordinary
	// maze boss coordinates can designate a conditional phase-shift map;
	// they do not replace an actual source target death (normal Ica has no
	// phase shift). Both the owned C39 death and its delivered ACK are required.
	if !apocalypseBossDeathDelivered(w, r) {
		return legionResult{}, fmt.Errorf("source combat boss death has not been confirmed and delivered")
	}
	var projection []byte
	if r.PendingPhase == 0 {
		projection = append([]byte(nil), s.projection...)
		binary.LittleEndian.PutUint32(projection[13:], stage)
		projection[2+27+12*int(stage)] = byte(next)
	}
	return s.prepareCombatLoad(w, r, request, byte(next), stage, projection, "apocalypse_following_combat_prepared")
}

func apocalypseBossDeathDelivered(w *worldSession, r *apocalypseRoute) bool {
	boss := r.Active.Definition.SourceBoss
	if boss == 0 {
		return false
	}
	for _, m := range r.Active.Monsters {
		if m.Template == boss && m.Rank == 3 && m.Team != 0 && !m.NonCombat && !m.APC && r.Active.Dead[m.Entity] && w.deathSent[m.Entity] {
			return true
		}
	}
	return false
}

func (s *legionSession) prepareCombatDeathAdvance(w *worldSession) (legionResult, error) {
	if w == nil || w.apocalypseRun == nil || w.activeDungeon == w.apocalypseNavigation {
		return legionResult{}, nil
	}
	r, err := s.validateRoute(w)
	if err != nil {
		return legionResult{}, err
	}
	if r.PendingPhase != 0 || !apocalypseBossDeathDelivered(w, r) {
		return legionResult{}, nil
	}
	declared := false
	for _, move := range s.catalog.BossMoves {
		if move.Boss == r.Active.Definition.SourceBoss && move.Boss == r.Entry.Bosses[r.PhaseKey] && move.MonsterPath != "" && move.ActionPath != "" && len(move.MonsterHash) == 64 && len(move.ActionHash) == 64 && move.AllowsOperation(r.Operation) {
			declared = true
		}
	}
	if !declared {
		return legionResult{}, nil
	}
	if s.clock == nil || r.Stage == 0 || r.Stage+1 >= uint32(len(r.Entry.Destinations)) || s.projection[2+27+12*int(r.Stage)] != r.PhaseKey {
		return legionResult{}, fmt.Errorf("direct-move death requires owned delivered combat stage")
	}
	order := s.clock.Order()
	next, ok, err := s.nextSourceCombat(r.PhaseKey)
	if err != nil {
		return legionResult{}, err
	}
	if !ok || next <= 0 || next >= int64(len(r.Entry.Destinations)) || int(r.Stage) >= len(order) || order[r.Stage] != int64(r.PhaseKey) {
		return legionResult{}, fmt.Errorf("direct-move death has no following source combat")
	}
	projection := append([]byte(nil), s.projection...)
	binary.LittleEndian.PutUint32(projection[13:], r.Stage+1)
	projection[2+27+12*int(r.Stage+1)] = byte(next)
	route := *r
	route.Stage++
	route.Cleared[r.PhaseKey] = true
	route.PendingPhase = byte(next)
	return legionResult{Route: &route, Packets: []outboundPacket{{"apocalypse_boss_death_phase_advanced", 0, legion.NotiLegionInfo, projection}}, Events: []map[string]any{{"kind": "apocalypse_native_next_phase_published", "stage": route.Stage, "phase_key": r.PhaseKey, "next_phase_key": next, "dungeon": r.Entry.Destinations[next], "attempt": "3/3"}}}, nil
}

func (s *legionSession) nextSourceCombat(phase byte) (int64, bool, error) {
	if s.plan == nil || s.clock == nil {
		return 0, false, fmt.Errorf("owned source route plan unavailable")
	}
	path, linear, err := s.plan.Gates.LinearRoute()
	if err != nil {
		return 0, false, err
	}
	if linear {
		for i, key := range path {
			if key == phase && i+1 < len(path) {
				return int64(path[i+1]), true, nil
			}
		}
		return 0, false, nil
	}
	// Existing common-clock path is retained for the unconfirmed branching
	// modes. It does not implement gate choice, shortcuts or elapsed scheduling.
	next, ok := s.clock.Next(int64(phase))
	return next, ok, nil
}

func (s *legionSession) prepareCombatLoad(w *worldSession, r *apocalypseRoute, request protocol.LegionPortal115, phase byte, stage uint32, projection []byte, event string) (legionResult, error) {
	sel := protocol.DungeonSelection{ID: request.Dungeon, Difficulty: byte(request.Difficulty), Party: 65535}
	battle, err := dungeon.Select(*w.dungeons, sel, w.level, nil)
	if err != nil {
		return legionResult{}, err
	}
	if request.TargetGrid != [2]int32{int32(battle.Maze.Start[0]), int32(battle.Maze.Start[1])} || s.contents.Contents["Apocalypse"].NeedFatigue != 0 {
		return legionResult{}, fmt.Errorf("portal grid or source fatigue admission mismatch")
	}
	battle.Definition.NoFatigue = true
	frames, err := w.dungeonEntryPlanPreservingParty(context.Background(), "unused_apocalypse_ack", legion.CmdEnterDungeon, sel, battle, &s.party.Context, true)
	if err != nil {
		return legionResult{}, err
	}
	// Reuse the verified N27/N28/N29 loading chain and real roster. The
	// legacy zero N2281 helper takes the native failure branch (u8@13);
	// do not reuse that body or inject ordinary ACK16/N115 settlement.
	plan := []outboundPacket{}
	if projection != nil {
		plan = append(plan, outboundPacket{"apocalypse_combat_phase_advanced", 0, legion.NotiLegionInfo, projection})
	}
	for _, frame := range frames[1:] {
		if frame.ID == 28 {
			plan = append(plan, outboundPacket{"apocalypse_combat_selection", 0, 27, protocol.EnterDungeonSelection()})
		}
		plan = append(plan, frame)
	}
	route := *r
	if r.PhaseKey != 0 && apocalypseBossDeathDelivered(w, r) {
		route.Cleared[r.PhaseKey] = true
	}
	route.Active = battle
	route.Stage, route.PhaseKey = stage, phase
	route.PendingPhase = 0
	return legionResult{Combat: battle, Route: &route, Packets: plan, Events: []map[string]any{{"kind": event, "stage": stage, "phase_key": phase, "dungeon": battle.Definition.ID, "map": battle.Room.Map, "monsters": len(battle.Monsters), "source": r.Entry.Source, "attempt": "1/3"}}}, nil
}
