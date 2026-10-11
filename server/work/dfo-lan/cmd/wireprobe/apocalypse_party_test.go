package main

import (
	"bytes"
	"dfolan/internal/catalog"
	"dfolan/internal/catalog/pvf"
	"dfolan/internal/character"
	"dfolan/internal/database"
	"dfolan/internal/dungeon"
	"dfolan/internal/game/protocol"
	"dfolan/internal/game/wire"
	"dfolan/internal/world"
	"encoding/binary"
	"encoding/hex"
	"net"
	"os"
	"reflect"
	"testing"
	"time"
)

// Native C12, DFO43560, verified checksum, 2026-10-04T14:33:04Z.
const apocalypsePartyLiveHex = "00000200000069690100000000040000002601000101020407070707ffffffff00000000000000000000000000000000"

func apocalypsePartyFixture(t *testing.T) (*legionSession, *worldSession, []byte) {
	t.Helper()
	cat, err := catalog.LoadApocalypseCatalog("../../configs/apocalypse.generated.json")
	if err != nil {
		t.Fatal(err)
	}
	s := &legionSession{catalog: cat, contents: catalog.LegionContents{Source: pvf.ArchiveSnapshot{Checksum: "fixture"}, Path: catalog.LegionContentsPath, Contents: map[string]catalog.LegionContent{"Apocalypse": {Complete: true, Recruiting: catalog.LegionAreaPoint{Town: 239, Area: 1}, Waiting: catalog.LegionAreaPoint{Town: 239, Area: 2}}}}}
	w := &worldSession{channelType: 119, level: 115, state: database.WorldState{Position: database.WorldPosition{Town: 239, Area: 2}}, role: database.Character{ID: 2, WireID: 2, Name: "owner", State: []byte(`{"level":115,"advancement":5,"source_sha256":"fixture","attributes":{"[hp max]":100,"[mp max]":100}}`)}, characters: &character.Service{ChannelContext: [2]byte{1, 30}}}
	body, err := hex.DecodeString(apocalypsePartyLiveHex)
	if err != nil {
		t.Fatal(err)
	}
	return s, w, body
}

func TestApocalypsePartyNativeCreateUpdateLeave(t *testing.T) {
	s, w, body := apocalypsePartyFixture(t)
	for _, capacity := range []byte{1, 4} {
		body[8] = capacity
		for _, action := range []byte{0, 0, 1} {
			body[0] = action
			result, err := s.handleParty(w, body, 12)
			if err != nil || len(result.Packets) != 3 {
				t.Fatal(result, err)
			}
			if s.party == nil || s.party.CharacterID != 2 || s.party.Options.Byte5 != 4 || s.party.Options.Word6 != 0 || s.party.Options.Title != "ii" || s.party.Options.Capacity != capacity || w.soloPartyReady || w.activeDungeon != nil {
				t.Fatal("not a real local owner", s.party)
			}
			for i, id := range []uint16{2, 2, 9} {
				if result.Packets[i].ID != id || result.Packets[i].Kind != 0 {
					t.Fatal("actor must precede native roster")
				}
			}
			p := result.Packets[2].Payload
			if len(p) != 114 || binary.LittleEndian.Uint32(p[14:]) != 2 || string(p[18:20]) != "ii" || p[22] != capacity || p[71] != 1 || binary.LittleEndian.Uint16(p[73:]) != 2 || p[31] != 38 || p[97] != 38 || p[108] != 87 || !bytes.Equal(p[109:], make([]byte, 5)) {
				t.Fatalf("native grammar=%x", p)
			}
		}
	}
	if result, err := s.handleParty(w, make([]byte, 8), 13); err != nil || len(result.Packets) != 1 || len(result.Packets[0].Payload) != 12 || result.Packets[0].Payload[9] != 3 || s.party != nil {
		t.Fatal("leave", result, err)
	}
	if _, err := s.handleParty(w, nil, 13); err == nil {
		t.Fatal("repeat leave invented owner")
	}
}

func TestApocalypsePartyRejectsWithoutMutation(t *testing.T) {
	for _, mutate := range []func(*legionSession, *worldSession, []byte){
		func(s *legionSession, w *worldSession, b []byte) { w.channelType = 99 },
		func(s *legionSession, w *worldSession, b []byte) { w.role.ID = 3 },
		func(s *legionSession, w *worldSession, b []byte) { w.state.Position.Area = 3 },
		func(s *legionSession, w *worldSession, b []byte) { w.activeDungeon = &dungeon.Session{} },
		func(s *legionSession, w *worldSession, b []byte) { w.characters = nil },
		func(s *legionSession, w *worldSession, b []byte) { b[17] = 31 },
		func(s *legionSession, w *worldSession, b []byte) { b[0] = 2 },
		func(s *legionSession, w *worldSession, b []byte) { b[1] = 1 },
		func(s *legionSession, w *worldSession, b []byte) { b[8] = 5 },
		func(s *legionSession, w *worldSession, b []byte) { s.catalog = nil },
	} {
		s, w, b := apocalypsePartyFixture(t)
		if _, err := s.handleParty(w, b, 12); err != nil {
			t.Fatal(err)
		}
		previous := *s.party
		mutate(s, w, b)
		result, err := s.handleParty(w, b, 12)
		if err == nil || len(result.Packets) != 0 || !reflect.DeepEqual(previous, *s.party) {
			t.Fatal("invalid request mutated owner", err)
		}
	}
	s, w, b := apocalypsePartyFixture(t)
	b[0] = 1
	if _, err := s.handleParty(w, b, 12); err == nil {
		t.Fatal("unowned update")
	}
	// The location follows the imported source, not a hard-coded239/2 check.
	c := s.contents.Contents["Apocalypse"]
	c.Waiting.Area = 6
	s.contents.Contents["Apocalypse"] = c
	b[0] = 0
	if _, err := s.handleParty(w, b, 12); err == nil {
		t.Fatal("ignored changed source waiting area")
	}
	w.state.Position.Area = 6
	if _, err := s.handleParty(w, b, 12); err != nil {
		t.Fatal(err)
	}
}

func TestApocalypsePartyActualRecruitingCreate(t *testing.T) {
	s, w, _ := apocalypsePartyFixture(t)
	// v6 manual14:58:17Z: title1, capacity1, current town239/area1.
	b, err := hex.DecodeString("000001000000310100000000000000002601000101020407070707ffffffff0000000000000000000000000000000000")
	if err != nil {
		t.Fatal(err)
	}
	c := s.contents.Contents["Apocalypse"]
	w.state.Position = database.WorldPosition{Town: c.Recruiting.Town, Area: c.Recruiting.Area, X: 780, Y: 372}
	before := w.state
	result, err := s.handleParty(w, b, 12)
	if err != nil || len(result.Packets) != 3 || s.party == nil || s.party.Options.Title != "1" || w.state != before {
		t.Fatal("native recruiting create", result, err)
	}
	// Creation publishes N9; it must not pre-empt the client's ensuing C36.
	s.clearCharacter()
	c.Recruiting.Town, c.Recruiting.Area = 301, 5
	s.contents.Contents["Apocalypse"] = c
	if _, err := s.handleParty(w, b, 12); err == nil {
		t.Fatal("ignored changed recruiting source")
	}
	w.state.Position.Town, w.state.Position.Area = 301, 5
	if _, err := s.handleParty(w, b, 12); err != nil {
		t.Fatal(err)
	}
}

func TestApocalypsePartyRecruitingNativePVF(t *testing.T) {
	path := os.Getenv("DFO_PVF_CORE_TEST_ARCHIVE")
	if path == "" {
		t.Skip("native archive required")
	}
	a, err := catalog.OpenTestArchiveCached(path, os.Getenv("DFO_PVF_CORE_TEST_SHA256"))
	if err != nil {
		t.Fatal(err)
	}
	contents, err := catalog.ImportLegionContents(a)
	if err != nil {
		t.Fatal(err)
	}
	operations, err := catalog.ImportApocalypse(a)
	if err != nil {
		t.Fatal(err)
	}
	worldCatalog, err := catalog.ImportWorldRuntime(a)
	if err != nil {
		t.Fatal(err)
	}
	s, w, b := apocalypsePartyFixture(t)
	s.catalog, s.contents = operations, contents
	c := contents.Contents["Apocalypse"]
	w.state.Position = database.WorldPosition{Town: c.Recruiting.Town, Area: c.Recruiting.Area, X: c.Recruiting.X, Y: c.Recruiting.Y}
	if _, err := s.handleParty(w, b, 12); err != nil {
		t.Fatal("source recruiting create rejected", err)
	}
	w.service = &world.Service{Catalog: worldCatalog, Rules: world.Rules{RequirePortalProximity: true}}
	request := protocol.AreaChangeRequest{PreviousTown: w.state.Position.Town, PreviousArea: uint16(w.state.Position.Area), Town: c.Waiting.Town, Area: c.Waiting.Area, X: c.Waiting.X, Y: c.Waiting.Y}
	_, moveErr := w.areaTransition(request)
	t.Logf("native source recruiting=%d/%d waiting=%d/%d ordinary portal transition=%v", c.Recruiting.Town, c.Recruiting.Area, c.Waiting.Town, c.Waiting.Area, moveErr)
	before := w.state
	next, err := s.partyAreaTransition(w, request)
	if err != nil || next.Town != c.Waiting.Town || next.Area != c.Waiting.Area || next.X != c.Waiting.X || next.Y != c.Waiting.Y || w.state != before {
		t.Fatal("source-owned C36 rejected or prematurely moved", next, err)
	}
	s.party = nil
	if _, err := s.partyAreaTransition(w, request); err == nil {
		t.Fatal("unowned recruiting warp accepted")
	}
}

func TestApocalypsePartyMoveAuthorization(t *testing.T) {
	for _, mutate := range []func(*legionSession, *worldSession, *protocol.AreaChangeRequest){
		func(s *legionSession, w *worldSession, r *protocol.AreaChangeRequest) { s.party = nil },
		func(s *legionSession, w *worldSession, r *protocol.AreaChangeRequest) { w.role.ID++ },
		func(s *legionSession, w *worldSession, r *protocol.AreaChangeRequest) { w.role.WireID++ },
		func(s *legionSession, w *worldSession, r *protocol.AreaChangeRequest) {
			w.characters.ChannelContext[0]++
		},
		func(s *legionSession, w *worldSession, r *protocol.AreaChangeRequest) { r.PreviousArea++ },
		func(s *legionSession, w *worldSession, r *protocol.AreaChangeRequest) { r.X++ },
		func(s *legionSession, w *worldSession, r *protocol.AreaChangeRequest) {
			w.activeDungeon = &dungeon.Session{}
		},
		func(s *legionSession, w *worldSession, r *protocol.AreaChangeRequest) { w.selectingDungeon = true },
		func(s *legionSession, w *worldSession, r *protocol.AreaChangeRequest) {
			w.service.Catalog.Source.Checksum = "other"
		},
		func(s *legionSession, w *worldSession, r *protocol.AreaChangeRequest) { w.level = 1 },
	} {
		s, w, b := apocalypsePartyFixture(t)
		w.state.Position = database.WorldPosition{Town: 239, Area: 1, X: 780, Y: 372}
		w.service = &world.Service{Catalog: catalog.WorldCatalog{Source: s.contents.Source, Areas: map[string]catalog.WorldArea{
			catalog.AreaKey(239, 1): {Walkable: [][4]int32{{0, 0, 1000, 1000}}},
			catalog.AreaKey(239, 2): {MinimumLevel: 115, Walkable: [][4]int32{{0, 0, 1000, 1000}}},
		}}}
		if _, err := s.handleParty(w, b, 12); err != nil {
			t.Fatal(err)
		}
		r := protocol.AreaChangeRequest{Town: 239, Area: 2, PreviousTown: 239, PreviousArea: 1}
		if _, err := s.partyAreaTransition(w, r); err != nil {
			t.Fatal("valid owner", err)
		}
		mutate(s, w, &r)
		before := w.state
		owner := s.party
		if _, err := s.partyAreaTransition(w, r); err == nil || w.state != before || s.party != owner {
			t.Fatal("invalid warp accepted or mutated session", err)
		}
	}
}

func TestApocalypsePartyDispatchEncryptedTransport(t *testing.T) {
	s, w, b := apocalypsePartyFixture(t)
	a, peer := net.Pipe()
	defer a.Close()
	defer peer.Close()
	peer.SetDeadline(time.Now().Add(5 * time.Second))
	keys := make([]byte, wire.SessionKeyBytes)
	log := func(map[string]any) {}
	c := &gameConnection{worldState: w, legionState: *s, bootstrapped: true, event: log, output: newConnectionOutput(a, keys, "test", log)}
	done := make(chan dispatchAction, 1)
	go func() {
		done <- c.dispatchLegion(&clientRequest{frame: wire.Frame{Type: 1, ID: 12}, verified: true, plaintext: b})
	}()
	for _, id := range []uint16{2, 2, 9} {
		frame := readDispatchServerFrame(t, peer)
		if frame[0] != 0 || binary.LittleEndian.Uint16(frame[1:]) != id {
			t.Fatal("wrong dispatch order")
		}
		plain, err := wire.DecryptPayload(keys, id, frame[wire.ServerHeaderSize:])
		if err != nil || len(plain) == 0 {
			t.Fatal(err)
		}
		if id == 9 {
			want, err := protocol.LegionPartyRoster115(1, [2]byte{1, 30}, [4]uint16{2}, 2, c.legionState.party.Options)
			if err != nil || !bytes.Equal(plain[:len(want)], want) {
				t.Fatal("native roster changed in transport", err)
			}
		}
	}
	if <-done != dispatchHandled || c.legionState.party == nil {
		t.Fatal("C12 still unhandled")
	}
}

func TestApocalypsePartyBrokenDeliveryRollsBack(t *testing.T) {
	s, w, b := apocalypsePartyFixture(t)
	a, peer := net.Pipe()
	peer.Close()
	defer a.Close()
	log := func(map[string]any) {}
	c := &gameConnection{worldState: w, legionState: *s, bootstrapped: true, event: log, output: newConnectionOutput(a, make([]byte, wire.SessionKeyBytes), "test", log)}
	if c.dispatchLegion(&clientRequest{frame: wire.Frame{Type: 1, ID: 12}, verified: true, plaintext: b}) != dispatchClose || c.legionState.party != nil {
		t.Fatal("broken delivery retained party")
	}
}

func TestApocalypsePartyCharacterReselectionClearsOwner(t *testing.T) {
	s, w, b := apocalypsePartyFixture(t)
	if _, err := s.handleParty(w, b, 12); err != nil {
		t.Fatal(err)
	}
	s.clearCharacter()
	w.role.ID, w.role.WireID = 3, 3
	if _, err := s.handleParty(w, b, 12); err != nil || s.party.CharacterID != 3 || s.party.Actor != 3 {
		t.Fatal("stale member after reselection", err)
	}
}
