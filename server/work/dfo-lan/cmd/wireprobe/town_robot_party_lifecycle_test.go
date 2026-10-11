package main

import (
	"bytes"
	"dfolan/internal/game/protocol"
	"dfolan/internal/game/wire"
	"encoding/binary"
	"testing"
)

func lifecyclePackets(t *testing.T, c *gameConnection, out *recordingConnection) []outboundPacket {
	t.Helper()
	var packets []outboundPacket
	for raw := out.Bytes(); len(raw) > 0; {
		if len(raw) < wire.ServerHeaderSize {
			t.Fatal("short frame")
		}
		size := int(binary.LittleEndian.Uint32(raw[3:]))
		id := binary.LittleEndian.Uint16(raw[1:])
		if size < wire.ServerHeaderSize || size > len(raw) {
			t.Fatal("invalid frame")
		}
		p, e := wire.DecryptPayload(c.keys, id, raw[wire.ServerHeaderSize:size])
		if e != nil {
			t.Fatal(e)
		}
		packets = append(packets, outboundPacket{"", raw[0], id, p})
		raw = raw[size:]
	}
	return packets
}

func TestRobotLifecyclePaddedLeaveClearsRosterBeforeReinvite(t *testing.T) {
	c, f, out, _ := townPartyFixture(t)
	c.dispatchTownRobotParty(townInviteRequest())
	p := f.parties[c.worldState.peer]
	out.Reset()
	q := &clientRequest{frame: wire.Frame{Type: 1, ID: 13}, plaintext: make([]byte, 8), verified: true}
	if c.dispatchTownRobotParty(q) != dispatchHandled || f.parties[c.worldState.peer] != nil || !p.transport.closed {
		t.Fatal("live padded leave not handled")
	}
	packets := lifecyclePackets(t, c, out)
	gone, _ := protocol.OrdinaryPartyGone115(p.id, p.identity)
	if len(packets) != 3 || packets[0].ID != 13 || packets[1].ID != 10 || packets[2].ID != 9 || !bytes.Equal(packets[2].Payload[:12], gone) {
		t.Fatalf("incomplete native cleanup: %+v", packets)
	}
	out.Reset()
	c.dispatchTownRobotParty(townInviteRequest())
	fresh := f.parties[c.worldState.peer]
	if fresh == nil || fresh.id == p.id || fresh.transport == p.transport || fresh.transport.closed || fresh.transport.completed {
		t.Fatal("reinvite reused stale transport")
	}
}

func TestRobotLifecycleKickRebuildsOnlySurvivors(t *testing.T) {
	c, f, out, _ := townPartyFixture(t)
	for i := 1; i < 3; i++ {
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
	old := f.parties[c.worldState.peer].transport
	q := &clientRequest{frame: wire.Frame{Type: 1, ID: 14}, plaintext: make([]byte, 16), verified: true}
	q.plaintext[0] = 2
	out.Reset()
	c.dispatchTownRobotParty(q)
	p := f.parties[c.worldState.peer]
	if p == nil || len(p.bots) != 2 || p.bots[0].peer.actorID != 65534 || p.bots[1].peer.actorID != 65532 || !old.closed || p.transport == old {
		t.Fatal("kick lost surviving members or retained old transport")
	}
	packets := lifecyclePackets(t, c, out)
	want := []uint16{14, 10, 9, 11, 9, 153}
	if len(packets) != len(want) {
		t.Fatal(packets)
	}
	for i, id := range want {
		if packets[i].ID != id {
			t.Fatal("cleanup/announce order", packets)
		}
	}
	if packets[2].Payload[9] != 3 || packets[4].Payload[69] != 3 {
		t.Fatal("roster not removed/rebuilt")
	}
	q.plaintext[0] = 1
	c.dispatchTownRobotParty(q)
	c.dispatchTownRobotParty(q)
	if f.parties[c.worldState.peer] != nil {
		t.Fatal("last bot kick left a ghost party")
	}
}

func TestRobotLifecycleSoloBootstrapOwnershipAndMalformedRequests(t *testing.T) {
	c, f, out, _ := townPartyFixture(t)
	w := c.worldState
	solo, _ := protocol.SoloPartyInfo(3, [2]byte{3, 86})
	w.soloPartyReady = true
	w.rememberOrdinarySoloParty(solo)
	if w.ordinarySoloParty == nil {
		t.Fatal("ordinary bootstrap untracked")
	}
	q := &clientRequest{frame: wire.Frame{Type: 1, ID: 13}, plaintext: make([]byte, 8), verified: true}
	q.plaintext[7] = 1
	out.Reset()
	c.dispatchTownRobotParty(q)
	if out.Len() != 0 || w.ordinarySoloParty == nil {
		t.Fatal("malformed request mutated party")
	}
	q.plaintext[7] = 0
	c.dispatchTownRobotParty(q)
	if w.ordinarySoloParty != nil || w.soloPartyReady || len(f.parties) != 0 {
		t.Fatal("solo bootstrap not cleared")
	}
	if packets := lifecyclePackets(t, c, out); len(packets) != 3 || packets[2].ID != 9 {
		t.Fatal("missing solo HUD cleanup")
	}
	out.Reset()
	w.soloPartyReady = true
	if c.dispatchTownRobotParty(q) != dispatchNext || out.Len() != 0 || !w.soloPartyReady {
		t.Fatal("unowned specialized party captured")
	}
}
