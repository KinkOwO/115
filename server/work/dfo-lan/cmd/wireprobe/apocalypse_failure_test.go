package main

import (
	"bytes"
	"dfolan/internal/legion"
	"encoding/binary"
	"testing"
)

func failedApocalypseFixture(t *testing.T) (*legionSession, *worldSession, legionResult) {
	t.Helper()
	s, w := directDeathFixture(t)
	w.activeDungeon.Dead[4096] = false
	w.deathSent = nil
	w.pilotDeath = &odysseyDeath{Run: w.activeDungeon.RunID, Dead: true, Sequence: 2}
	r, err := s.prepareApocalypseDeathReturn(w, w.activeDungeon.RunID, 2)
	if err != nil {
		t.Fatal(err)
	}
	return s, w, r
}

func TestApocalypseDeathReturnAndContinuePreserveWholeRun(t *testing.T) {
	s, w, paused := failedApocalypseFixture(t)
	oldActive, navigation, state := w.activeDungeon, w.apocalypseNavigation, append([]byte(nil), w.role.State...)
	oldRoute := w.apocalypseRun
	if w.activeDungeon != oldActive || w.apocalypseRun != oldRoute || oldRoute.Paused {
		t.Fatal("preparation adopted before delivery")
	}
	var worn bool
	for _, p := range paused.Packets {
		if p.ID == 13 {
			worn = true
		}
		if p.ID == 33 || p.ID == 42 || p.ID == 72 || p.ID == 2252 || p.ID == 2253 {
			t.Fatal("ordinary fail, fabricated ACK or reward", p.ID)
		}
	}
	if !worn {
		t.Fatal("waiting equipment not restored")
	}
	c := &gameConnection{worldState: w, legionState: *s, event: func(map[string]any) {}}
	c.finishApocalypseDeathReturn(paused)
	*s = c.legionState
	if w.activeDungeon != nil || w.apocalypseNavigation != navigation || !w.apocalypseRun.Paused || s.session.Phase != 2 || s.plan == nil || w.pilotDeath.Dead || !bytes.Equal(state, w.role.State) {
		t.Fatal("death return lost content or equipment")
	}
	continued, err := s.prepareApocalypseResume(w, 2)
	if err != nil {
		t.Fatal(err)
	}
	if w.activeDungeon != nil || !w.apocalypseRun.Paused || continued.Combat == nil || continued.Combat == oldActive || continued.Combat.RunID == oldActive.RunID || continued.Route.Paused || continued.Route.Stage != 2 || continued.Route.PhaseKey != 2 || continued.Route.Cleared != oldRoute.Cleared || w.apocalypseNavigation.RunID != navigation.RunID {
		t.Fatal("continue changed whole run/phase or reused dead room")
	}
	if continued.Packets[0].ID != legion.CmdEnterDungeon || !bytes.Equal(continued.Packets[0].Payload, legion.EnterDungeonAck()) {
		t.Fatal("continue ACK missing")
	}
	var mapCount, phaseCount int
	for _, p := range continued.Packets {
		if p.ID == 29 {
			mapCount++
		}
		if p.ID == legion.NotiLegionInfo {
			phaseCount++
			if !bytes.Equal(p.Payload, paused.Route.ResumeProjection) {
				t.Fatal("role or prior progress lost")
			}
		}
		if p.ID == 2252 || p.ID == 2253 {
			t.Fatal("death retry awarded")
		}
	}
	if mapCount != 1 || phaseCount != 1 {
		t.Fatal("not one actual reload", mapCount, phaseCount)
	}
}

func TestApocalypseDeathReturnRequiresSameDeathAndRun(t *testing.T) {
	for i, change := range []func(*legionSession, *worldSession){
		func(s *legionSession, w *worldSession) { w.pilotDeath.Run = "old-run" },
		func(s *legionSession, w *worldSession) { w.pilotDeath.Sequence++ },
		func(s *legionSession, w *worldSession) { w.pilotDeath.Dead = false },
		func(s *legionSession, w *worldSession) { w.apocalypseRun.RewardShown = true },
		func(s *legionSession, w *worldSession) { s.party.Actor++ },
		func(s *legionSession, w *worldSession) { w.activeDungeon.Loaded = false },
	} {
		s, w, _ := failedApocalypseFixture(t)
		run := w.activeDungeon.RunID
		change(s, w)
		result, err := s.prepareApocalypseDeathReturn(w, run, 2)
		if err == nil || result.Route != nil || len(result.Packets) != 0 {
			t.Fatal("stale death returned", i)
		}
	}
}

func TestApocalypsePausedAbandonReleasesProgressAndAllowsNewOperation(t *testing.T) {
	s, w, result := failedApocalypseFixture(t)
	c := &gameConnection{worldState: w, legionState: *s, event: func(map[string]any) {}}
	c.finishApocalypseDeathReturn(result)
	*s = c.legionState
	if _, err := s.handle(w, apocalypseOperationBody(2, 0), legion.CmdOperationSelect); err == nil {
		t.Fatal("difficulty changed without abandoning")
	}
	if _, err := s.prepareApocalypseAbandon(w, 1); err == nil {
		t.Fatal("unknown fail argument accepted")
	}
	drop, err := s.prepareApocalypseAbandon(w, 0)
	if err != nil || !drop.ReturnWaiting || len(drop.Packets) != 2 || drop.Packets[0].ID != legion.CmdFail || binary.LittleEndian.Uint32(drop.Packets[1].Payload[13:]) != 0 || drop.Packets[1].Payload[4] != 255 {
		t.Fatal("abandon did not reset actual stage and choice", err)
	}
	if !w.apocalypseRun.Paused || s.plan == nil {
		t.Fatal("abandon adopted before send")
	}
	c.legionState = *s
	c.legionState.adoptProjection(drop.Packets)
	c.finishApocalypseReturn("apocalypse_abandon", "apocalypse_abandon_completed")
	*s = c.legionState
	if w.apocalypseRun != nil || w.apocalypseNavigation != nil || s.plan != nil || s.session.Entered || s.party == nil {
		t.Fatal("abandon leaked content owner or dropped party")
	}
	start, err := s.handle(w, startPayload(uint32(legion.ApocalypseContent)), legion.CmdStart)
	if err != nil {
		t.Fatal(err)
	}
	s.adoptProjection(start.Packets)
	selected, err := s.handle(w, apocalypseOperationBody(2, 0), legion.CmdOperationSelect)
	if err != nil {
		t.Fatal(err)
	}
	s.adoptProjection(selected.Packets)
	entry, err := s.handle(w, navigationRequest(), legion.CmdEnterDungeon)
	if err != nil || entry.Navigation == nil {
		t.Fatal("new admission after abandoning unavailable", err)
	}
}

func TestApocalypsePausedResumeRejectsWrongStageOwnerOrSource(t *testing.T) {
	for i, change := range []func(*legionSession, *worldSession){
		func(s *legionSession, w *worldSession) { s.party.Actor++ },
		func(s *legionSession, w *worldSession) { s.catalog.SHA256 = "changed" },
		func(s *legionSession, w *worldSession) { w.state.Position.Area++ },
		func(s *legionSession, w *worldSession) { w.apocalypseRun.ResumeProjection[109] = 0 },
		func(s *legionSession, w *worldSession) { w.apocalypseRun.ResumeProjection = nil },
		func(s *legionSession, w *worldSession) { w.apocalypseRun.CatalogHash = "changed" },
	} {
		s, w, paused := failedApocalypseFixture(t)
		c := &gameConnection{worldState: w, legionState: *s, event: func(map[string]any) {}}
		c.finishApocalypseDeathReturn(paused)
		*s = c.legionState
		change(s, w)
		result, err := s.prepareApocalypseResume(w, 2)
		if err == nil || result.Combat != nil {
			t.Fatal("foreign continue accepted", i)
		}
	}
	s, w, paused := failedApocalypseFixture(t)
	c := &gameConnection{worldState: w, legionState: *s, event: func(map[string]any) {}}
	c.finishApocalypseDeathReturn(paused)
	*s = c.legionState
	if _, err := s.prepareApocalypseResume(w, 3); err == nil {
		t.Fatal("continue skipped source stage")
	}
	// A native start/continue UI request must preserve the frozen source row.
	start, err := s.handle(w, startPayload(uint32(legion.ApocalypseContent)), legion.CmdStart)
	if err != nil {
		t.Fatal(err)
	}
	s.adoptProjection(start.Packets)
	if _, err := s.prepareApocalypseResume(w, 2); err != nil {
		t.Fatal("continue after UI restore", err)
	}
}
