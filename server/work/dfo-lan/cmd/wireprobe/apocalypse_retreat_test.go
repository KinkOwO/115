package main

import (
	"bytes"
	"dfolan/internal/dungeon"
	"dfolan/internal/game/protocol"
	"dfolan/internal/game/wire"
	"dfolan/internal/inventory"
	"dfolan/internal/legion"
	"encoding/binary"
	"encoding/json"
	"net"
	"testing"
	"time"
)

func apocalypseRetreatBody() []byte {
	p := make([]byte, 16)
	copy(p, []byte{1, 2, 1})
	return p
}

func TestApocalypseLateBossRetreatBooleanBranches(t *testing.T) {
	for _, flag := range []byte{0, 1} {
		s, w := directDeathFixture(t)
		body := apocalypseRetreatBody()
		body[2] = flag
		plan, leave, err := s.prepareApocalypseRetreat(w, body)
		if err != nil || !leave || len(plan) < 6 || plan[1].ID != 72 || !bytes.Equal(plan[1].Payload, []byte{1, 1, 2}) {
			t.Fatal("native retreat branch rejected", flag, err)
		}
		// Bool0 must still require the same owned source-bound loaded run.
		s.party.Actor++
		if _, _, err := s.prepareApocalypseRetreat(w, body); err == nil {
			t.Fatal("boolean flag bypassed owner validation")
		}
	}
	s, w, _ := initialRouteFixture(t)
	body := apocalypseRetreatBody()
	body[2] = 2
	if _, _, err := s.prepareApocalypseRetreat(w, body); err == nil {
		t.Fatal("unknown native boolean accepted")
	}
}

func TestApocalypseWaitingReturnPreservesWornInventory(t *testing.T) {
	s, w, _ := initialRouteFixture(t)
	record := inventory.EquipmentRow(inventory.BagEquipment{Slot: 12, Template: 101000013})
	record[60] = 0x7a
	state, err := inventory.SaveBag(json.RawMessage(`{"future":{"keep":true}}`), inventory.Bag{Version: "ordinary-bag-v1", Worn: []inventory.BagEquipment{
		{Slot: 12, Template: 101000013, Record: append([]byte(nil), record[:]...)},
		{Slot: 47, Template: 100610096},
		{Slot: 3, Template: 517500000},
	}})
	if err != nil {
		t.Fatal(err)
	}
	w.role.State = state
	before := append([]byte(nil), state...)
	want, err := inventory.WornPayload(state)
	if err != nil {
		t.Fatal(err)
	}
	plan, leave, err := s.prepareApocalypseRetreat(w, apocalypseRetreatBody())
	if err != nil || !leave || !bytes.Equal(w.role.State, before) {
		t.Fatal("return changed persisted equipment", err)
	}
	snapshot, visual, count := -1, -1, 0
	for i, p := range plan {
		if p.Name == "apocalypse_waiting_worn_inventory_restored" {
			snapshot = i
			count++
			if p.ID != 13 || p.Kind != 0 || !bytes.Equal(p.Payload, want) {
				t.Fatal("worn inventory lost a slot or original equipment record")
			}
		}
		if p.Name == "town_worn_visuals_restored" {
			visual = i
		}
	}
	if count != 1 || snapshot <= 4 || visual <= snapshot {
		t.Fatal("worn container must follow town return and precede visuals", snapshot, visual)
	}
}

func TestApocalypseRepeatedEntryDoesNotConsumeWeeklyTestQuota(t *testing.T) {
	s, w, _ := initialRouteFixture(t)
	// More attempts than the source weekly cap: the current test server has
	// no weekly admission counter or reward-limit persistence for this mode.
	limit := s.contents.Contents["Apocalypse"].WeeklyEnter
	if limit < 1 || limit > 100 {
		t.Fatal("fixture source cap is not usable")
	}
	for attempt := int64(0); attempt <= limit+1; attempt++ {
		c := &gameConnection{worldState: w, legionState: *s, event: func(map[string]any) {}}
		if _, leave, err := c.legionState.prepareApocalypseRetreat(w, apocalypseRetreatBody()); err != nil || !leave {
			t.Fatal("test return refused", attempt, err)
		}
		c.finishApocalypseRetreat()
		*s = c.legionState
		start, err := s.handle(w, startPayload(uint32(legion.ApocalypseContent)), legion.CmdStart)
		if err != nil {
			t.Fatal("weekly count prevented another start", attempt, err)
		}
		s.adoptProjection(start.Packets)
		selected, err := s.handle(w, apocalypseOperationBody(2, 1), legion.CmdOperationSelect)
		if err != nil {
			t.Fatal(err)
		}
		s.adoptProjection(selected.Packets)
		entry, err := s.handle(w, navigationRequest(), legion.CmdEnterDungeon)
		if err != nil || entry.Navigation == nil {
			t.Fatal("weekly count prevented another entry", attempt, err)
		}
		s.adoptProjection(entry.Packets)
		w.activeDungeon, w.apocalypseNavigation, w.apocalypseRun = entry.Navigation, entry.Navigation, entry.Route
		w.activeDungeon.Loaded = true
	}
}

func TestApocalypseRetreatBeforeGateAndWithoutRole(t *testing.T) {
	s, w, _ := initialRouteFixture(t)
	w.activeDungeon.Dead[4096] = false
	w.deathSent = nil
	clear(s.projection[109:125])
	before := append([]byte(nil), s.projection...)
	owner, party, session, state := w.apocalypseRun, s.party, *s.session, w.state
	plan, leave, err := s.prepareApocalypseRetreat(w, apocalypseRetreatBody())
	if err != nil || !leave {
		t.Fatal("uncleared retreat refused", err)
	}
	if w.apocalypseRun != owner || s.party != party || *s.session != session || w.state != state || !bytes.Equal(before, s.projection) {
		t.Fatal("prepare changed live state")
	}
	if plan[0].ID != legion.NotiLegionInfo || plan[1].ID != 72 || !bytes.Equal(plan[1].Payload, []byte{1, 1, 2}) {
		t.Fatal("native retreat order")
	}
	if binary.LittleEndian.Uint32(plan[0].Payload[5:]) != 0 || !bytes.Equal(plan[0].Payload[109:125], make([]byte, 16)) {
		t.Fatal("UI or roles not cleared")
	}
	var ids []uint16
	for _, p := range plan {
		ids = append(ids, p.ID)
		if p.ID == 42 || p.ID == 68 || p.ID == 71 {
			t.Fatal("ordinary leave/cards injected", ids)
		}
	}
	if plan[2].ID != 3 || plan[3].ID != 23 || plan[4].ID != 24 {
		t.Fatal("waiting-area return missing", ids)
	}
	focus := apocalypseRetreatBody()
	focus[0] = 2
	p, leave, err := s.prepareApocalypseRetreat(w, focus)
	if err != nil || leave || len(p) != 1 || !bytes.Equal(p[0].Payload, []byte{1, 2, 2}) {
		t.Fatal("focus response must not leave", err)
	}
}

func TestApocalypseRetreatRejectsChangedOwnership(t *testing.T) {
	for _, change := range []func(*legionSession, *worldSession){
		func(s *legionSession, w *worldSession) { w.apocalypseRun.Character++ },
		func(s *legionSession, w *worldSession) { s.party.Actor++ },
		func(s *legionSession, w *worldSession) { s.party.Context[1]++ },
		func(s *legionSession, w *worldSession) { s.session.Entered = false },
		func(s *legionSession, w *worldSession) { w.activeDungeon.Loaded = false },
		func(s *legionSession, w *worldSession) { w.activeDungeon = &dungeon.Session{} },
		func(s *legionSession, w *worldSession) { s.catalog.SHA256 = "changed" },
		func(s *legionSession, w *worldSession) { w.state.Position.Area++ },
		func(s *legionSession, w *worldSession) { s.projection[13]++ },
	} {
		s, w, _ := initialRouteFixture(t)
		change(s, w)
		if p, leave, err := s.prepareApocalypseRetreat(w, apocalypseRetreatBody()); err == nil || leave || len(p) != 0 {
			t.Fatal("foreign retreat accepted")
		}
	}
	s, w, _ := initialRouteFixture(t)
	for _, body := range [][]byte{{1, 2}, {1, 2, 2}, {1, 1, 1}, {1, 2, 1, 9}} {
		if _, leave, err := s.prepareApocalypseRetreat(w, body); err == nil || leave {
			t.Fatal("invalid retreat accepted", body)
		}
	}
}

func TestApocalypseRetreatDispatchCommitsOnlyAfterCompleteReturn(t *testing.T) {
	for _, broken := range []bool{false, true} {
		s, w, _ := initialRouteFixture(t)
		previous, nav, party, before := w.apocalypseRun, w.activeDungeon, s.party, append([]byte(nil), s.projection...)
		prepared, _, err := s.prepareApocalypseRetreat(w, apocalypseRetreatBody())
		if err != nil {
			t.Fatal(err)
		}
		a, peer := net.Pipe()
		peer.SetDeadline(time.Now().Add(5 * time.Second))
		log := func(map[string]any) {}
		c := &gameConnection{worldState: w, legionState: *s, bootstrapped: true, event: log, output: newConnectionOutput(a, make([]byte, wire.SessionKeyBytes), "test", log)}
		done := make(chan dispatchAction, 1)
		go func() {
			done <- c.dispatchLegion(&clientRequest{frame: wire.Frame{Type: 1, ID: 72}, verified: true, plaintext: apocalypseRetreatBody()})
		}()
		for i, p := range prepared {
			if broken && i == len(prepared)-1 {
				peer.Close()
				break
			}
			frame := readDispatchServerFrame(t, peer)
			if frame[0] != p.Kind || binary.LittleEndian.Uint16(frame[1:]) != p.ID {
				t.Fatal("wrong retreat frame")
			}
		}
		action := <-done
		if broken {
			if action != dispatchClose || w.apocalypseRun != previous || w.activeDungeon != nav || !bytes.Equal(c.legionState.projection, before) || !c.legionState.session.Entered {
				t.Fatal("partial send cleared owner")
			}
		} else {
			if action != dispatchHandled || w.apocalypseRun != nil || w.apocalypseNavigation != nil || w.activeDungeon != nil || c.legionState.session.Entered || c.legionState.plan != nil || c.legionState.projection != nil || c.legionState.party != party || w.selectingDungeon {
				t.Fatal("retreat did not release run and retain party")
			}
			if _, err := c.legionState.handle(w, startPayload(uint32(legion.ApocalypseContent)), legion.CmdStart); err != nil {
				t.Fatal("next start stuck", err)
			}
			if _, err := protocol.DecodeSettlementExit(apocalypseRetreatBody()); err != nil {
				t.Fatal(err)
			}
		}
		a.Close()
		peer.Close()
	}
}

func TestApocalypseMenuExitReleasesOwnedSession(t *testing.T) {
	for _, id := range []uint16{3, 7} {
		s, w, _ := initialRouteFixture(t)
		a, peer := net.Pipe()
		peer.SetDeadline(time.Now().Add(5 * time.Second))
		log := func(map[string]any) {}
		c := &gameConnection{worldState: w, legionState: *s, bootstrapped: true, selectedCharacterID: w.role.ID, event: log, output: newConnectionOutput(a, make([]byte, wire.SessionKeyBytes), "test", log)}
		done := make(chan dispatchAction, 1)
		var body []byte
		if id == 3 {
			body = make([]byte, 16)
		}
		go func() {
			done <- c.dispatchSessionTransitions(&clientRequest{frame: wire.Frame{Type: 1, ID: id}, verified: true, plaintext: body})
		}()
		frame := readDispatchServerFrame(t, peer)
		if frame[0] != 1 || binary.LittleEndian.Uint16(frame[1:]) != id {
			t.Fatal("wrong native menu response")
		}
		action := <-done
		if id == 3 && action != dispatchClose || id == 7 && action != dispatchHandled {
			t.Fatal("wrong menu transition")
		}
		if w.role.ID != 0 || w.activeDungeon != nil || w.apocalypseNavigation != nil || w.apocalypseRun != nil || c.legionState.session.Entered || c.legionState.plan != nil || c.legionState.projection != nil {
			t.Fatal("quit/character switch retained content state")
		}
		a.Close()
		peer.Close()
	}
}
