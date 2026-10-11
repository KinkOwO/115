package main

import (
	"context"
	"dfolan/internal/catalog"
	"dfolan/internal/catalog/pvf"
	"dfolan/internal/database"
	"dfolan/internal/game/protocol"
	"dfolan/internal/game/wire"
	"dfolan/internal/world"
	"encoding/binary"
	"fmt"
	"net"
	"testing"
	"time"
)

type raidRoomWorldStore struct {
	saved   database.WorldState
	loads   int
	typ     uint32
	initial database.WorldPosition
}

func (s *raidRoomWorldStore) LoadWorld(_ context.Context, _, _ int64, typ uint32, initial database.WorldPosition, version string) (database.WorldState, error) {
	s.loads++
	s.typ, s.initial = typ, initial
	state := s.saved
	if state.ConfigVersion == "" {
		state.ConfigVersion = version
	}
	return state, nil
}

func raidRoomWorld(entry catalog.RaidEntrance, store *raidRoomWorldStore) *worldSession {
	areas := map[string]catalog.WorldArea{}
	for _, room := range []catalog.TownArea{*entry.Recruiting, entry.Waiting} {
		areas[catalog.AreaKey(room.TownID, room.AreaID)] = catalog.WorldArea{Town: room.TownID, Area: room.AreaID, MinimumLevel: room.MinimumLevel, Walkable: room.Walkable}
	}
	return &worldSession{channelType: 112, channelWorldIsolated: true, raidEntrance: &entry, account: 1,
		service: &world.Service{Store: store, Catalog: catalog.WorldCatalog{Source: entry.Recruiting.Source, Areas: areas}}}
}

func raidRoomFixture() catalog.RaidEntrance {
	return catalog.RaidEntrance{MemberMax: 12, StartMinimum: 1, PartyMax: 3,
		Recruiting: &catalog.TownArea{Source: pvf.ArchiveSnapshot{Checksum: "room-fixture"}, TownID: 701, AreaID: 7, MinimumLevel: 115, Walkable: [][4]int32{{100, 100, 400, 200}}},
		Waiting:    catalog.TownArea{TownID: 701, AreaID: 9, MinimumLevel: 115, Walkable: [][4]int32{{200, 100, 400, 200}}}}
}

func raidRoomRole() database.Character {
	return database.Character{ID: 1, WireID: 9, Profession: 16, Name: "owner", State: []byte(`{"level":115,"advancement":3,"awakening":3}`)}
}

func roomAreaRequest(from, to database.WorldPosition) []byte {
	p := binary.LittleEndian.AppendUint32(nil, to.Town)
	p = binary.LittleEndian.AppendUint32(p, to.Area)
	p = binary.LittleEndian.AppendUint16(p, to.X)
	p = binary.LittleEndian.AppendUint16(p, to.Y)
	p = append(p, 0)
	p = binary.LittleEndian.AppendUint32(p, from.Town)
	p = binary.LittleEndian.AppendUint16(p, uint16(from.Area))
	return append(p, 0, 0)
}

func TestRaidPreparationRestoresSourceInsteadOfOrphanSavedRoom(t *testing.T) {
	entry := raidRoomFixture()
	for _, old := range []database.WorldPosition{
		{Town: entry.Waiting.TownID, Area: entry.Waiting.AreaID, X: 400, Y: 200},
		{Town: 9999, Area: 9999, X: 65535, Return: &database.WorldReturn{Town: 3, Area: 4}},
	} {
		store := &raidRoomWorldStore{saved: database.WorldState{Position: old, Revision: 53}}
		w := raidRoomWorld(entry, store)
		w.raidWaiting = true
		if err := w.enter(raidRoomRole(), database.WorldPosition{Town: 3, Area: 4}); err != nil {
			t.Fatal(err)
		}
		x, y := entry.Recruiting.Spawn()
		want := database.WorldPosition{Town: entry.Recruiting.TownID, Area: entry.Recruiting.AreaID, X: x, Y: y}
		if w.state.Position != want || w.raidWaiting || w.state.Revision != 53 || store.typ != 112 || store.initial != want || store.saved.Position != old {
			t.Fatalf("fresh preparation polluted saved state: position=%+v saved=%+v", w.state, store.saved)
		}
		// A real CMD35 goes through validation and the persistence branch. The
		// SQL store is deliberately nil: writing it would panic this regression.
		body := binary.LittleEndian.AppendUint16(nil, x+1)
		body = binary.LittleEndian.AppendUint16(body, y)
		body = append(body, 0, 0, 0)
		if err := w.handle(35, body, func(byte, uint16, []byte) error { return nil }, func(map[string]any) {}); err != nil {
			t.Fatal(err)
		}
		if w.state.Position.X != x+1 || w.state.Revision != 53 || store.loads != 1 || store.saved.Position != old {
			t.Fatal("room movement persisted or reverted the session position")
		}
	}
	store := &raidRoomWorldStore{saved: database.WorldState{ConfigVersion: "incompatible-save"}}
	w := raidRoomWorld(entry, store)
	if err := w.enter(raidRoomRole(), database.WorldPosition{}); err == nil {
		t.Fatal("transient room entry bypassed save identity validation")
	}
}

func TestRaidRoomAreaRequestsCannotBypassTeamEntryOrLeave(t *testing.T) {
	entry := raidRoomFixture()
	x, y := entry.Recruiting.Spawn()
	prep := database.WorldPosition{Town: entry.Recruiting.TownID, Area: entry.Recruiting.AreaID, X: x, Y: y}
	x, y = entry.Waiting.Spawn()
	waiting := database.WorldPosition{Town: entry.Waiting.TownID, Area: entry.Waiting.AreaID, X: x, Y: y}
	for _, waitingOwned := range []bool{false, true} {
		w := raidRoomWorld(entry, &raidRoomWorldStore{})
		w.role, w.level, w.raidWaiting = raidRoomRole(), 115, waitingOwned
		from, to := prep, waiting
		if waitingOwned {
			from, to = waiting, prep
		}
		w.state.Position = from
		var sent []outboundPacket
		send := func(kind byte, id uint16, body []byte) error {
			sent = append(sent, outboundPacket{Kind: kind, ID: id, Payload: body})
			return nil
		}
		if err := w.handle(36, roomAreaRequest(from, to), send, func(map[string]any) {}); err != nil {
			t.Fatal(err)
		}
		if len(sent) != 1 || sent[0].Kind != 1 || sent[0].ID != 36 || sent[0].Payload[0] != 0 || w.state.Position != from || w.raidWaiting != waitingOwned {
			t.Fatalf("area request bypassed team boundary: %+v", sent)
		}
		// Scene synchronization in either legitimate room stays functional.
		sent = nil
		if err := w.handle(36, roomAreaRequest(from, from), send, func(map[string]any) {}); err != nil {
			t.Fatal(err)
		}
		if len(sent) != 3 || sent[0].Payload[0] != 1 || sent[1].ID != 23 || sent[2].ID != 24 || w.state.Position != from {
			t.Fatal("owned room synchronization failed")
		}
	}
}

func TestRaidSameRoleOtherConnectionCannotQueryAssignOrLeave(t *testing.T) {
	t.Setenv("DFO_RAID_OPEN_EVENTS_PROBE", "1")
	runtime := &gatewayRuntime{raidEntrances: map[uint32]catalog.RaidEntrance{82: {MemberMax: 12}}}
	team, err := runtime.raidTeams.create(1, 82, 1, protocol.RaidRecruitment{Create: protocol.RaidCreateRequest{Kind: 8, Title: "owned"}, Leader: 9, LeaderName: "owner", MemberCount: 1}, database.WorldPosition{})
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range []uint16{1353, 661, 657} {
		server, peer := net.Pipe()
		peer.SetDeadline(time.Now().Add(5 * time.Second))
		log := func(map[string]any) {}
		c := &gameConnection{gatewayRuntime: runtime, worldState: &worldSession{role: raidRoomRole(), channelType: 82, raidWaiting: true}, channel: 82, bootstrapped: true, event: log, output: newConnectionOutput(server, make([]byte, wire.SessionKeyBytes), "test", log)}
		body := []byte{2, 0}
		if id == 1353 {
			body = binary.LittleEndian.AppendUint32([]byte{1}, team.Recruitment.ID)
		}
		if id == 661 {
			body = binary.LittleEndian.AppendUint32(nil, 0)
			body = binary.LittleEndian.AppendUint32(body, 9)
			body = binary.LittleEndian.AppendUint32(body, 3)
		}
		result := make(chan dispatchAction, 1)
		go func() {
			result <- c.dispatchRaidEntrance(&clientRequest{frame: wire.Frame{Type: 1, ID: id}, plaintext: body, verified: true})
		}()
		frame := readDispatchServerFrame(t, peer)
		reply, err := wire.DecryptPayload(make([]byte, wire.SessionKeyBytes), id, frame[wire.ServerHeaderSize:])
		if err != nil || frame[0] != 1 || binary.LittleEndian.Uint16(frame[1:3]) != id || len(reply) == 0 || reply[0] != 0 || <-result != dispatchHandled {
			t.Fatalf("other connection accepted CMD%d", id)
		}
		after, ok := runtime.raidTeams.get(1, 82)
		if !ok || after.Recruitment != team.Recruitment {
			t.Fatal("other connection mutated team")
		}
		server.Close()
		peer.Close()
	}
}

func TestRaidDisconnectReleaseDoesNotRemoveNewAllocation(t *testing.T) {
	runtime := &gatewayRuntime{}
	origin := database.WorldPosition{Town: 701, Area: 7, X: 200, Y: 200}
	entry := protocol.RaidRecruitment{Leader: 9, LeaderName: "owner", MemberCount: 1}
	team, err := runtime.raidTeams.create(1, 112, 1, entry, origin)
	if err != nil {
		t.Fatal(err)
	}
	w := &worldSession{role: raidRoomRole(), raidWaiting: true, soloPartyReady: true, bakalParty: 3, bakalTown: 701, state: database.WorldState{Position: database.WorldPosition{Town: 701, Area: 9}}}
	c := &gameConnection{gatewayRuntime: runtime, channel: 112, worldState: w, raidOwnerRole: 1, ownedRaidID: team.Recruitment.ID}
	c.releaseOwnedRaid()
	if _, ok := runtime.raidTeams.get(1, 112); ok || c.ownedRaidID != 0 || c.raidOwnerRole != 0 || w.raidWaiting || w.soloPartyReady || w.bakalParty != 0 || w.bakalTown != 0 || w.state.Position != origin {
		t.Fatal("disconnect retained room state")
	}
	next, err := runtime.raidTeams.create(1, 112, 1, entry, origin)
	if err != nil {
		t.Fatal(err)
	}
	c.raidOwnerRole, c.ownedRaidID = 1, team.Recruitment.ID
	c.releaseOwnedRaid()
	if got, ok := runtime.raidTeams.get(1, 112); !ok || got.Recruitment.ID != next.Recruitment.ID {
		t.Fatal("stale disconnect removed new allocation")
	}
	if c.ownedRaidID != 0 {
		t.Fatal("stale connection kept ownership")
	}
}

func TestRaidWaitingRoomPresenceIsPrivateAndLeaveReturnsToLobby(t *testing.T) {
	hub := newLanHub()
	first, second := newTestPeer(1, 9), newTestPeer(2, 10)
	first.channel, second.channel = 112, 112
	hub.add(first)
	hub.add(second)
	prep := database.WorldPosition{Town: 701, Area: 7, X: 300, Y: 200}
	w1 := &worldSession{hub: hub, peer: first, role: database.Character{ID: 1, WireID: 9}, state: database.WorldState{Position: prep}}
	w2 := &worldSession{hub: hub, peer: second, role: database.Character{ID: 2, WireID: 10}, state: database.WorldState{Position: prep}}
	w1.enterArea()
	w2.enterArea()
	if len(hub.roster(first)) != 2 {
		t.Fatal("recruiting scene did not share its public roster")
	}
	for _, w := range []*worldSession{w1, w2} {
		w.raidWaiting = true
		w.state.Position = database.WorldPosition{Town: 701, Area: 9, X: 400, Y: 200}
		w.enterArea()
	}
	for _, peer := range []*lanPeer{first, second} {
		if roster := hub.roster(peer); len(roster) != 1 || roster[0].actorID != peer.actorID {
			t.Fatal("waiting room exposed another team's actor")
		}
	}
	for _, w := range []*worldSession{w1, w2} {
		w.raidWaiting = false
		w.state.Position = prep
		w.enterArea()
	}
	if len(hub.roster(first)) != 2 {
		t.Fatal("leaving waiting room did not restore public presence")
	}
}

func TestRaidCreateDeliveryFailureAtEveryRoomStage(t *testing.T) {
	t.Setenv("DFO_RAID_OPEN_EVENTS_PROBE", "1")
	for stage := 0; stage < 4; stage++ {
		t.Run(fmt.Sprint(stage), func(t *testing.T) {
			server, peer := net.Pipe()
			defer server.Close()
			defer peer.Close()
			peer.SetDeadline(time.Now().Add(5 * time.Second))
			log := func(map[string]any) {}
			origin := database.WorldPosition{Town: 152, Area: 1, X: 500, Y: 250}
			runtime := &gatewayRuntime{raidEntrances: map[uint32]catalog.RaidEntrance{82: {MemberMax: 12, StartMinimum: 1, PartyMax: 3, Waiting: catalog.TownArea{TownID: 152, AreaID: 2, Walkable: [][4]int32{{100, 100, 400, 200}}}}}}
			w := &worldSession{role: raidRoomRole(), level: 115, channelType: 82, state: database.WorldState{Position: origin}}
			c := &gameConnection{gatewayRuntime: runtime, channel: 82, worldState: w, bootstrapped: true, event: log, output: newConnectionOutput(server, make([]byte, wire.SessionKeyBytes), "test", log)}
			c.channelCfg.ServerID = 1
			body := binary.LittleEndian.AppendUint32([]byte{8}, 1)
			body = append(body, 'x', 0, 0, 0, 0, 0, 0)
			body = binary.LittleEndian.AppendUint32(body, 0)
			result := make(chan dispatchAction, 1)
			go func() {
				result <- c.dispatchRaidEntrance(&clientRequest{frame: wire.Frame{Type: 1, ID: 656}, plaintext: body, verified: true})
			}()
			for i := 0; i < stage; i++ {
				readDispatchServerFrame(t, peer)
			}
			peer.Close()
			if <-result != dispatchClose || c.raidOwnerRole != 0 || c.ownedRaidID != 0 || w.raidWaiting || w.state.Position != origin {
				t.Fatal("failed room delivery retained partial state")
			}
			if _, ok := runtime.raidTeams.get(1, 82); ok {
				t.Fatal("failed room delivery retained registry entry")
			}
		})
	}
}
