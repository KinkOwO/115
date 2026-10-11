package main

import (
	"bytes"
	"dfolan/internal/database"
	"dfolan/internal/game/wire"
	"encoding/binary"
	"encoding/hex"
	"testing"
)

func townPartyFixture(t *testing.T) (*gameConnection, *townRobotFleet, *recordingConnection, *townRobot) {
	f := robotManagementFixture(t)
	c, out, _ := newDispatchTestClient()
	c.hub = f.hub
	c.hub.townRobots = f
	c.selectedCharacterID = 2
	f.manager.characters.ChannelContext = [2]byte{3, 86}
	p := &lanPeer{roleID: 2, actorID: 3, info: []byte{0, 1, 0, 3, 86}}
	p.send = c.output.send
	b := &townRobot{recordID: 1, peer: &lanPeer{robot: true, roleID: -1, actorID: 65534, info: []byte{0, 1, 0, 3, 86}}}
	f.bots = []*townRobot{b}
	f.hub.add(p)
	f.hub.registerTownRobot(b.peer)
	f.hub.publish(p, 1, 38, 0, 500, 200, false)
	f.hub.publish(b.peer, 1, 38, 0, 600, 200, false)
	c.worldState = &worldSession{hub: f.hub, peer: p, account: 12, characters: f.manager.characters, role: database.Character{ID: 2, AccountID: 12, WireID: 3}}
	c.peer = "127.0.0.1:32000"
	login, e := wire.EncryptPayload(c.keys, 1, []byte{1, 0x11, 0x22, 0x33, 0x44, 0xb5, 0x3e, 0x45, 4})
	if e != nil {
		t.Fatal(e)
	}
	raw, e := wire.ServerFrame(1, 1, login)
	if e != nil {
		t.Fatal(e)
	}
	c.responses = map[uint16][]byte{1: raw}
	t.Cleanup(func() {
		f.mu.Lock()
		defer f.mu.Unlock()
		for owner := range f.parties {
			f.releaseTownPartyLocked(owner, false)
		}
	})
	return c, f, out, b
}

func townInviteRequest() *clientRequest {
	p, _ := hex.DecodeString("feff0093000000fa3e02000000000000")
	return &clientRequest{frame: wire.Frame{Type: 1, ID: 10}, plaintext: p, verified: true}
}

func TestTownRobotPartyInviteIdempotentLeaveAndReinvite(t *testing.T) {
	c, f, out, b := townPartyFixture(t)
	before := bytes.Clone(f.manager.records[1].Role.State)
	for i := 0; i < 2; i++ {
		out.Reset()
		if c.dispatchTownRobotParty(townInviteRequest()) != dispatchHandled {
			t.Fatal("not handled")
		}
		party := f.parties[c.worldState.peer]
		if party == nil || len(party.bots) != 1 || party.bots[0] != b {
			t.Fatal("duplicate or missing member")
		}
		packet := out.Bytes()
		first := int(binary.LittleEndian.Uint32(packet[3:]))
		if packet[0] != 1 || binary.LittleEndian.Uint16(packet[1:]) != 10 || first > len(packet) {
			t.Fatalf("no ACK10 %x", packet)
		}
		if i == 1 {
			if first != len(packet) {
				t.Fatal("duplicate invitation restarted native peer construction")
			}
			continue
		}
		next := packet[first:]
		if next[0] != 0 || binary.LittleEndian.Uint16(next[1:]) != 11 {
			t.Fatal("missing endpoint announcement before roster")
		}
		endpoint, e := wire.DecryptPayload(c.keys, 11, next[wire.ServerHeaderSize:int(binary.LittleEndian.Uint32(next[3:]))])
		if e != nil || endpoint[0] != 1 {
			t.Fatal("invalid endpoints", e)
		}
		next = next[int(binary.LittleEndian.Uint32(next[3:])):]
		if next[0] != 0 || binary.LittleEndian.Uint16(next[1:]) != 9 {
			t.Fatal("missing N9")
		}
		p, e := wire.DecryptPayload(c.keys, 9, next[wire.ServerHeaderSize:int(binary.LittleEndian.Uint32(next[3:]))])
		if e != nil {
			t.Fatal(e)
		}
		if binary.LittleEndian.Uint32(p[65:69]) != 0 || p[69] != 2 || binary.LittleEndian.Uint16(p[71:]) != 3 || binary.LittleEndian.Uint16(p[88:]) != 65534 || !bytes.Equal(p[6:8], []byte{3, 86}) {
			t.Fatalf("wrong roster %x", p)
		}
	}
	q := &clientRequest{frame: wire.Frame{Type: 1, ID: 13}, plaintext: make([]byte, 4), verified: true}
	out.Reset()
	if c.dispatchTownRobotParty(q) != dispatchHandled || len(f.parties) != 0 || out.Len() == 0 {
		t.Fatal("leave failed")
	}
	if c.dispatchTownRobotParty(townInviteRequest()) != dispatchHandled || len(f.parties) != 1 {
		t.Fatal("reinvite failed")
	}
	c.worldState.departArea()
	if len(f.parties) != 0 {
		t.Fatal("scene departure retained reservation")
	}
	if !bytes.Equal(before, f.manager.records[1].Role.State) {
		t.Fatal("party altered robot inventory or player persistence")
	}
}

func TestTownRobotPartyScopeAndReservations(t *testing.T) {
	for _, scope := range []string{"unverified", "foreign", "private", "channel", "departed", "real", "identity", "busy"} {
		t.Run(scope, func(t *testing.T) {
			c, f, out, b := townPartyFixture(t)
			q := townInviteRequest()
			switch scope {
			case "unverified":
				q.verified = false
			case "foreign":
				c.worldState.role.AccountID = 99
			case "private":
				b.peer.private = true
			case "channel":
				b.peer.channel = 9
			case "departed":
				delete(f.hub.peers, b.peer)
			case "real":
				b.peer.robot = false
			case "identity":
				b.peer.info[3] = 2
			case "busy":
				f.parties = map[*lanPeer]*townRobotParty{&lanPeer{}: {id: 1, bots: []*townRobot{b}}}
			}
			// A live second human owns the busy robot and sees its scene.
			if scope == "busy" {
				for owner, party := range f.parties {
					if party != nil {
						owner.actorID = 4
						owner.info = []byte{0, 1, 0, 3, 86}
						f.hub.add(owner)
						f.hub.publish(owner, 1, 38, 0, 600, 200, false)
					}
				}
			}
			c.dispatchTownRobotParty(q)
			if f.parties[c.worldState.peer] != nil {
				t.Fatal("out-of-scope invitation created party")
			}
			if scope != "identity" && scope != "busy" && out.Len() != 0 {
				t.Fatal("out-of-scope reply")
			}
		})
	}
}

func TestTownRobotPartyCapacityAndPopup(t *testing.T) {
	c, f, out, _ := townPartyFixture(t)
	for i := 1; i < 4; i++ {
		b := &townRobot{recordID: i + 1, peer: &lanPeer{robot: true, roleID: -int64(i + 1), actorID: uint16(65534 - i), info: []byte{0, 1, 0, 3, 86}}}
		f.bots = append(f.bots, b)
		f.hub.registerTownRobot(b.peer)
		f.hub.publish(b.peer, 1, 38, 0, 600, 200, false)
	}
	for i := 0; i < 3; i++ {
		q := townInviteRequest()
		binary.LittleEndian.PutUint16(q.plaintext, uint16(65534-i))
		c.dispatchTownRobotParty(q)
	}
	party := f.parties[c.worldState.peer]
	if party == nil || len(party.bots) != 3 {
		t.Fatal("party did not fill")
	}
	q := townInviteRequest()
	binary.LittleEndian.PutUint16(q.plaintext, 65531)
	c.dispatchTownRobotParty(q)
	if len(party.bots) != 3 {
		t.Fatal("over capacity")
	}
	out.Reset()
	q = townInviteRequest()
	q.plaintext[2] = 15
	for i := 3; i < len(q.plaintext); i++ {
		q.plaintext[i] = 0
	}
	if c.dispatchTownRobotContext(q) != dispatchHandled {
		t.Fatal("popup not handled")
	}
	payload, e := wire.DecryptPayload(c.keys, 7, out.Bytes()[wire.ServerHeaderSize:])
	if e != nil {
		t.Fatal(e)
	}
	if binary.LittleEndian.Uint16(payload[7:]) != party.id {
		t.Fatal("popup lost target party identity")
	}
	f.hub.depart(c.worldState.peer)
	f.mu.Lock()
	f.pruneTownPartiesLocked()
	f.mu.Unlock()
	if len(f.parties) != 0 {
		t.Fatal("disconnected owner retained robots")
	}
}

func TestTownRobotPartyDungeonBoundaryPreservesSoloFlow(t *testing.T) {
	c, f, out, _ := townPartyFixture(t)
	c.dispatchTownRobotParty(townInviteRequest())
	out.Reset()
	q := &clientRequest{frame: wire.Frame{Type: 1, ID: 15}, plaintext: make([]byte, 8), verified: true}
	if c.dispatchTownRobotParty(q) != dispatchNext || len(f.parties) != 1 || out.Len() != 0 {
		t.Fatal("selection menu dissolved party or captured ordinary selection")
	}
	c.dispatchTownRobotParty(townInviteRequest())
	out.Reset()
	c.worldState.leaveScene()
	if len(f.parties) != 0 || out.Len() != 0 {
		t.Fatal("direct-entry retirement overwrote new native roster")
	}
}
