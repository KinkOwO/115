package main

import (
	"bytes"
	"dfolan/internal/game/protocol"
	"dfolan/internal/game/wire"
	"encoding/binary"
	"net"
	"testing"
	"time"
)

func TestTownRobotTransportRequiresBidirectionalNativeUDP(t *testing.T) {
	for _, count := range []int{1, 3} {
		t.Run(string(rune('0'+count)), func(t *testing.T) {
			results := make(chan []byte, 4)
			tr := &townRobotTransport{ownerActor: 3, ownerKey: 0x4453eb5, event: func(map[string]any) {}, send: func(kind byte, id uint16, p []byte) error {
				if kind != 0 || id != 2373 {
					t.Errorf("unexpected result")
				}
				results <- bytes.Clone(p)
				return nil
			}}
			defer tr.close()
			for i := 0; i < count; i++ {
				if e := tr.add(uint16(65534 - i)); e != nil {
					t.Fatal(e)
				}
			}
			for i, p := range tr.peers {
				// A TCP claim alone cannot complete the connection.
				if e := tr.check(protocol.PartyConnectionCheck115{Mode: 1, Slot: uint16(i + 1), Mask: 2}); e != nil {
					t.Fatal(e)
				}
				select {
				case <-results:
					t.Fatal("TCP claim accepted without UDP")
				default:
				}
				c, e := net.DialUDP("udp4", nil, p.conn.LocalAddr().(*net.UDPAddr))
				if e != nil {
					t.Fatal(e)
				}
				defer c.Close()
				body, _ := protocol.PartyCheckConnectUDP115(0, 4, 1, 3)
				raw, _ := wire.EncodeNativeUDP115(0, tr.ownerKey, 0, body)
				// A valid checksum with a foreign actor cannot authorize this party.
				foreign := bytes.Clone(body)
				binary.LittleEndian.PutUint16(foreign[6:], 4)
				bad, _ := wire.EncodeNativeUDP115(0, tr.ownerKey, 0, foreign)
				c.SetReadDeadline(time.Now().Add(40 * time.Millisecond))
				c.Write(bad)
				buf := make([]byte, 512)
				if _, e = c.Read(buf); e == nil {
					t.Fatal("foreign actor received handshake")
				}
				c.SetReadDeadline(time.Now().Add(time.Second))
				if _, e = c.Write(raw); e != nil {
					t.Fatal(e)
				}
				n, e := c.Read(buf)
				if e != nil {
					t.Fatal(e)
				}
				slot, _, reply, e := wire.DecodeNativeUDP115(buf[:n], p.key)
				expected, _ := protocol.PartyCheckConnectUDP115(byte(i+1), 4, 1, p.actor)
				if e != nil || slot != byte(i+1) || !bytes.Equal(reply, expected) {
					t.Fatal("native reply mismatch", e)
				}
				if i < count-1 {
					select {
					case <-results:
						t.Fatal("completed before all members connected")
					default:
					}
				}
			}
			select {
			case p := <-results:
				expected, _ := protocol.PartyDirectConnectionResult115(bytes.Repeat([]byte{2}, count))
				if !bytes.Equal(p, expected) {
					t.Fatalf("wrong proof %x", p)
				}
			case <-time.After(time.Second):
				t.Fatal("no connection result")
			}
			if e := tr.check(protocol.PartyConnectionCheck115{Mode: 1, Slot: 1, Mask: 2}); e != nil {
				t.Fatal(e)
			}
			select {
			case <-results:
				t.Fatal("duplicate connection completion")
			default:
			}
			addresses := []*net.UDPAddr{}
			for _, p := range tr.peers {
				addresses = append(addresses, p.conn.LocalAddr().(*net.UDPAddr))
			}
			tr.close()
			for _, address := range addresses {
				c, e := net.ListenUDP("udp4", address)
				if e != nil {
					t.Fatal("endpoint leaked after close", e)
				}
				c.Close()
			}
		})
	}
}

func TestTownRobotTransportUDPAloneCannotFinish(t *testing.T) {
	tr := &townRobotTransport{ownerActor: 3, ownerKey: 1, event: func(map[string]any) {}, send: func(byte, uint16, []byte) error { t.Error("UDP alone completed party"); return nil }}
	defer tr.close()
	if e := tr.add(65534); e != nil {
		t.Fatal(e)
	}
	c, e := net.DialUDP("udp4", nil, tr.peers[0].conn.LocalAddr().(*net.UDPAddr))
	if e != nil {
		t.Fatal(e)
	}
	defer c.Close()
	p, _ := protocol.PartyCheckConnectUDP115(0, 4, 1, 3)
	raw, _ := wire.EncodeNativeUDP115(0, 1, 0, p)
	c.Write(raw)
	c.SetReadDeadline(time.Now().Add(time.Second))
	if _, e = c.Read(make([]byte, 512)); e != nil {
		t.Fatal(e)
	}
	tr.mu.Lock()
	defer tr.mu.Unlock()
	if tr.completed || tr.peers[0].received != 2 {
		t.Fatal("incorrect one-way state")
	}
}

func TestTownRobotTransportNativeSequenceGate(t *testing.T) {
	tr := &townRobotTransport{ownerActor: 2, ownerKey: 0x11223344, event: func(map[string]any) {}, send: func(byte, uint16, []byte) error { t.Error("sequence test completed without TCP proof"); return nil }}
	defer tr.close()
	if e := tr.add(65534); e != nil {
		t.Fatal(e)
	}
	c, e := net.DialUDP("udp4", nil, tr.peers[0].conn.LocalAddr().(*net.UDPAddr))
	if e != nil {
		t.Fatal(e)
	}
	defer c.Close()
	body, _ := protocol.PartyCheckConnectUDP115(0, 4, 1, 2)
	buf := make([]byte, 512)
	for i, sequence := range []uint32{0x10203040, 0x10203040, 0x10203041} {
		raw, _ := wire.EncodeNativeUDP115(0, tr.ownerKey, sequence, body)
		if _, e = c.Write(raw); e != nil {
			t.Fatal(e)
		}
		if i == 1 {
			c.SetReadDeadline(time.Now().Add(40 * time.Millisecond))
			if _, e = c.Read(buf); e == nil {
				t.Fatal("duplicate native sequence accepted")
			}
			continue
		}
		c.SetReadDeadline(time.Now().Add(time.Second))
		n, e := c.Read(buf)
		if e != nil {
			t.Fatal(e)
		}
		_, replySequence, _, e := wire.DecodeNativeUDP115(buf[:n], tr.peers[0].key)
		expected := uint32(0)
		if i == 2 {
			expected = 1
		}
		if e != nil || replySequence != expected {
			t.Fatal("non-monotonic robot sequence", e)
		}
	}
}
