package main

import (
	"encoding/binary"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func nativeAITestTransport(t *testing.T) *townRobotTransport {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("DFO_LOCAL_ROBOT_AI", "r23-native-input-v1")
	t.Setenv("DFO_ROBOT_AI_OFFER", filepath.Join(dir, "ai-offer.bin"))
	t.Setenv("DFO_ROBOT_AI_ACK", filepath.Join(dir, "ai-ack.bin"))
	tr := &townRobotTransport{ownerActor: 2, authorityPath: filepath.Join(dir, "authority.bin"), completed: true, room: &[2]uint16{1, 2}, observeEpoch: 1,
		peers: []*townRobotUDP{{actor: 65534}}, event: func(map[string]any) {}}
	tr.resetNativeAILocked()
	return tr
}
func writeNativeTestAck(t *testing.T, tr *townRobotTransport, now time.Time, phase uint32) {
	t.Helper()
	b := nativeAIMessage(tr.authorityBody(now), tr.nativeAI.generation, phase)
	binary.LittleEndian.PutUint32(b[76:], 1234)
	if err := os.WriteFile(tr.nativeAI.ackPath, b[:], 0600); err != nil {
		t.Fatal(err)
	}
}
func TestRobotNativeAIHandoverAndExpiryFence(t *testing.T) {
	tr := nativeAITestTransport(t)
	now := time.Now()
	tr.publishNativeAILocked(now, tr.authorityBody(now))
	if tr.nativeAIControlsLocked() {
		t.Fatal("stopped server before native ready")
	}
	writeNativeTestAck(t, tr, now, 1)
	tr.publishNativeAILocked(now, tr.authorityBody(now))
	if tr.nativeAI.phase != 1 {
		t.Fatal("handover before first room placement")
	}
	tr.peers[0].history = [][]byte{{1}}
	tr.publishNativeAILocked(now, tr.authorityBody(now))
	if !tr.nativeAIControlsLocked() || tr.nativeAI.phase != 2 {
		t.Fatal("did not hand over")
	}
	b, err := os.ReadFile(tr.nativeAI.offerPath)
	if err != nil || len(b) != 80 || binary.LittleEndian.Uint32(b[72:]) != 2 {
		t.Fatal("native drive not published", err)
	}
	later := now.Add(3 * time.Second)
	tr.publishNativeAILocked(later, tr.authorityBody(later))
	if tr.nativeAI.phase != 3 || !tr.nativeAIControlsLocked() {
		t.Fatal("expired adapter resumed coordinate loop")
	}
	writeNativeTestAck(t, tr, later, 1)
	tr.publishNativeAILocked(later, tr.authorityBody(later))
	if tr.nativeAI.phase != 3 {
		t.Fatal("stale prepare revived halted room")
	}
}
func TestRobotNativeAIRoomIdentityAndLifecycle(t *testing.T) {
	tr := nativeAITestTransport(t)
	now := time.Now()
	old := tr.nativeAI.generation
	writeNativeTestAck(t, tr, now, 1)
	tr.observeEpoch++
	tr.resetNativeAILocked()
	tr.peers[0].history = [][]byte{{1}}
	if old == tr.nativeAI.generation {
		t.Fatal("room reused generation")
	}
	tr.publishNativeAILocked(now, tr.authorityBody(now))
	if tr.nativeAI.phase != 1 {
		t.Fatal("old room ack accepted")
	}
	writeNativeTestAck(t, tr, now, 1)
	tr.publishNativeAILocked(now, tr.authorityBody(now))
	if tr.nativeAI.phase != 2 {
		t.Fatal("fresh room rejected")
	}
	tr.room = nil
	tr.publishNativeAILocked(now, tr.authorityBody(now))
	if tr.nativeAI.phase != 3 {
		t.Fatal("town did not revoke")
	}
	if tr.closed || len(tr.peers) != 1 {
		t.Fatal("AI revocation changed accepted party lifecycle")
	}
}
func TestRobotNativeAIRejectsWrongOrTruncatedAck(t *testing.T) {
	tr := nativeAITestTransport(t)
	now := time.Now()
	offer := nativeAIMessage(tr.authorityBody(now), tr.nativeAI.generation, 1)
	ack := offer
	binary.LittleEndian.PutUint32(ack[76:], 1234)
	if _, ok := validNativeAIAck(ack[:], offer, now); !ok {
		t.Fatal("fixture rejected")
	}
	for _, off := range []int{0, 8, 12, 24, 28, 32, 34, 38, 52, 56, 64} {
		altered := ack
		altered[off] ^= 1
		if _, ok := validNativeAIAck(altered[:], offer, now); ok {
			t.Fatalf("identity byte %d accepted", off)
		}
	}
	if _, ok := validNativeAIAck(ack[:79], offer, now); ok {
		t.Fatal("truncation accepted")
	}
	if _, ok := validNativeAIAck(ack[:], offer, now.Add(-time.Second)); ok {
		t.Fatal("future ack accepted")
	}
	if _, ok := validNativeAIAck(ack[:], offer, now.Add(3*time.Second)); ok {
		t.Fatal("expired ack accepted")
	}
}
func TestRobotNativeAIRequiresExplicitOptIn(t *testing.T) {
	tr := nativeAITestTransport(t)
	t.Setenv("DFO_LOCAL_ROBOT_AI", "")
	tr.resetNativeAILocked()
	if tr.nativeAIControlsLocked() || tr.nativeAI.offerPath != "" {
		t.Fatal("default server behavior changed")
	}
}

func TestRobotNativeAIWaitsForRoomLoadBeforeSetupTimeout(t *testing.T) {
	tr := nativeAITestTransport(t)
	now := time.Now().Add(time.Minute)
	tr.publishNativeAILocked(now, tr.authorityBody(now))
	if tr.nativeAI.phase != 1 || !tr.nativeAI.started.IsZero() {
		t.Fatal("loading screen consumed AI setup timeout")
	}
	tr.peers[0].history = [][]byte{{1}}
	tr.publishNativeAILocked(now, tr.authorityBody(now))
	later := now.Add(11 * time.Second)
	tr.publishNativeAILocked(later, tr.authorityBody(later))
	if tr.nativeAI.phase != 3 {
		t.Fatal("unresponsive adapter never halted after load")
	}
}

func TestRobotNativeAIStopsBothMovementTransportsAfterHandover(t *testing.T) {
	tr := nativeAITestTransport(t)
	listen := func() *net.UDPConn {
		c, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1)})
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { c.Close() })
		return c
	}
	peer := tr.peers[0]
	peer.conn = listen()
	client := listen()
	peer.remote = client.LocalAddr().(*net.UDPAddr)
	peer.key = 7
	body := append([]byte{0, 4, 0, 1, 5, 0, 2, 0, 0, 0, 1, 0, 2, 0, 0x30, 0, 0, 0}, make([]byte, 48)...)
	tr.followMovement(peer, 1, peer.remote, 0, body)
	client.SetReadDeadline(time.Now().Add(time.Second))
	if _, err := client.Read(make([]byte, 512)); err != nil {
		t.Fatal("first room placement missing", err)
	}
	now := time.Now()
	writeNativeTestAck(t, tr, now, 1)
	tr.publishNativeAILocked(now, tr.authorityBody(now))
	if tr.nativeAI.phase != 2 {
		t.Fatal("fixture did not hand over")
	}
	for _, phase := range []uint32{2, 3} {
		tr.nativeAI.phase = phase
		tr.followMovementFrom(peer, 1, peer.remote, phase, body, false)
		tr.followMovementFrom(peer, 1, peer.remote, 0, body, true)
		client.SetReadDeadline(time.Now().Add(30 * time.Millisecond))
		if _, err := client.Read(make([]byte, 512)); err == nil {
			t.Fatal("server moved native-controlled actor", phase)
		}
		if len(peer.history) != 1 {
			t.Fatal("native phase accumulated server coordinate history")
		}
	}
}
