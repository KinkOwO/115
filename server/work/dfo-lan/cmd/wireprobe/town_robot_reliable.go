package main

import (
	"dfolan/internal/game/protocol"
	"dfolan/internal/game/wire"
	"encoding/binary"
	"net"
	"time"
)

type robotReliablePending struct {
	raw      []byte
	sent     time.Time
	attempts uint8
}

func (t *townRobotTransport) receiveReliable(peer *townRobotUDP, slot byte, remote *net.UDPAddr, f wire.NativeUDPFrame115) {
	t.mu.Lock()
	if t.closed || f.Slot != 0 || peer.remote == nil || !peer.remote.IP.Equal(remote.IP) || peer.remote.Port != remote.Port {
		t.mu.Unlock()
		return
	}
	if f.Type == 0 {
		p := f.Payload
		if len(p) >= 6 && p[4] <= 245 && len(p) == 6+int(p[4])/8 {
			base := binary.LittleEndian.Uint32(p)
			if base <= peer.nextReliableOut {
				for seq := range peer.pendingReliable {
					if seq < base || (seq-base < uint32(p[4]) && p[5+(seq-base)/8]&(0x80>>((seq-base)%8)) != 0) {
						delete(peer.pendingReliable, seq)
					}
				}
			}
		}
		t.mu.Unlock()
		return
	}
	// Validate the complete bundle before acknowledging or publishing effects.
	messages, err := wire.SplitNativeUDPReliable115(f.Payload)
	if err != nil || f.Sequence-peer.nextReliableIn > 256 && f.Sequence >= peer.nextReliableIn {
		t.mu.Unlock()
		return
	}
	if f.Sequence >= peer.nextReliableIn {
		if peer.reliableIn == nil {
			peer.reliableIn = make(map[uint32][][]byte)
		}
		if len(peer.reliableIn) >= 64 {
			t.mu.Unlock()
			return
		}
		peer.reliableIn[f.Sequence] = messages
	}
	var ready [][]byte
	for {
		rows, ok := peer.reliableIn[peer.nextReliableIn]
		if !ok {
			break
		}
		delete(peer.reliableIn, peer.nextReliableIn)
		peer.nextReliableIn++
		ready = append(ready, rows...)
	}
	ack, err := wire.EncodeNativeUDPAck115(slot, peer.key, peer.nextReliableIn)
	if err == nil {
		peer.conn.WriteToUDP(ack, remote)
	}
	t.mu.Unlock()
	for _, p := range ready {
		if len(p) >= 4 && p[0] <= 2 && binary.LittleEndian.Uint16(p[1:]) == 4 {
			t.followMovementFrom(peer, slot, remote, 0, p, true)
		} else {
			t.roomSync(peer, slot, p)
		}
	}
}

func (t *townRobotTransport) roomSync(peer *townRobotUDP, slot byte, p []byte) {
	body, err := protocol.RobotRoomSync115(p, slot)
	if err != nil {
		return
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.closed || peer.remote == nil || (t.room == nil && !t.roomLoading) {
		return
	}
	if t.room == nil || !t.completed {
		if peer.roomSync == nil {
			peer.roomSync = make(map[uint16][]byte)
		}
		peer.roomSync[binary.LittleEndian.Uint16(body[1:])] = body
		return
	}
	t.sendReliableLocked(peer, slot, body)
}

func (t *townRobotTransport) sendReliableLocked(peer *townRobotUDP, slot byte, p []byte) {
	if peer.remote == nil || len(peer.pendingReliable) >= 64 {
		return
	}
	bundle := make([]byte, 2+len(p))
	binary.LittleEndian.PutUint16(bundle, uint16(len(p)))
	copy(bundle[2:], p)
	raw, err := wire.EncodeNativeUDPReliable115(slot, peer.key, peer.nextReliableOut, bundle)
	if err != nil {
		return
	}
	if _, err = peer.conn.WriteToUDP(raw, peer.remote); err != nil {
		return
	}
	if peer.pendingReliable == nil {
		peer.pendingReliable = make(map[uint32]*robotReliablePending)
	}
	peer.pendingReliable[peer.nextReliableOut] = &robotReliablePending{raw: raw, sent: time.Now(), attempts: 1}
	peer.nextReliableOut++
	t.event(map[string]any{"kind": "town_robot_room_sync", "slot": slot, "command": binary.LittleEndian.Uint16(p[1:])})
}

func (t *townRobotTransport) resendReliable(peer *townRobotUDP) {
	tick := time.NewTicker(150 * time.Millisecond)
	defer tick.Stop()
	for now := range tick.C {
		t.mu.Lock()
		if t.closed {
			t.mu.Unlock()
			return
		}
		if len(t.peers) > 0 && t.peers[0] == peer {
			t.publishAuthorityLocked(false, false)
		}
		for seq, p := range peer.pendingReliable {
			if p.attempts >= 120 {
				if p.attempts == 120 {
					p.attempts++
					t.event(map[string]any{"kind": "town_robot_reliable_expired", "sequence": seq})
				}
				continue
			}
			if now.Sub(p.sent) >= 150*time.Millisecond {
				peer.conn.WriteToUDP(p.raw, peer.remote)
				p.sent = now
				p.attempts++
			}
		}
		t.mu.Unlock()
	}
}
