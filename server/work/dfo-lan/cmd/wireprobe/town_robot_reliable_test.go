package main

import (
	"bytes"
	"dfolan/internal/game/protocol"
	"dfolan/internal/game/wire"
	"encoding/binary"
	"encoding/hex"
	"net"
	"testing"
	"time"
)

func TestTownRobotReliableRoomSyncAndRealMovement(t *testing.T) {
	tr := &townRobotTransport{ownerActor: 2, ownerKey: 0x11223344, event: func(map[string]any) {}, send: func(byte, uint16, []byte) error { return nil }}
	defer tr.close()
	if e := tr.add(65534); e != nil {
		t.Fatal(e)
	}
	peer := tr.peers[0]
	c, e := net.DialUDP("udp4", nil, peer.conn.LocalAddr().(*net.UDPAddr))
	if e != nil {
		t.Fatal(e)
	}
	defer c.Close()
	read := func() wire.NativeUDPFrame115 {
		t.Helper()
		c.SetReadDeadline(time.Now().Add(time.Second))
		buf := make([]byte, 4096)
		n, e := c.Read(buf)
		if e != nil {
			t.Fatal(e)
		}
		f, e := wire.DecodeNativeUDPDatagram115(buf[:n], peer.key)
		if e != nil || len(f) != 1 {
			t.Fatal("invalid reply", e)
		}
		return f[0]
	}
	send := func(seq uint32, messages ...[]byte) {
		t.Helper()
		var bundle []byte
		for _, m := range messages {
			bundle = binary.LittleEndian.AppendUint16(bundle, uint16(len(m)))
			bundle = append(bundle, m...)
		}
		raw, e := wire.EncodeNativeUDPReliable115(0, tr.ownerKey, seq, bundle)
		if e != nil {
			t.Fatal(e)
		}
		if _, e = c.Write(raw); e != nil {
			t.Fatal(e)
		}
	}
	body, _ := protocol.PartyCheckConnectUDP115(0, 4, 1, 2)
	raw, _ := wire.EncodeNativeUDP115(0, tr.ownerKey, 0, body)
	c.Write(raw)
	if read().Type != 2 {
		t.Fatal("handshake changed")
	}
	tr.check(protocol.PartyConnectionCheck115{Mode: 1, Slot: 1, Mask: 2})
	tr.setRoom(nil)
	entry, _ := hex.DecodeString("014d00000602c600")
	loading, _ := hex.DecodeString("01c000000003000000e621f605")
	send(0, entry, loading)
	ack := read()
	if ack.Type != 0 || binary.LittleEndian.Uint32(ack.Payload) != 1 {
		t.Fatal("missing cumulative ACK")
	}
	c.SetReadDeadline(time.Now().Add(40 * time.Millisecond))
	if _, e = c.Read(make([]byte, 512)); e == nil {
		t.Fatal("published actor sync before native room")
	}
	room := [2]uint16{0, 0}
	tr.setRoom(&room)
	for i, original := range [][]byte{entry, loading} {
		f := read()
		if f.Type != 1 || f.Sequence != uint32(i) {
			t.Fatal("reliable output sequence")
		}
		rows, e := wire.SplitNativeUDPReliable115(f.Payload)
		want, _ := protocol.RobotRoomSync115(original, 1)
		if e != nil || len(rows) != 1 || !bytes.Equal(rows[0], want) {
			t.Fatal("bot identity/position/loading sync", e)
		}
	}
	ackRaw, _ := wire.EncodeNativeUDPAck115(0, tr.ownerKey, 2)
	c.Write(ackRaw)
	deadline := time.Now().Add(time.Second)
	for {
		tr.mu.Lock()
		n := len(peer.pendingReliable)
		tr.mu.Unlock()
		if n == 0 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("ACK did not retire retries")
		}
		time.Sleep(time.Millisecond)
	}
	movement, _ := hex.DecodeString("0104000105000200000000000000305555410080014400004643")
	send(1, movement)
	if f := read(); f.Type != 0 || binary.LittleEndian.Uint32(f.Payload) != 2 {
		t.Fatal("movement bundle not acknowledged")
	}
	f := read()
	want, _ := protocol.RobotFollowMove115(movement, 2, 65534, room)
	if f.Type != 2 || !bytes.Equal(f.Payload, want) {
		t.Fatalf("real player movement did not select bot %x", f.Payload)
	}
	// Out-of-order reliable messages are held until the gap closes.
	send(3, []byte{2, 195, 0, 0, 0})
	if f := read(); f.Type != 0 || binary.LittleEndian.Uint32(f.Payload) != 2 {
		t.Fatal("acknowledged missing sequence")
	}
	send(2, []byte{1, 1, 0})
	if f := read(); f.Type != 0 || binary.LittleEndian.Uint32(f.Payload) != 4 {
		t.Fatal("gap did not close")
	}
	if f := read(); f.Type != 1 || f.Sequence != 2 {
		t.Fatal("held room state not forwarded once")
	}
	ackRaw, _ = wire.EncodeNativeUDPAck115(0, tr.ownerKey, 3)
	c.Write(ackRaw)
	send(4, []byte{1, 5, 0})
	if f := read(); f.Type != 0 {
		t.Fatal("unrelated state not acknowledged")
	}
	c.SetReadDeadline(time.Now().Add(40 * time.Millisecond))
	if _, e = c.Read(make([]byte, 512)); e == nil {
		t.Fatal("combat state replayed")
	}
}
