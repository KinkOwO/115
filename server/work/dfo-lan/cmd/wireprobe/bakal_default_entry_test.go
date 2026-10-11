package main

import (
	"dfolan/internal/catalog"
	"dfolan/internal/database"
	"dfolan/internal/game/wire"
	"encoding/binary"
	"encoding/hex"
	"net"
	"testing"
	"time"
)

func TestBakalCreateCapturedRequestOnDefaultProfile(t *testing.T) {
	t.Setenv("DFO_RAID_OPEN_EVENTS_PROBE", "")
	server, peer := net.Pipe()
	defer server.Close()
	defer peer.Close()
	peer.SetDeadline(time.Now().Add(5 * time.Second))
	var created bool
	log := func(e map[string]any) {
		if e["kind"] == "raid_created_waiting" {
			created = true
		}
	}
	origin := database.WorldPosition{Town: 152, Area: 1, X: 500, Y: 250}
	runtime := &gatewayRuntime{raidEntrances: map[uint32]catalog.RaidEntrance{82: {MemberMax: 12, StartMinimum: 1, PartyMax: 3, Waiting: catalog.TownArea{TownID: 152, AreaID: 2, Walkable: [][4]int32{{100, 100, 400, 200}}}}}}
	w := &worldSession{role: raidRoomRole(), level: 115, channelType: 82, state: database.WorldState{Position: origin}}
	c := &gameConnection{gatewayRuntime: runtime, channel: 82, worldState: w, bootstrapped: true, event: log, output: newConnectionOutput(server, make([]byte, wire.SessionKeyBytes), "test", log)}
	c.channelCfg.ServerID = 1
	// Four real create attempts in the 2026-10-07 01:04 local session.
	body, _ := hex.DecodeString("08010000003100000007000000000000")
	done := make(chan dispatchAction, 1)
	go func() {
		done <- c.dispatch(&clientRequest{frame: wire.Frame{Type: 1, ID: 656}, plaintext: body, verified: true})
		server.Close()
	}()
	for _, id := range []uint16{578, 656, 23, 24} {
		frame := readDispatchServerFrame(t, peer)
		if binary.LittleEndian.Uint16(frame[1:3]) != id {
			t.Fatalf("creation sequence: got %x, want %d", frame[:3], id)
		}
	}
	if <-done != dispatchHandled || !created || !w.raidWaiting || c.ownedRaidID == 0 || w.state.Position.Area != 2 {
		t.Fatal("default create did not enter the owned room")
	}
	if team, ok := runtime.raidTeams.get(w.role.ID, 82); !ok || team.Recruitment.MemberCount != 1 {
		t.Fatal("real leader was not registered")
	}
}

func TestDefaultBakalDoesNotEnableUnfinishedRaid(t *testing.T) {
	t.Setenv("DFO_RAID_OPEN_EVENTS_PROBE", "")
	c := &gameConnection{worldState: &worldSession{channelType: 112}, gatewayRuntime: &gatewayRuntime{}, bootstrapped: true}
	if c.dispatchRaidEntrance(&clientRequest{frame: wire.Frame{Type: 1, ID: 656}, verified: true}) != dispatchNext {
		t.Fatal("unfinished raid gate changed")
	}
}
