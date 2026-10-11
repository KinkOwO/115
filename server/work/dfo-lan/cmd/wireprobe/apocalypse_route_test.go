package main

import (
	"bytes"
	"dfolan/internal/dungeon"
	"dfolan/internal/game/protocol"
	"dfolan/internal/game/wire"
	"dfolan/internal/legion"
	"encoding/binary"
	"net"
	"testing"
	"time"
)

func initialRouteFixture(t *testing.T) (*legionSession, *worldSession, []byte) {
	s, w := navigationEntryFixture(t)
	entry, err := s.handle(w, navigationRequest(), legion.CmdEnterDungeon)
	if err != nil {
		t.Fatal(err)
	}
	s.adoptProjection(entry.Packets)
	w.activeDungeon, w.apocalypseNavigation, w.apocalypseRun = entry.Navigation, entry.Navigation, entry.Route
	w.activeDungeon.Loaded = true
	w.activeDungeon.Monsters = []protocol.DungeonMonster{{Entity: 4096, Rank: 3}}
	w.activeDungeon.Dead = map[uint16]bool{4096: true}
	w.deathSent = map[uint16]bool{4096: true}
	role, err := s.handle(w, rolePayload(1, legion.OperationChannelCode), legion.CmdRoleSelect)
	if err != nil {
		t.Fatal(err)
	}
	s.adoptProjection(role.Packets)
	p := make([]byte, 16)
	binary.LittleEndian.PutUint16(p, w.role.WireID)
	binary.LittleEndian.PutUint16(p[2:], 4096)
	return s, w, p
}

func firstPortal(r *apocalypseRoute, grid [2]byte) []byte {
	p := make([]byte, 48)
	binary.LittleEndian.PutUint32(p[13:], r.Entry.Destinations[r.First])
	binary.LittleEndian.PutUint32(p[17:], uint32(r.Active.Difficulty))
	binary.LittleEndian.PutUint32(p[21:], uint32(grid[0]))
	binary.LittleEndian.PutUint32(p[25:], uint32(grid[1]))
	return p
}

func TestApocalypseInitialRouteRequiresDeliveredTargetAndRealDeath(t *testing.T) {
	for _, change := range []func(*legionSession, *worldSession){
		func(s *legionSession, w *worldSession) { w.activeDungeon.Dead[4096] = false },
		func(s *legionSession, w *worldSession) { w.deathSent[4096] = false },
		func(s *legionSession, w *worldSession) { w.activeDungeon.Loaded = false },
		func(s *legionSession, w *worldSession) { s.projection[41] = 255 },
		func(s *legionSession, w *worldSession) { w.apocalypseRun.Character++ },
		func(s *legionSession, w *worldSession) { w.apocalypseRun.Operation++ },
		func(s *legionSession, w *worldSession) { w.activeDungeon = &dungeon.Session{} },
		func(s *legionSession, w *worldSession) { s.contents.Source.Checksum = "another-source" },
		func(s *legionSession, w *worldSession) { s.catalog.Source = "another.ctp" },
		func(s *legionSession, w *worldSession) { s.catalog.SHA256 = "changed-script" },
		func(s *legionSession, w *worldSession) { binary.LittleEndian.PutUint32(s.projection[109:], 0) },
		func(s *legionSession, w *worldSession) { binary.LittleEndian.PutUint32(s.projection[113:], 1) },
	} {
		s, w, p := initialRouteFixture(t)
		change(s, w)
		if result, err := s.prepareNavigationAdvance(w, p); err == nil || len(result.Packets) != 0 {
			t.Fatal("invalid navigation owner/death advanced")
		}
	}
	s, w, p := initialRouteFixture(t)
	before := append([]byte(nil), s.projection...)
	result, err := s.prepareNavigationAdvance(w, p)
	if err != nil || result.Route.Stage != 1 || w.apocalypseRun.Stage != 0 || s.session.Phase != 0 {
		t.Fatal("prepare mutated progression", err)
	}
	expected := append([]byte(nil), before...)
	binary.LittleEndian.PutUint32(expected[13:], 1)
	if !bytes.Equal(result.Packets[0].Payload, expected) || !bytes.Equal(s.projection, before) {
		t.Fatal("progression reset role or unrelated native fields")
	}
	if packets, err := w.completeDungeon(); err != nil || len(packets) != 0 {
		t.Fatal("navigation created ordinary settlement", err)
	}
}

func TestApocalypseCombatDispatchFailureKeepsNavigationOwner(t *testing.T) {
	for _, broken := range []bool{false, true} {
		s, w, p := initialRouteFixture(t)
		advanced, err := s.prepareNavigationAdvance(w, p)
		if err != nil {
			t.Fatal(err)
		}
		w.apocalypseRun = advanced.Route
		s.adoptProjection(advanced.Packets)
		s.session.Phase = 1
		previous, nav := w.apocalypseRun, w.activeDungeon
		d := w.dungeons.Dungeons[previous.Entry.Destinations[previous.First]]
		portal := firstPortal(previous, d.Mazes[0].Start)
		prepared, err := s.prepareFirstCombat(w, portal)
		if err != nil {
			t.Fatal(err)
		}
		a, peer := net.Pipe()
		peer.SetDeadline(time.Now().Add(5 * time.Second))
		log := func(map[string]any) {}
		c := &gameConnection{worldState: w, legionState: *s, bootstrapped: true, event: log, output: newConnectionOutput(a, make([]byte, wire.SessionKeyBytes), "test", log)}
		done := make(chan dispatchAction, 1)
		go func() {
			done <- c.dispatchLegion(&clientRequest{frame: wire.Frame{Type: 1, ID: 2062}, verified: true, plaintext: portal})
		}()
		for index, packet := range prepared.Packets {
			if broken && index == len(prepared.Packets)-1 {
				peer.Close()
				break
			}
			frame := readDispatchServerFrame(t, peer)
			if frame[0] != packet.Kind || binary.LittleEndian.Uint16(frame[1:]) != packet.ID {
				t.Fatal("wrong combat load frame")
			}
		}
		action := <-done
		if broken {
			if action != dispatchClose || w.activeDungeon != nav || w.apocalypseRun != previous {
				t.Fatal("partial combat delivery adopted")
			}
		} else {
			if action != dispatchHandled || w.activeDungeon == nav || w.apocalypseRun.Active != w.activeDungeon || w.activeDungeon.Loaded {
				t.Fatal("combat owner not adopted")
			}
			if again, err := c.legionState.prepareFirstCombat(w, portal); err == nil || again.Combat != nil {
				t.Fatal("duplicate portal replaced owned combat")
			}
		}
		a.Close()
		peer.Close()
	}
}

func TestApocalypseFirstCombatUsesSourceAndRetainsContentOwnership(t *testing.T) {
	s, w, p := initialRouteFixture(t)
	advance, err := s.prepareNavigationAdvance(w, p)
	if err != nil {
		t.Fatal(err)
	}
	w.apocalypseRun = advance.Route
	s.adoptProjection(advance.Packets)
	definition := w.dungeons.Dungeons[w.apocalypseRun.Entry.Destinations[w.apocalypseRun.First]]
	portal := firstPortal(w.apocalypseRun, definition.Mazes[0].Start)
	result, err := s.prepareFirstCombat(w, portal)
	if err != nil || result.Combat == nil || result.Route.Active != result.Combat || !result.Combat.Definition.NoFatigue || w.apocalypseRun.Active != w.activeDungeon {
		t.Fatal("source combat preparation", err)
	}
	var sequence []uint16
	for _, packet := range result.Packets {
		if packet.ID == 16 || packet.ID == 9 || packet.ID == 115 || packet.ID == 2062 || packet.ID == 2281 {
			t.Fatal("ordinary ACK/party/settlement injected")
		}
		if packet.ID == 27 || packet.ID == 2281 || packet.ID == 28 || packet.ID == 29 {
			sequence = append(sequence, packet.ID)
		}
	}
	if len(sequence) != 3 || sequence[0] != 27 || sequence[1] != 28 || sequence[2] != 29 {
		t.Fatal("native load sequence", sequence)
	}
	for _, offset := range []int{13, 17, 21, 25, 45} {
		changed := append([]byte(nil), portal...)
		changed[offset]++
		if denied, err := s.prepareFirstCombat(w, changed); err == nil || denied.Combat != nil {
			t.Fatal("unowned portal accepted", offset)
		}
	}
	// Changing the COS destination changes the selected battle; the opcode
	// contract has no parallel content-ID map.
	content := s.contents.Contents["Apocalypse"]
	old := content.Dungeons[w.apocalypseRun.First].Dungeon
	donor := w.dungeons.Dungeons[old]
	donor.ID++
	w.dungeons.Dungeons[donor.ID] = donor
	content.Dungeons[w.apocalypseRun.First].Dungeon = donor.ID
	s.contents.Contents["Apocalypse"] = content
	w.apocalypseRun.Entry.Destinations[w.apocalypseRun.First] = donor.ID
	binary.LittleEndian.PutUint32(portal[13:], donor.ID)
	changed, err := s.prepareFirstCombat(w, portal)
	if err != nil || changed.Combat.Definition.ID != donor.ID {
		t.Fatal("source target edit did not change execution", err)
	}
	w.apocalypseRun, w.activeDungeon = changed.Route, changed.Combat
	if !w.ownsApocalypseDungeon() {
		t.Fatal("combat lost content owner")
	}
	if packets, err := w.completeDungeon(); err != nil || len(packets) != 0 {
		t.Fatal("combat escaped to ordinary completion")
	}
	if _, err := w.dungeonResult(make([]byte, 8)); err == nil {
		t.Fatal("combat escaped to ordinary reward")
	}
}

func TestApocalypseRouteDispatchAdoptsOnlyAfterDelivery(t *testing.T) {
	for _, broken := range []bool{false, true} {
		s, w, p := initialRouteFixture(t)
		previous := w.apocalypseRun
		a, peer := net.Pipe()
		peer.SetDeadline(time.Now().Add(5 * time.Second))
		if broken {
			peer.Close()
		}
		log := func(map[string]any) {}
		c := &gameConnection{worldState: w, legionState: *s, bootstrapped: true, event: log, output: newConnectionOutput(a, make([]byte, wire.SessionKeyBytes), "test", log)}
		done := make(chan dispatchAction, 1)
		go func() {
			done <- c.dispatchLegion(&clientRequest{frame: wire.Frame{Type: 1, ID: 117}, verified: true, plaintext: p})
		}()
		if !broken {
			frame := readDispatchServerFrame(t, peer)
			if frame[0] != 0 || binary.LittleEndian.Uint16(frame[1:]) != legion.NotiLegionInfo {
				t.Fatal("wrong stage packet")
			}
		}
		action := <-done
		if broken {
			if action != dispatchClose || w.apocalypseRun != previous || c.legionState.session.Phase != 0 {
				t.Fatal("failed stage delivery adopted")
			}
		} else {
			if action != dispatchHandled || w.apocalypseRun.Stage != 1 || c.legionState.session.Phase != 1 {
				t.Fatal("delivered stage not adopted")
			}
		}
		a.Close()
		peer.Close()
	}
}
