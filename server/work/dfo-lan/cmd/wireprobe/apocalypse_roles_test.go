package main

import (
	"bytes"
	"dfolan/internal/dungeon"
	"dfolan/internal/game/wire"
	"dfolan/internal/legion"
	"encoding/binary"
	"net"
	"strings"
	"testing"
	"time"
)

func TestApocalypseRoleCannotAcknowledgeUnownedDungeon(t *testing.T) {
	s, w := apocalypseSelectionSession(t)
	if _, err := s.handle(w, apocalypseOperationBody(2, 1), legion.CmdOperationSelect); err != nil {
		t.Fatal(err)
	}
	w.activeDungeon = &dungeon.Session{} // ordinary dungeon does not confer legion ownership
	before, plan := *s.session, s.plan
	for _, role := range []uint32{0, 1, 2, 3, ^uint32(0)} {
		result, err := s.handle(w, rolePayload(role, legion.OperationChannelCode), legion.CmdRoleSelect)
		if err == nil || len(result.Packets) != 0 || len(result.Events) != 0 || *s.session != before || s.plan != plan {
			t.Fatalf("unowned role %d acknowledged or changed state: %+v %v", role, result, err)
		}
		if role <= 2 && !strings.Contains(err.Error(), "party seat are not bound") {
			t.Fatal("missing owner no longer diagnosed", err)
		}
	}
	s.catalog = nil
	if _, err := s.handle(w, rolePayload(1, legion.OperationChannelCode), legion.CmdRoleSelect); err == nil || !strings.Contains(err.Error(), "source unavailable") {
		t.Fatal("missing native role source accepted", err)
	}
}

func ownedNavigationRoleFixture(t *testing.T) (*legionSession, *worldSession) {
	t.Helper()
	s, w := navigationEntryFixture(t)
	entry, err := s.handle(w, navigationRequest(), legion.CmdEnterDungeon)
	if err != nil {
		t.Fatal(err)
	}
	w.activeDungeon, w.apocalypseNavigation = entry.Navigation, entry.Navigation
	w.activeDungeon.Loaded = true
	return s, w
}

func TestApocalypseOwnedRolePreservesDeliveredStateAndUsesSourceQuota(t *testing.T) {
	s, w := ownedNavigationRoleFixture(t)
	// Unrelated fields may acquire non-default values later. A role update
	// must preserve even fields it does not interpret.
	s.projection[2+27], s.projection[2+123], s.projection[2+200] = 4, 0, 71
	before := append([]byte(nil), s.projection...)
	result, err := s.handle(w, rolePayload(1, legion.OperationChannelCode), legion.CmdRoleSelect)
	if err != nil || len(result.Packets) != 2 || !bytes.Equal(s.projection, before) {
		t.Fatal("role preparation changed committed state", err)
	}
	next := append([]byte(nil), result.Packets[0].Payload...)
	if binary.LittleEndian.Uint32(next[109:]) != 1 || result.Packets[0].ID != legion.NotiLegionInfo || result.Packets[1].ID != legion.CmdRoleSelect {
		t.Fatal("wrong native seat or acknowledgement")
	}
	copy(next[109:113], before[109:113])
	if !bytes.Equal(next, before) {
		t.Fatal("role change reset unrelated state")
	}
	s.adoptProjection(result.Packets)
	duplicate, err := s.handle(w, rolePayload(1, legion.OperationChannelCode), legion.CmdRoleSelect)
	if err != nil || !bytes.Equal(duplicate.Packets[0].Payload, s.projection) {
		t.Fatal("same role is not idempotent", err)
	}
	if denied, err := s.handle(w, rolePayload(2, legion.OperationChannelCode), legion.CmdRoleSelect); err == nil || len(denied.Packets) != 0 {
		t.Fatal("solo guardian bypassed source quota")
	}
	// Alter the actual source row: availability must follow the parsed source,
	// rather than a hard-coded solo scout rule.
	for i, row := range s.catalog.Records {
		if row.Name == "[member]" && len(row.Cells) == 3 && row.Cells[0].Float == 1 {
			s.catalog.Records[i].Cells[1].Float = 2
		}
	}
	if _, err := s.handle(w, rolePayload(1, legion.OperationChannelCode), legion.CmdRoleSelect); err == nil {
		t.Fatal("changed source still allowed old role")
	}
	if _, err := s.handle(w, rolePayload(2, legion.OperationChannelCode), legion.CmdRoleSelect); err != nil {
		t.Fatal("changed source quota not consumed", err)
	}
	if _, err := s.handle(w, rolePayload(0, legion.OperationChannelCode), legion.CmdRoleSelect); err != nil {
		t.Fatal("native role cancellation refused", err)
	}
}

func TestApocalypseOwnedRoleRejectsStaleLoadSourceAndRoster(t *testing.T) {
	for _, mutate := range []func(*legionSession, *worldSession){
		func(s *legionSession, w *worldSession) { w.activeDungeon.Loaded = false },
		func(s *legionSession, w *worldSession) { w.apocalypseNavigation = &dungeon.Session{} },
		func(s *legionSession, w *worldSession) { s.party.Actor++ },
		func(s *legionSession, w *worldSession) { s.party.Context[0]++ },
		func(s *legionSession, w *worldSession) { w.dungeons.Source.Checksum = "other" },
		func(s *legionSession, w *worldSession) { w.activeDungeon.Definition.ID++ },
		func(s *legionSession, w *worldSession) { s.projection = nil },
		func(s *legionSession, w *worldSession) { s.projection[2+11] = 1 },
		func(s *legionSession, w *worldSession) { s.projection[2+111] = 1 },
	} {
		s, w := ownedNavigationRoleFixture(t)
		mutate(s, w)
		before := append([]byte(nil), s.projection...)
		got, err := s.handle(w, rolePayload(1, legion.OperationChannelCode), legion.CmdRoleSelect)
		if err == nil || len(got.Packets) != 0 || !bytes.Equal(before, s.projection) {
			t.Fatal("invalid owner acknowledged or changed state", err)
		}
	}
}

func TestApocalypseRoleDispatchCommitsOnlyAfterFullDelivery(t *testing.T) {
	for _, disconnect := range []bool{false, true} {
		s, w := ownedNavigationRoleFixture(t)
		before := append([]byte(nil), s.projection...)
		a, peer := net.Pipe()
		log := func(map[string]any) {}
		c := &gameConnection{worldState: w, legionState: *s, bootstrapped: true, event: log, output: newConnectionOutput(a, make([]byte, wire.SessionKeyBytes), "test", log)}
		if disconnect {
			peer.Close()
		} else {
			peer.SetDeadline(time.Now().Add(5 * time.Second))
		}
		done := make(chan dispatchAction, 1)
		go func() {
			done <- c.dispatchLegion(&clientRequest{frame: wire.Frame{Type: 1, ID: legion.CmdRoleSelect}, verified: true, plaintext: rolePayload(1, legion.OperationChannelCode)})
		}()
		if !disconnect {
			for _, id := range []uint16{legion.NotiLegionInfo, legion.CmdRoleSelect} {
				frame := readDispatchServerFrame(t, peer)
				if binary.LittleEndian.Uint16(frame[1:]) != id {
					t.Fatal("role dependency order changed")
				}
			}
		}
		action := <-done
		a.Close()
		peer.Close()
		if disconnect {
			if action != dispatchClose || !bytes.Equal(before, c.legionState.projection) {
				t.Fatal("failed delivery committed a role")
			}
		} else if action != dispatchHandled || binary.LittleEndian.Uint32(c.legionState.projection[109:]) != 1 {
			t.Fatal("successful delivery did not commit role")
		}
	}
}

func TestApocalypseRoleRejectsOtherCharacterAndAbandonedTracker(t *testing.T) {
	for _, mutate := range []func(*legionSession, *worldSession){
		func(s *legionSession, w *worldSession) { w.role.ID++ },
		func(s *legionSession, w *worldSession) { s.session.Reset() },
		func(s *legionSession, w *worldSession) { s.session.Failed = true },
	} {
		s, w := apocalypseSelectionSession(t)
		if _, err := s.handle(w, apocalypseOperationBody(2, 1), legion.CmdOperationSelect); err != nil {
			t.Fatal(err)
		}
		w.activeDungeon = &dungeon.Session{}
		mutate(s, w)
		before := *s.session
		result, err := s.handle(w, rolePayload(1, legion.OperationChannelCode), legion.CmdRoleSelect)
		if err == nil || len(result.Packets) != 0 || *s.session != before {
			t.Fatal("unbound character/reset/failed tracker acknowledged role", result, err)
		}
	}
}
