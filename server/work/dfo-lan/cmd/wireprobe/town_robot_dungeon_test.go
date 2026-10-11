package main

import (
	"bytes"
	"context"
	"dfolan/internal/catalog"
	"dfolan/internal/dungeon"
	"dfolan/internal/game/protocol"
	"dfolan/internal/game/wire"
	"encoding/binary"
	"net"
	"testing"
	"time"
)

func TestTownRobotAdmittedPlanPreservesPartyAcrossNextDungeon(t *testing.T) {
	t.Setenv("DFO_ELITE_COMPANION_PILOT", "attunement")
	c, f, _, _ := townPartyFixture(t)
	c.dispatchTownRobotParty(townInviteRequest())
	w := c.worldState
	// channel.local35.json: channel 10 (ordinary) is type 22, not type 0.
	w.channelType = 22
	w.characters = nil
	w.soloPartyBootstrap = true
	p := f.parties[w.peer]
	s := &dungeon.Session{Room: catalog.DungeonRoom{X: 2, Y: 3, Map: 76121}, Maze: catalog.DungeonMaze{Start: [2]byte{2, 3}}}
	s.Definition = catalog.DungeonDefinition{ID: 1, Type: "boundary of attunement", Script: catalog.ScriptRecord{Path: "contents/2026/skyofathousandseasofborder/dungeon/test.dgn"}}
	for step := 0; step < 2; step++ {
		if !w.ownsTownRobotParty(p.id) || w.ownsTownRobotParty(p.id+1) {
			t.Fatal("party admission must match this connection")
		}
		plan, err := w.dungeonEntryPlan(context.Background(), "entry", 16, protocol.DungeonSelection{ID: 1}, s)
		if err != nil || !p.entryPrepared {
			t.Fatal("robot entry not prepared", err)
		}
		appearance, info := -1, -1
		for i, packet := range plan {
			if packet.ID == 9 {
				t.Fatal("robot roster replaced by solo party")
			}
			if packet.Name == "robot_dungeon_appearance" {
				appearance = i
			}
			if packet.ID == 28 {
				info = i
			}
		}
		if appearance < 0 || info <= appearance {
			t.Fatal("robot appearance must precede dungeon info")
		}
		w.activeDungeon = s
		w.leaveScene()
		if f.parties[w.peer] != p || !p.dungeon || p.entryPrepared || p.transport.closed {
			t.Fatal("admitted entry released native peers")
		}
		loaded, err := w.finishDungeonLoading(make([]byte, 16))
		if err != nil {
			t.Fatal(err)
		}
		complete, own, peer, room := -1, -1, -1, -1
		for i, packet := range loaded {
			if packet.Kind == 0 && packet.ID == 24 {
				t.Fatal("loading redirected dungeon actors to town")
			}
			switch packet.Name {
			case "dungeon_actor_state":
				own = i
			case "dungeon_loading_complete":
				complete = i
			case "robot_dungeon_actor_state":
				peer = i
			case "robot_dungeon_actor_room":
				room = i
			}
		}
		if own < 0 || complete <= own || peer <= complete || room <= peer {
			t.Fatalf("missing ordered player/robot loading plan: own=%d complete=%d peer=%d room=%d", own, complete, peer, room)
		}
	}
	// A later incompatible loader must not inherit a successful preparation.
	for _, channelType := range []uint32{68, 73, 74, 76, 78} {
		w.channelType = channelType
		if w.ownsTownRobotParty(p.id) {
			t.Fatal("specialized companion channel captured", channelType)
		}
	}
	w.channelType = 22
	p.entryPrepared = true
	w.channelWorldIsolated = true
	t.Setenv("DFO_ELITE_COMPANION_PILOT", "")
	if w.ownsTownRobotParty(p.id) {
		t.Fatal("special channel admission captured")
	}
	_, err := w.dungeonEntryPlanImpl(context.Background(), "special", 16, protocol.DungeonSelection{ID: 1}, s, &[2]byte{1, 2})
	if err != nil || p.entryPrepared {
		t.Fatal("stale preparation survived another loader", err)
	}
}

func TestTownRobotVitalsFollowRosterAndPreservePlayer(t *testing.T) {
	c, _, out, _ := townPartyFixture(t)
	c.dispatchTownRobotParty(townInviteRequest())
	p := out.Bytes()
	var ids []uint16
	var body []byte
	for len(p) > 0 {
		n := int(binary.LittleEndian.Uint32(p[3:]))
		id := binary.LittleEndian.Uint16(p[1:])
		ids = append(ids, id)
		if id == 153 {
			var e error
			body, e = wire.DecryptPayload(c.keys, id, p[wire.ServerHeaderSize:n])
			if e != nil {
				t.Fatal(e)
			}
		}
		p = p[n:]
	}
	if !bytes.Equal(body, []byte{1, 254, 255, 100, 0, 0, 0, 0}) || len(ids) != 4 || ids[2] != 9 || ids[3] != 153 {
		t.Fatal("town vitals missing after roster", ids, body)
	}
}
func TestTownRobotDungeonReservationRoomAndReturn(t *testing.T) {
	c, f, out, b := townPartyFixture(t)
	w := c.worldState
	c.dispatchTownRobotParty(townInviteRequest())
	p := f.parties[w.peer]
	identity := p.id
	p.entryPrepared = true
	out.Reset()
	w.leaveScene()
	if !p.dungeon || p.transport.closed || b.peer.placed || w.peer.placed {
		t.Fatal("entry lost reservation or transport")
	}
	f.mu.Lock()
	f.prune()
	f.mu.Unlock()
	if f.parties[w.peer] != p || len(f.bots) != 1 {
		t.Fatal("dungeon party pruned as empty town")
	}
	w.activeDungeon = &dungeon.Session{Room: catalog.DungeonRoom{X: 2, Y: 3, Map: 76121}}
	plan, e := w.robotDungeonRoomPlan()
	if e != nil || len(plan) != 2 || plan[1].ID != 29 || plan[1].Payload[33] != 1 || plan[1].Payload[31] != 0 {
		t.Fatal("missing cached room actor attach", e, plan)
	}
	if !w.endRobotDungeonParty() || len(f.parties) != 0 || !p.transport.closed || !b.peer.placed || w.soloPartyReady {
		t.Fatal("return retained invisible party")
	}
	w.activeDungeon = nil
	f.hub.publish(w.peer, 1, 38, 0, 500, 200, false)
	out.Reset()
	c.dispatchTownRobotParty(townInviteRequest())
	if f.parties[w.peer] == nil || f.parties[w.peer].id == identity {
		t.Fatal("return cannot invite fresh party")
	}
}

func TestTownRobotEntryDoesNotDeleteCompanionOnOwnersClient(t *testing.T) {
	c, f, out, b := townPartyFixture(t)
	w := c.worldState
	c.dispatchTownRobotParty(townInviteRequest())
	f.parties[w.peer].entryPrepared = true
	var bystanderDeletes []uint16
	bystander := &lanPeer{actorID: 8, info: []byte{0, 1, 0, 3, 86}, send: func(kind byte, id uint16, payload []byte) error {
		if kind == 0 && id == 6 {
			bystanderDeletes = append(bystanderDeletes, binary.LittleEndian.Uint16(payload))
		}
		return nil
	}}
	f.hub.add(bystander)
	f.hub.publish(bystander, 1, 38, 0, 700, 200, false)
	out.Reset()
	w.leaveScene()
	for data := out.Bytes(); len(data) > 0; {
		if len(data) < wire.ServerHeaderSize {
			t.Fatal("truncated output")
		}
		n := int(binary.LittleEndian.Uint32(data[3:]))
		id := binary.LittleEndian.Uint16(data[1:])
		if n > len(data) || n < wire.ServerHeaderSize {
			t.Fatal("invalid frame size")
		}
		if data[0] == 0 && id == 6 {
			t.Fatal("entering player received N6 deletion for its cached companion")
		}
		data = data[n:]
	}
	if w.peer.placed || b.peer.placed || len(bystanderDeletes) != 2 {
		t.Fatal("town observers did not retire both entering actors", bystanderDeletes)
	}
	seen := map[uint16]bool{}
	for _, actor := range bystanderDeletes {
		seen[actor] = true
	}
	if !seen[w.peer.actorID] || !seen[b.peer.actorID] {
		t.Fatal("wrong town departures", bystanderDeletes)
	}
}
func TestTownRobotFollowRequiresConnectedEndpointAndRoom(t *testing.T) {
	tr := &townRobotTransport{ownerActor: 3, ownerKey: 7, event: func(map[string]any) {}, send: func(byte, uint16, []byte) error { return nil }}
	defer tr.close()
	if e := tr.add(65534); e != nil {
		t.Fatal(e)
	}
	socket, e := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1)})
	if e != nil {
		t.Fatal(e)
	}
	defer socket.Close()
	remote := socket.LocalAddr().(*net.UDPAddr)
	peer := tr.peers[0]
	p := append([]byte{0, 4, 0, 1, 5, 0, 3, 0, 0, 0, 2, 0, 3, 0, 0, 0, 0, 0}, make([]byte, 48)...)
	buf := make([]byte, 512)
	for stage := 0; stage < 4; stage++ {
		if stage == 1 {
			tr.mu.Lock()
			peer.remote = remote
			tr.completed = true
			tr.mu.Unlock()
		}
		if stage == 2 {
			tr.setRoom(&[2]uint16{2, 3})
		}
		if stage == 3 {
			tr.setRoom(nil)
		}
		tr.followMovement(peer, 1, remote, uint32(stage), p)
		socket.SetReadDeadline(time.Now().Add(35 * time.Millisecond))
		n, e := socket.Read(buf)
		if stage != 2 {
			if e == nil {
				t.Fatal("movement escaped lifecycle gate", stage)
			}
			continue
		}
		if e != nil {
			t.Fatal(e)
		}
		slot, _, body, e := wire.DecodeNativeUDP115(buf[:n], peer.key)
		if e != nil || slot != 1 || binary.LittleEndian.Uint16(body[6:]) != 65534 {
			t.Fatal("invalid native follow datagram", e)
		}
	}
}

func TestTownRobotFollowInterpolatesAfterPlacementAndResetsByRoom(t *testing.T) {
	tr := &townRobotTransport{ownerActor: 3, ownerKey: 7, event: func(map[string]any) {}, send: func(byte, uint16, []byte) error { return nil }}
	defer tr.close()
	if e := tr.add(65534); e != nil {
		t.Fatal(e)
	}
	socket, e := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1)})
	if e != nil {
		t.Fatal(e)
	}
	defer socket.Close()
	remote := socket.LocalAddr().(*net.UDPAddr)
	peer := tr.peers[0]
	tr.mu.Lock()
	peer.remote = remote
	tr.completed = true
	tr.mu.Unlock()
	tr.setRoom(&[2]uint16{2, 3})
	p := append([]byte{0, 4, 0, 1, 5, 0, 3, 0, 0, 0, 2, 0, 3, 0, 0x30, 0, 0, 0}, make([]byte, 48)...)
	for i := 0; i < 4; i++ {
		if i == 3 {
			tr.setRoom(&[2]uint16{3, 3})
			binary.LittleEndian.PutUint16(p[10:], 3)
		}
		tr.followMovement(peer, 1, remote, uint32(i), p)
		socket.SetReadDeadline(time.Now().Add(time.Second))
		buf := make([]byte, 512)
		n, e := socket.Read(buf)
		if e != nil {
			t.Fatal(e)
		}
		_, _, body, e := wire.DecodeNativeUDP115(buf[:n], peer.key)
		if e != nil {
			t.Fatal(e)
		}
		want := byte(0x20)
		if i == 0 || i == 3 {
			want = 0x30
		}
		if body[14] != want || binary.LittleEndian.Uint16(body[6:]) != 65534 {
			t.Fatalf("movement mode/identity: step%d body%x", i, body)
		}
		if p[14] != 0x30 {
			t.Fatal("owner packet mutated")
		}
	}
}
