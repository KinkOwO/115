package main

import (
	"dfolan/internal/catalog"
	"dfolan/internal/database"
	"dfolan/internal/game/protocol"
	"dfolan/internal/game/wire"
	"encoding/binary"
	"net"
	"testing"
	"time"
)

func TestRaidVoteRejectsUnsolicitedChoicesAndForeignLeader(t *testing.T) {
	t.Setenv("DFO_RAID_OPEN_EVENTS_PROBE", "1")
	server, peer := net.Pipe()
	defer server.Close()
	defer peer.Close()
	peer.SetDeadline(time.Now().Add(5 * time.Second))
	runtime := &gatewayRuntime{raidEntrances: map[uint32]catalog.RaidEntrance{82: {Bakal: &catalog.BakalRaidRules{}}}}
	team, err := runtime.raidTeams.create(1, 82, 1, protocol.RaidRecruitment{Create: protocol.RaidCreateRequest{Kind: 8}, Leader: 9, MemberCount: 1}, database.WorldPosition{})
	if err != nil {
		t.Fatal(err)
	}
	w := &worldSession{role: database.Character{ID: 1, WireID: 9}, channelType: 82, raidWaiting: true}
	noop := func(map[string]any) {}
	keys := make([]byte, wire.SessionKeyBytes)
	c := &gameConnection{gatewayRuntime: runtime, worldState: w, channel: 82, ownedRaidID: team.Recruitment.ID, bootstrapped: true, event: noop, output: newConnectionOutput(server, keys, "test", noop)}
	request := func(choice byte, ok bool, ids ...uint16) {
		t.Helper()
		p := make([]byte, 32)
		p[13] = choice
		binary.LittleEndian.PutUint32(p[14:], 1)
		done := make(chan dispatchAction, 1)
		go func() {
			done <- c.dispatchRaidEntrance(&clientRequest{frame: wire.Frame{Type: 1, ID: 2089}, verified: true, plaintext: p})
		}()
		for _, id := range ids {
			f := readDispatchServerFrame(t, peer)
			if binary.LittleEndian.Uint16(f[1:3]) != id {
				t.Fatal("vote wire ID")
			}
			if f[0] == 1 {
				b, err := wire.DecryptPayload(keys, id, f[wire.ServerHeaderSize:])
				if err != nil {
					t.Fatal(err)
				}
				if (b[0] == 1) != ok {
					t.Fatal("unsafe vote acknowledgement", b)
				}
			}
		}
		if <-done != dispatchHandled {
			t.Fatal("vote dispatch closed")
		}
	}
	request(1, false, 2089)
	request(2, false, 2089)
	w.role.WireID = 10
	request(0, false, 2089)
	w.role.WireID = 9
	w.channelType = 83
	request(0, false, 2089)
	if w.bakalOpening != nil {
		t.Fatal("unsolicited or foreign vote started raid")
	}
	owned, _ := runtime.raidTeams.get(1, 82)
	if owned.Recruitment.State != 0 || owned.Recruitment.MemberCount != 1 {
		t.Fatal("vote changed real waiting roster")
	}
}
