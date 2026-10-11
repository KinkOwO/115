package main

import (
	"bytes"
	"dfolan/internal/catalog"
	"dfolan/internal/game/wire"
	"encoding/binary"
	"net"
	"strings"
	"testing"
	"time"
)

func directDeathFixture(t *testing.T) (*legionSession, *worldSession) {
	t.Helper()
	s, w, p := combatProgressFixture(t)
	second, err := s.prepareCombatPortal(w, p)
	if err != nil {
		t.Fatal(err)
	}
	w.apocalypseRun, w.activeDungeon = second.Route, second.Combat
	s.adoptProjection(second.Packets)
	s.session.Phase = 2
	w.activeDungeon.Loaded = true
	boss := w.apocalypseRun.Entry.Bosses[w.apocalypseRun.PhaseKey]
	w.activeDungeon.Definition.SourceBoss = boss
	w.activeDungeon.Monsters = w.apocalypseNavigation.Monsters[:1]
	w.activeDungeon.Monsters = append(w.activeDungeon.Monsters[:0:0], w.activeDungeon.Monsters...)
	w.activeDungeon.Monsters[0].Template = boss
	w.activeDungeon.Monsters[0].Team = 100
	w.activeDungeon.Dead[4096] = true
	w.deathSent = map[uint16]bool{4096: true}
	s.catalog.BossMoves = []catalog.ApocalypseBossMove{{Boss: boss, MonsterPath: "native.mob", MonsterHash: strings.Repeat("a", 64), ActionPath: "native.act", ActionHash: strings.Repeat("b", 64)}}
	return s, w
}

func TestApocalypseDirectDeathRequiresDeliveredSourceTarget(t *testing.T) {
	for i, change := range []func(*legionSession, *worldSession){
		func(s *legionSession, w *worldSession) { w.activeDungeon.Dead[4096] = false },
		func(s *legionSession, w *worldSession) { w.deathSent[4096] = false },
		func(s *legionSession, w *worldSession) { w.activeDungeon.Monsters[0].Template++ },
		func(s *legionSession, w *worldSession) { s.catalog.BossMoves = nil },
		func(s *legionSession, w *worldSession) { s.catalog.BossMoves[0].Boss++ },
		func(s *legionSession, w *worldSession) { s.catalog.BossMoves[0].ActionHash = "" },
		func(s *legionSession, w *worldSession) { w.apocalypseRun.Entry.Bosses[2]++ },
		func(s *legionSession, w *worldSession) { s.party.Actor++ },
		func(s *legionSession, w *worldSession) { w.activeDungeon.Loaded = false },
		func(s *legionSession, w *worldSession) {
			s.catalog.BossMoves[0].Operations = []uint32{w.apocalypseRun.Operation + 1}
		},
	} {
		s, w := directDeathFixture(t)
		old := w.apocalypseRun
		change(s, w)
		r, _ := s.prepareCombatDeathAdvance(w)
		if r.Route != nil || len(r.Packets) != 0 || w.apocalypseRun != old {
			t.Fatal("invalid death advanced", i)
		}
	}
}

func TestApocalypseDirectDeathPublishesBeforeNativePortal(t *testing.T) {
	s, w := directDeathFixture(t)
	old, active := w.apocalypseRun, w.activeDungeon
	before := append([]byte(nil), s.projection...)
	r, err := s.prepareCombatDeathAdvance(w)
	if err != nil || r.Route == nil || r.Route.Stage != 3 || r.Route.PendingPhase != 3 || r.Route.PhaseKey != 2 || r.Route.Active != active || w.apocalypseRun != old {
		t.Fatal("death preparation changed active combat", err)
	}
	want := append([]byte(nil), before...)
	binary.LittleEndian.PutUint32(want[13:], 3)
	want[2+27+36] = 3
	if !bytes.Equal(r.Packets[0].Payload, want) || !bytes.Equal(s.projection, before) {
		t.Fatal("death advance changed unrelated role/route fields")
	}
	w.apocalypseRun = r.Route
	s.adoptProjection(r.Packets)
	if again, err := s.prepareCombatDeathAdvance(w); err != nil || again.Route != nil || len(again.Packets) != 0 {
		t.Fatal("repeated death advanced twice", err)
	}
	if _, leave, err := s.prepareApocalypseRetreat(w, apocalypseRetreatBody()); err != nil || !leave {
		t.Fatal("pending native portal blocked retreat", err)
	}
	portal := followingPortal(t, s, w)
	third, err := s.prepareCombatPortal(w, portal)
	if err != nil || third.Route.Stage != 3 || third.Route.PhaseKey != 3 || third.Route.PendingPhase != 0 {
		t.Fatal("native portal advanced a second time", err)
	}
	for _, p := range third.Packets {
		if p.Name == "apocalypse_combat_phase_advanced" {
			t.Fatal("already-delivered progression republished at loading")
		}
	}
	stale := append([]byte(nil), portal...)
	binary.LittleEndian.PutUint32(stale[13:], active.Definition.ID)
	if denied, err := s.prepareCombatPortal(w, stale); err == nil || denied.Combat != nil {
		t.Fatal("old current-map portal accepted")
	}
}

func TestApocalypseDeathDispatchAdoptsPhaseOnlyAfterDelivery(t *testing.T) {
	for _, broken := range []bool{false, true} {
		s, w := directDeathFixture(t)
		old, active := w.apocalypseRun, w.activeDungeon
		w.quests = nil
		w.loot = nil
		w.progression = nil
		p := make([]byte, 112)
		binary.LittleEndian.PutUint32(p, 4096)
		binary.LittleEndian.PutUint16(p[4:], w.role.WireID)
		plan, err := w.monsterDeath(p, func(map[string]any) {})
		if err != nil {
			t.Fatal(err)
		}
		a, peer := net.Pipe()
		peer.SetDeadline(time.Now().Add(5 * time.Second))
		log := func(map[string]any) {}
		key := make([]byte, wire.SessionKeyBytes)
		c := &gameConnection{worldState: w, legionState: *s, bootstrapped: true, event: log, keys: key, output: newConnectionOutput(a, key, "test", log)}
		done := make(chan dispatchAction, 1)
		go func() {
			done <- c.dispatchDungeon(&clientRequest{frame: wire.Frame{Type: 1, ID: 39}, verified: true, plaintext: p})
		}()
		for _, packet := range plan {
			f := readDispatchServerFrame(t, peer)
			if f[0] != packet.Kind || binary.LittleEndian.Uint16(f[1:]) != packet.ID {
				t.Fatal("unexpected death response")
			}
		}
		if broken {
			peer.Close()
		} else {
			f := readDispatchServerFrame(t, peer)
			if binary.LittleEndian.Uint16(f[1:]) != 2895 {
				t.Fatal("next target not delivered after death")
			}
		}
		action := <-done
		if broken {
			if action != dispatchClose || w.apocalypseRun != old || c.legionState.session.Phase != 2 {
				t.Fatal("failed next-target delivery adopted")
			}
		} else {
			if action != dispatchHandled || w.apocalypseRun.Stage != 3 || w.apocalypseRun.PendingPhase != 3 || w.activeDungeon != active || c.legionState.session.Phase != 3 {
				t.Fatal("delivered pending phase not adopted")
			}
		}
		a.Close()
		peer.Close()
	}
}
