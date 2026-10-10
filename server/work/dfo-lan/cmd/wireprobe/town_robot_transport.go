package main

import (
	"crypto/rand"
	"dfolan/internal/game/protocol"
	"dfolan/internal/game/wire"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"net"
	"os"
	"sync"
	"time"
)

// A native peer endpoint belongs to one town party/session. It never writes
// player storage and is closed before N10, disconnect or dungeon admission.
type townRobotUDP struct {
	conn            *net.UDPConn
	actor           uint16
	key             uint32
	received        byte
	reported        byte
	rejected        uint8
	nextInSequence  uint32
	nextOutSequence uint32
	remote          *net.UDPAddr
	history         [][]byte
	followSamples   uint8
	observeTotal    uint16
	observeEpoch    uint32
	observeCount    uint8
	nextReliableIn  uint32
	nextReliableOut uint32
	reliableIn      map[uint32][][]byte
	pendingReliable map[uint32]*robotReliablePending
	roomSync        map[uint16][]byte
}
type townRobotTransport struct {
	nativeAI       robotNativeAI
	authorityPath  string
	authorityLast  time.Time
	authorityError bool
	mu             sync.Mutex
	ownerActor     uint16
	ownerKey       uint32
	peers          []*townRobotUDP
	closed         bool
	completed      bool
	room           *[2]uint16
	observeEpoch   uint32
	roomLoading    bool
	send           func(byte, uint16, []byte) error
	event          func(map[string]any)
}

func (client *gameConnection) townRobotNativeKey() (uint32, error) {
	// Live native LOGIN global matches payload[5:9]. Success cursor is1,
	// followed by four u8 reads and u32 key; rechecked against actual fixture.
	raw := client.responses[1]
	if len(raw) <= wire.ServerHeaderSize || raw[0] != 1 || binary.LittleEndian.Uint16(raw[1:]) != 1 {
		return 0, fmt.Errorf("native login response unavailable")
	}
	p, e := wire.DecryptPayload(client.keys, 1, raw[wire.ServerHeaderSize:])
	if e != nil || len(p) < 9 || p[0] != 1 {
		return 0, fmt.Errorf("native login peer key unavailable")
	}
	return binary.LittleEndian.Uint32(p[5:9]), nil
}

func (t *townRobotTransport) add(actor uint16) error {
	var material [4]byte
	if _, e := rand.Read(material[:]); e != nil {
		return e
	}
	key := binary.LittleEndian.Uint32(material[:])
	if key == 0 {
		key = 1
	}
	conn, e := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1)})
	if e != nil {
		return e
	}
	t.mu.Lock()
	if t.closed || len(t.peers) >= 3 {
		t.mu.Unlock()
		conn.Close()
		return fmt.Errorf("party transport closed or full")
	}
	p := &townRobotUDP{conn: conn, actor: actor, key: key}
	slot := byte(len(t.peers) + 1)
	t.peers = append(t.peers, p)
	t.completed = false
	t.mu.Unlock()
	go t.receive(p, slot)
	go t.resendReliable(p)
	return nil
}

func (t *townRobotTransport) endpoints() ([]byte, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	var rows []protocol.PartyPeerEndpoint115
	for _, p := range t.peers {
		rows = append(rows, protocol.PartyPeerEndpoint115{Actor: p.actor, IP: [4]byte{127, 0, 0, 1}, Port: uint16(p.conn.LocalAddr().(*net.UDPAddr).Port), Key: p.key, MTU: 512})
	}
	// 512 is the native MTU advertised in the captured local CMD2; this
	// handshake is8 bytes and never enters the reliable fragmentation path.
	return protocol.PartyPeerEndpoints115(rows)
}

func (t *townRobotTransport) close() {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.closed {
		return
	}
	t.closed = true
	t.publishAuthorityLocked(false, true)
	for _, p := range t.peers {
		p.conn.Close()
	}
}

func (t *townRobotTransport) finishLocked() error {
	if t.closed || t.completed || len(t.peers) == 0 {
		return nil
	}
	var masks []byte
	for _, p := range t.peers {
		mask := p.received & p.reported & 3
		if mask == 0 {
			return nil
		}
		masks = append(masks, mask)
	}
	payload, e := protocol.PartyDirectConnectionResult115(masks)
	if e != nil {
		return e
	}
	if e = t.send(0, 2373, payload); e != nil {
		return e
	}
	t.completed = true
	t.flushRoomSyncLocked()
	t.publishAuthorityLocked(false, true)
	t.event(map[string]any{"kind": "town_robot_party_transport_ready", "direct_masks": masks, "bidirectional_udp": true, "relay": false, "attempt": "r7-wirefix1-selected-native-send-evidence"})
	return nil
}

func (t *townRobotTransport) check(r protocol.PartyConnectionCheck115) error {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.closed {
		return nil
	}
	if r.Mode == 0 {
		if r.Slot != 0 || r.Mask != 4 {
			return nil
		}
		t.event(map[string]any{"kind": "town_robot_party_connection_started", "members": len(t.peers), "attempt": "r7-wirefix1-selected-native-send-evidence"})
		return nil
	}
	if r.Slot == 0 || int(r.Slot) > len(t.peers) {
		return nil
	}
	t.peers[r.Slot-1].reported |= byte(r.Mask) & 3
	t.event(map[string]any{"kind": "town_robot_party_connection_report", "slot": r.Slot, "mask": r.Mask, "attempt": "r7-wirefix1-selected-native-send-evidence"})
	return t.finishLocked()
}

// Keep only the first eight rejections per endpoint; never log raw packets or keys.
func (t *townRobotTransport) reject(peer *townRobotUDP, slot byte, reason string, size int) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.closed || peer.rejected >= 8 {
		return
	}
	peer.rejected++
	t.event(map[string]any{"kind": "town_robot_party_udp_rejected", "slot": slot, "stage": reason, "bytes": size, "sample": peer.rejected})
}

func (t *townRobotTransport) receive(peer *townRobotUDP, slot byte) {
	buf := make([]byte, 4096)
	for {
		n, remote, e := peer.conn.ReadFromUDP(buf)
		if e != nil {
			return
		}
		if !remote.IP.IsLoopback() {
			t.reject(peer, slot, "remote", n)
			continue
		}
		t.observeUDP(peer, slot, buf[:n])
		frames, e := wire.DecodeNativeUDPDatagram115(buf[:n], t.ownerKey)
		if e != nil {
			t.reject(peer, slot, "decode", n)
			continue
		}
		for _, f := range frames {
			if f.Type == 2 {
				t.receiveUnreliable(peer, slot, remote, f.Slot, f.Sequence, f.Payload, n)
			} else {
				t.receiveReliable(peer, slot, remote, f)
			}
		}
	}
}
func (t *townRobotTransport) receiveUnreliable(peer *townRobotUDP, slot byte, remote *net.UDPAddr, sender byte, sequence uint32, p []byte, n int) {
	if sender == 0 && len(p) >= 4 && p[0] <= 2 && binary.LittleEndian.Uint16(p[1:]) == 4 {
		t.followMovement(peer, slot, remote, sequence, p)
		return
	}
	// 14665c1c0 emits inner type0, opcode0, slot, kind, address, actor.
	if sender != 0 || len(p) != 8 || p[0] != 0 || binary.LittleEndian.Uint16(p[1:]) != 0 || p[3] != 0 || p[4] > 4 || p[5] != 1 || binary.LittleEndian.Uint16(p[6:]) != t.ownerActor {
		t.reject(peer, slot, "envelope", n)
		return
	}
	kind := p[4]
	responseKind := kind
	switch kind {
	case 0:
		responseKind = 1
	case 4:
	default:
		t.reject(peer, slot, "kind", n)
		return
	}
	body, e := protocol.PartyCheckConnectUDP115(slot, responseKind, 1, peer.actor)
	if e != nil {
		return
	}
	// Kind4 is the native coordination probe. Reflect it from the robot's
	// own endpoint; the client then proves receipt with CMD2116 mode1/mask2.
	t.mu.Lock()
	if t.closed {
		t.mu.Unlock()
		return
	}
	if sequence < peer.nextInSequence {
		t.mu.Unlock()
		return
	}
	peer.nextInSequence = sequence + 1
	peer.remote = remote
	raw, e := wire.EncodeNativeUDP115(slot, peer.key, peer.nextOutSequence, body)
	if e != nil {
		t.mu.Unlock()
		return
	}
	if _, e = peer.conn.WriteToUDP(raw, remote); e == nil {
		peer.nextOutSequence++
		if kind == 4 {
			peer.received |= 2
		}
		t.event(map[string]any{"kind": "town_robot_party_udp_check", "slot": slot, "actor": peer.actor, "check_kind": kind, "response_kind": responseKind, "address_kind": 1, "attempt": "r7-wirefix1-selected-native-send-evidence"})
		if e = t.finishLocked(); e != nil {
			t.event(map[string]any{"kind": "town_robot_party_transport_error", "error": e.Error()})
		}
	}
	t.mu.Unlock()
}

// setRoom is called only after native room notifications have been written.
func (t *townRobotTransport) setRoom(room *[2]uint16) {
	t.setRoomState(room, room == nil)
}

// loading is false when returning to town; late room-sync messages must not
// become pending initial state for a subsequent dungeon.
func (t *townRobotTransport) setRoomState(room *[2]uint16, loading bool) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.observeEpoch++
	t.resetNativeAILocked()
	t.room = nil
	t.roomLoading = loading
	if room != nil {
		copy := *room
		t.room = &copy
	}
	for _, p := range t.peers {
		if room == nil {
			p.roomSync = nil
		}
		p.history = nil
		p.followSamples = 0
	}
	t.flushRoomSyncLocked()
	t.publishAuthorityLocked(room != nil, true)
}
func (t *townRobotTransport) followMovement(peer *townRobotUDP, slot byte, remote *net.UDPAddr, sequence uint32, payload []byte) {
	t.followMovementFrom(peer, slot, remote, sequence, payload, false)
}
func (t *townRobotTransport) followMovementFrom(peer *townRobotUDP, slot byte, remote *net.UDPAddr, sequence uint32, payload []byte, reliable bool) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.closed || !t.completed || t.room == nil || peer.remote == nil || !peer.remote.IP.Equal(remote.IP) || peer.remote.Port != remote.Port || (!reliable && sequence < peer.nextInSequence) {
		return
	}
	body, e := protocol.RobotFollowMove115(payload, t.ownerActor, peer.actor, *t.room)
	if e != nil {
		return
	}
	if !reliable {
		peer.nextInSequence = sequence + 1
	}
	if t.nativeAIControlsLocked() {
		return
	}
	peer.history = append(peer.history, body)
	if len(peer.history) > 8 {
		peer.history = peer.history[1:]
	}
	// Replay a short trail, retaining the native movement scalar grammar.
	lag := int(slot) * 2
	index := len(peer.history) - 1 - lag
	if index < 0 {
		index = 0
	}
	movement := append([]byte(nil), peer.history[index]...)
	// Native145e05df0 bit4 forces an immediate position copy. Keep the first
	// room placement; subsequent owner samples use native interpolation.
	if len(peer.history) > 1 {
		movement[14] &^= 0x10
	}
	raw, e := wire.EncodeNativeUDP115(slot, peer.key, peer.nextOutSequence, movement)
	if e != nil {
		return
	}
	if _, e = peer.conn.WriteToUDP(raw, remote); e != nil {
		return
	}
	peer.nextOutSequence++
	if peer.followSamples < 3 {
		peer.followSamples++
		t.event(map[string]any{"kind": "town_robot_dungeon_follow", "slot": slot, "actor": peer.actor, "room": *t.room, "sample": peer.followSamples})
	}
}

// Explicit isolated diagnostic: bounded ciphertext only, no session keys and
// no decoding, sending or protocol changes. The owner manually reproduces once.
func (t *townRobotTransport) observeUDP(peer *townRobotUDP, slot byte, p []byte) {
	if os.Getenv("DFO_TOWN_ROBOT_UDP_OBSERVE") != "r12-observe1" || len(p) == 0 || len(p) > 4096 {
		return
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.closed || peer.observeTotal >= 96 {
		return
	}
	if peer.observeEpoch != t.observeEpoch {
		peer.observeEpoch = t.observeEpoch
		peer.observeCount = 0
	}
	if peer.observeCount >= 24 {
		return
	}
	peer.observeCount++
	peer.observeTotal++
	var room any
	if t.room != nil {
		room = *t.room
	}
	t.event(map[string]any{"kind": "town_robot_udp_observe", "slot": slot, "epoch": t.observeEpoch, "room": room, "sample": peer.observeCount, "total": peer.observeTotal, "bytes": len(p), "cipher_hex": hex.EncodeToString(p)})
}

// Initial state can precede either native loading completion or TCP/UDP
// coordination. Release it only after both gates; normal room transitions
// keep the established transport and retain their existing sync sequence.
func (t *townRobotTransport) flushRoomSyncLocked() {
	if t.closed || !t.completed || t.room == nil {
		return
	}
	for i, p := range t.peers {
		for _, id := range []uint16{77, 192, 195} {
			if body := p.roomSync[id]; body != nil {
				t.sendReliableLocked(p, byte(i+1), body)
			}
		}
		p.roomSync = nil
	}
}
