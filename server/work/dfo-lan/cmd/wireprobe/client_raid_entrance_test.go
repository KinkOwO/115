package main

import (
	"dfolan/internal/catalog"
	"dfolan/internal/channelrefresh"
	"dfolan/internal/database"
	"dfolan/internal/game/protocol"
	"dfolan/internal/game/wire"
	"encoding/binary"
	"net"
	"testing"
	"time"
)

func TestRaidEntryCostQueryCompletesAfterRepeatedWindowReopen(t *testing.T) {
	t.Setenv("DFO_RAID_OPEN_EVENTS_PROBE", "1")
	server, peer := net.Pipe()
	defer server.Close()
	defer peer.Close()
	peer.SetDeadline(time.Now().Add(5 * time.Second))
	keys := make([]byte, wire.SessionKeyBytes)
	log := func(map[string]any) {}
	c := &gameConnection{gatewayRuntime: &gatewayRuntime{}, worldState: &worldSession{channelType: 82}, bootstrapped: true, event: log, output: newConnectionOutput(server, keys, "test", log)}
	for i := 0; i < BodySampleLimit+2; i++ {
		if !observedGameRequest(650) {
			t.Fatal("reopen query would stop being decoded after sampling cap")
		}
		result := make(chan dispatchAction, 1)
		p := make([]byte, 8)
		p[0] = byte(i % 2)
		go func() {
			result <- c.dispatchRaidEntrance(&clientRequest{frame: wire.Frame{Type: 1, ID: 650}, plaintext: p, verified: true})
		}()
		for j, id := range []uint16{585, 650} {
			frame := readDispatchServerFrame(t, peer)
			if frame[0] != byte(j) || binary.LittleEndian.Uint16(frame[1:3]) != id {
				t.Fatalf("query completion order: %x", frame[:3])
			}
			body, err := wire.DecryptPayload(keys, id, frame[wire.ServerHeaderSize:])
			if err != nil {
				t.Fatal(err)
			}
			if (j == 0 && (len(body) < 4 || binary.LittleEndian.Uint32(body) != 0)) || (j == 1 && (len(body) == 0 || body[0] != 1)) {
				t.Fatalf("invalid query completion %d: %x", id, body)
			}
		}
		if <-result != dispatchHandled {
			t.Fatal("entry cost query not handled")
		}
	}
}

func TestRaidCreateBrokenConnectionRollsBackTeamAndPosition(t *testing.T) {
	t.Setenv("DFO_RAID_OPEN_EVENTS_PROBE", "1")
	a, b := net.Pipe()
	b.Close()
	defer a.Close()
	log := func(map[string]any) {}
	old := database.WorldPosition{Town: 152, Area: 1, X: 50, Y: 60}
	runtime := &gatewayRuntime{raidEntrances: map[uint32]catalog.RaidEntrance{82: {MemberMax: 12, StartMinimum: 1, PartyMax: 3, Waiting: catalog.TownArea{TownID: 152, AreaID: 2, Walkable: [][4]int32{{10, 20, 300, 400}}}}}}
	w := &worldSession{role: database.Character{ID: 1, WireID: 9, Profession: 16, Name: "角色", State: []byte(`{"level":115,"advancement":3,"awakening":3}`)}, level: 115, channelType: 82, state: database.WorldState{Position: old}}
	c := &gameConnection{gatewayRuntime: runtime, worldState: w, channel: 82, channelCfg: channelrefresh.Config{ServerID: 1}, bootstrapped: true, event: log, output: newConnectionOutput(a, make([]byte, wire.SessionKeyBytes), "test", log)}
	p := []byte{8}
	title := []byte("队伍")
	p = binary.LittleEndian.AppendUint32(p, uint32(len(title)))
	p = append(p, title...)
	p = append(p, 0, 0, 0, 0, 0, 0)
	p = binary.LittleEndian.AppendUint32(p, 0)
	q := &clientRequest{frame: wire.Frame{Type: 1, ID: 656}, plaintext: p, verified: true}
	if got := c.dispatchRaidEntrance(q); got != dispatchClose {
		t.Fatalf("broken delivery action %v", got)
	}
	if _, ok := runtime.raidTeams.get(1, 82); ok {
		t.Fatal("team survived broken create delivery")
	}
	if w.state.Position != old || w.raidWaiting {
		t.Fatal("broken delivery changed scene")
	}
	if runtime.raidTeams.next != 1 {
		t.Fatal("create did not reach team allocation before broken delivery")
	}
}

func TestRaidCandidateNeverCapturesLegionOrNonCommand(t *testing.T) {
	t.Setenv("DFO_RAID_OPEN_EVENTS_PROBE", "1")
	c := new(gameConnection)
	for _, f := range []wire.Frame{{Type: 0, ID: 656}, {Type: 1, ID: 2043}} {
		if c.dispatchRaidEntrance(&clientRequest{frame: f}) != dispatchNext {
			t.Fatal("unrelated request captured")
		}
	}
}

func TestRaidOwnedInfoCompletesCharacterInitialization(t *testing.T) {
	t.Setenv("DFO_RAID_OPEN_EVENTS_PROBE", "1")
	server, peer := net.Pipe()
	defer server.Close()
	defer peer.Close()
	peer.SetDeadline(time.Now().Add(5 * time.Second))
	keys := make([]byte, wire.SessionKeyBytes)
	log := func(map[string]any) {}
	runtime := &gatewayRuntime{}
	team, err := runtime.raidTeams.create(1, 82, 1, protocol.RaidRecruitment{Create: protocol.RaidCreateRequest{Kind: 8, Title: "test"}, Leader: 9, LeaderName: "owner", MemberCount: 1, LeaderLevel: 115}, database.WorldPosition{})
	if err != nil {
		t.Fatal(err)
	}
	c := &gameConnection{gatewayRuntime: runtime, worldState: &worldSession{role: database.Character{ID: 1, WireID: 9}, channelType: 82}, channel: 82, bootstrapped: true, event: log, output: newConnectionOutput(server, keys, "test", log)}
	for _, mode := range []byte{1, 2, 1} {
		p := binary.LittleEndian.AppendUint32([]byte{mode}, team.Recruitment.ID)
		result := make(chan dispatchAction, 1)
		go func() {
			result <- c.dispatchRaidEntrance(&clientRequest{frame: wire.Frame{Type: 1, ID: 1353}, plaintext: p, verified: true})
		}()
		for i, id := range []uint16{1353, 1220} {
			frame := readDispatchServerFrame(t, peer)
			if frame[0] != byte(1-i) || binary.LittleEndian.Uint16(frame[1:3]) != id {
				t.Fatalf("completion order: %x", frame[:3])
			}
			body, err := wire.DecryptPayload(keys, id, frame[wire.ServerHeaderSize:])
			if err != nil {
				t.Fatal(err)
			}
			if len(body) == 0 || (i == 0 && body[0] != 1) || (i == 1 && body[0] != 0) {
				t.Fatalf("invalid completion %d: %x", id, body)
			}
		}
		if <-result != dispatchHandled {
			t.Fatal("owned details not handled")
		}
	}
}

func TestRaidLeaveCapturedContextAndRecreate(t *testing.T) {
	t.Setenv("DFO_RAID_OPEN_EVENTS_PROBE", "1")
	server, peer := net.Pipe()
	defer server.Close()
	defer peer.Close()
	peer.SetDeadline(time.Now().Add(5 * time.Second))
	keys := make([]byte, wire.SessionKeyBytes)
	log := func(map[string]any) {}
	runtime := &gatewayRuntime{}
	origin := database.WorldPosition{Town: 152, Area: 1, X: 730, Y: 223}
	recruitment := protocol.RaidRecruitment{Create: protocol.RaidCreateRequest{Kind: 8, Title: "test"}, Leader: 9, MemberCount: 1}
	team, err := runtime.raidTeams.create(1, 82, 1, recruitment, origin)
	if err != nil {
		t.Fatal(err)
	}
	foreign, err := runtime.raidTeams.create(2, 82, 1, recruitment, origin)
	if err != nil {
		t.Fatal(err)
	}
	w := &worldSession{role: database.Character{ID: 1, WireID: 9}, channelType: 82, raidWaiting: true, state: database.WorldState{Position: database.WorldPosition{Town: 152, Area: 2}}}
	c := &gameConnection{gatewayRuntime: runtime, worldState: w, channel: 82, ownedRaidID: team.Recruitment.ID, raidOwnerRole: 1, bootstrapped: true, event: log, output: newConnectionOutput(server, keys, "test", log)}
	result := make(chan dispatchAction, 1)
	// Real v30 leave body on channel 82: 0200000000000000, not 5200.
	go func() {
		result <- c.dispatchRaidEntrance(&clientRequest{frame: wire.Frame{Type: 1, ID: 657}, verified: true, plaintext: []byte{2, 0, 0, 0, 0, 0, 0, 0}})
	}()
	for _, id := range []uint16{657, 23, 24, 577} {
		frame := readDispatchServerFrame(t, peer)
		if binary.LittleEndian.Uint16(frame[1:3]) != id {
			t.Fatalf("leave sequence: %x", frame[:3])
		}
		if id == 657 {
			body, e := wire.DecryptPayload(keys, id, frame[wire.ServerHeaderSize:])
			if e != nil || len(body) < 2 || body[0] != 1 || body[1] != 0 {
				t.Fatalf("leave rejected: %x %v", body, e)
			}
		}
	}
	if <-result != dispatchHandled || w.raidWaiting || c.ownedRaidID != 0 || w.state.Position != origin {
		t.Fatal("owned exit left stale scene or ownership")
	}
	if _, ok := runtime.raidTeams.get(1, 82); ok {
		t.Fatal("departed owner retained team")
	}
	if got, ok := runtime.raidTeams.get(2, 82); !ok || got.Recruitment.ID != foreign.Recruitment.ID {
		t.Fatal("leave touched another owner's team")
	}
	if _, err = runtime.raidTeams.create(1, 82, 1, recruitment, origin); err != nil {
		t.Fatal("exit prevented recreation", err)
	}
}
