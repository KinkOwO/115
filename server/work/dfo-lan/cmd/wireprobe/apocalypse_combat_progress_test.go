package main

import (
	"bytes"
	"dfolan/internal/dungeon"
	"dfolan/internal/game/protocol"
	"dfolan/internal/game/wire"
	"encoding/binary"
	"net"
	"testing"
	"time"
)

func combatProgressFixture(t *testing.T) (*legionSession, *worldSession, []byte) {
	t.Helper()
	s, w, check := initialRouteFixture(t)
	advanced, err := s.prepareNavigationAdvance(w, check)
	if err != nil {
		t.Fatal(err)
	}
	w.apocalypseRun = advanced.Route
	s.adoptProjection(advanced.Packets)
	d := w.dungeons.Dungeons[advanced.Route.Entry.Destinations[advanced.Route.First]]
	first, err := s.prepareFirstCombat(w, firstPortal(advanced.Route, d.Mazes[0].Start))
	if err != nil {
		t.Fatal(err)
	}
	w.apocalypseRun, w.activeDungeon = first.Route, first.Combat
	w.activeDungeon.Loaded = true
	// A single source-declared target, matching the live C39/ACK39 sequence.
	w.activeDungeon.Definition.SourceBoss = 200
	w.activeDungeon.Monsters = []protocol.DungeonMonster{{Entity: 4096, Template: 200, Rank: 3, Team: 100}}
	w.activeDungeon.Dead = map[uint16]bool{4096: true}
	w.deathSent = map[uint16]bool{4096: true}
	s.session.Phase = byte(w.apocalypseRun.Stage)
	return s, w, followingPortal(t, s, w)
}

func followingPortal(t *testing.T, s *legionSession, w *worldSession) []byte {
	t.Helper()
	next, ok := s.clock.Next(int64(w.apocalypseRun.PhaseKey))
	if !ok {
		t.Fatal("no fixture next phase")
	}
	d := w.dungeons.Dungeons[w.apocalypseRun.Entry.Destinations[next]]
	p := firstPortal(w.apocalypseRun, d.Mazes[0].Start)
	binary.LittleEndian.PutUint32(p[13:], d.ID)
	return p
}

func TestApocalypseFollowingCombatRequiresOwnedSourceDeath(t *testing.T) {
	changes := []func(*legionSession, *worldSession, []byte){
		func(s *legionSession, w *worldSession, p []byte) { w.activeDungeon.Dead[4096] = false },
		func(s *legionSession, w *worldSession, p []byte) { w.deathSent[4096] = false },
		func(s *legionSession, w *worldSession, p []byte) { w.activeDungeon.Definition.SourceBoss++ },
		func(s *legionSession, w *worldSession, p []byte) { w.activeDungeon.Monsters[0].Team = 0 },
		func(s *legionSession, w *worldSession, p []byte) { w.activeDungeon.Monsters[0].NonCombat = true },
		func(s *legionSession, w *worldSession, p []byte) { w.activeDungeon.Loaded = false },
		func(s *legionSession, w *worldSession, p []byte) { w.apocalypseRun.Character++ },
		func(s *legionSession, w *worldSession, p []byte) { s.party.Actor++ },
		func(s *legionSession, w *worldSession, p []byte) { s.catalog.SHA256 = "changed" },
		func(s *legionSession, w *worldSession, p []byte) { s.projection[41] = 255 },
		func(s *legionSession, w *worldSession, p []byte) {
			binary.LittleEndian.PutUint32(s.projection[109:], 0)
		},
		func(s *legionSession, w *worldSession, p []byte) { p[13]++ },
		func(s *legionSession, w *worldSession, p []byte) { p[17]++ },
		func(s *legionSession, w *worldSession, p []byte) { p[21]++ },
		func(s *legionSession, w *worldSession, p []byte) { p[25]++ },
		func(s *legionSession, w *worldSession, p []byte) { p[45]++ },
	}
	for i, change := range changes {
		s, w, p := combatProgressFixture(t)
		old := w.apocalypseRun
		change(s, w, p)
		if result, err := s.prepareCombatPortal(w, p); err == nil || result.Combat != nil || len(result.Packets) != 0 || w.apocalypseRun != old {
			t.Fatal("invalid combat transition accepted", i, err)
		}
	}
}

func TestApocalypseFollowingCombatUsesSourceDestination(t *testing.T) {
	s, w, _ := combatProgressFixture(t)
	// Editing the next source destination changes the target, with no second ID table.
	content := s.contents.Contents["Apocalypse"]
	d := w.dungeons.Dungeons[content.Dungeons[2].Dungeon]
	d.ID++
	w.dungeons.Dungeons[d.ID] = d
	content.Dungeons[2].Dungeon = d.ID
	s.contents.Contents["Apocalypse"] = content
	w.apocalypseRun.Entry.Destinations[2] = d.ID
	p := followingPortal(t, s, w)
	before := append([]byte(nil), s.projection...)
	r, err := s.prepareCombatPortal(w, p)
	if err != nil || r.Route.Stage != 2 || r.Route.PhaseKey != 2 || r.Combat.Definition.ID != d.ID {
		t.Fatal("source clock ignored", err)
	}
	expected := append([]byte(nil), before...)
	binary.LittleEndian.PutUint32(expected[13:], 2)
	expected[2+27+24] = 2
	if !bytes.Equal(r.Packets[0].Payload, expected) || !bytes.Equal(s.projection, before) || w.apocalypseRun.Stage != 1 {
		t.Fatal("prepared transition mutated live state or unrelated fields")
	}
	w.activeDungeon, w.apocalypseRun = r.Combat, r.Route
	if packets, err := w.completeDungeon(); err != nil || len(packets) != 0 {
		t.Fatal("ordinary settlement escaped", err)
	}
	if _, err := w.dungeonResult(make([]byte, 8)); err == nil {
		t.Fatal("ordinary reward escaped")
	}
}

func TestApocalypseFollowingCombatDeliveryIsAtomic(t *testing.T) {
	for _, broken := range []bool{false, true} {
		s, w, p := combatProgressFixture(t)
		old, active := w.apocalypseRun, w.activeDungeon
		projection := append([]byte(nil), s.projection...)
		prepared, err := s.prepareCombatPortal(w, p)
		if err != nil {
			t.Fatal(err)
		}
		a, peer := net.Pipe()
		peer.SetDeadline(time.Now().Add(5 * time.Second))
		log := func(map[string]any) {}
		c := &gameConnection{worldState: w, legionState: *s, bootstrapped: true, event: log, output: newConnectionOutput(a, make([]byte, wire.SessionKeyBytes), "test", log)}
		done := make(chan dispatchAction, 1)
		go func() {
			done <- c.dispatchLegion(&clientRequest{frame: wire.Frame{Type: 1, ID: 2062}, verified: true, plaintext: p})
		}()
		for i, packet := range prepared.Packets {
			if broken && i == len(prepared.Packets)-1 {
				peer.Close()
				break
			}
			f := readDispatchServerFrame(t, peer)
			if f[0] != packet.Kind || binary.LittleEndian.Uint16(f[1:]) != packet.ID {
				t.Fatal("wrong load frame")
			}
		}
		action := <-done
		if broken {
			if action != dispatchClose || w.apocalypseRun != old || w.activeDungeon != active || !bytes.Equal(c.legionState.projection, projection) || c.legionState.session.Phase != 1 {
				t.Fatal("partial delivery adopted")
			}
		} else {
			if action != dispatchHandled || w.apocalypseRun.Stage != 2 || w.apocalypseRun.PhaseKey != 2 || w.activeDungeon == active || w.activeDungeon.Loaded || len(w.deathSent) != 0 {
				t.Fatal("new combat owner not adopted")
			}
			w.activeDungeon.Loaded = true
			if r, err := c.legionState.prepareCombatPortal(w, p); err == nil || r.Combat != nil {
				t.Fatal("replay reused previous boss death")
			}
		}
		a.Close()
		peer.Close()
	}
}

func TestApocalypseLastCombatCannotManufactureSettlement(t *testing.T) {
	s, w, p := combatProgressFixture(t)
	w.apocalypseRun.PhaseKey = 5
	w.apocalypseRun.Stage = 5
	w.activeDungeon.Definition.ID = w.apocalypseRun.Entry.Destinations[5]
	binary.LittleEndian.PutUint32(s.projection[13:], 5)
	s.projection[2+27+60] = 5
	if r, err := s.prepareCombatPortal(w, p); err == nil || len(r.Packets) != 0 || r.Combat != nil {
		t.Fatal("last combat manufactured reward/portal")
	}
	// A foreign active session must never inherit a clear from this run.
	w.activeDungeon = &dungeon.Session{}
	if r, err := s.prepareCombatPortal(w, p); err == nil || r.Combat != nil {
		t.Fatal("foreign combat owner accepted")
	}
}
