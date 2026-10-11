package main

import (
	"bytes"
	"crypto/sha256"
	"dfolan/internal/game/protocol"
	"dfolan/internal/game/wire"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"net"
	"os"
	"testing"
	"time"
)

// These vectors came from the current PE's LOGIN reads/native key store and
// UDP writer/encoder, independently of the Go offset/codec under test.
func TestTownRobotLoginKeyNativeProbe(t *testing.T) {
	var proof struct {
		ClientSHA256 string `json:"client_sha256"`
		Offset       int    `json:"udp_key_offset"`
		Vectors      []struct {
			Payload string `json:"payload_hex"`
			Key     uint32 `json:"key"`
			Probe   string `json:"native_probe_hex"`
		} `json:"vectors"`
		Fixture struct {
			SHA256 string `json:"sha256"`
			Key    uint32 `json:"key"`
			Probe  string `json:"native_probe_hex"`
		} `json:"actual_login_fixture"`
	}
	data, e := os.ReadFile("testdata/native_login_udp_key115.json")
	if e != nil {
		t.Fatal(e)
	}
	if e = json.Unmarshal(data, &proof); e != nil {
		t.Fatal(e)
	}
	if proof.ClientSHA256 != "515fefa1c71577a15f90513f860270320f3502bc492ad596f10cf176b91c21eb" || proof.Offset != 5 || len(proof.Vectors) != 2 {
		t.Fatal("unrecognized native evidence")
	}
	for i := 0; i <= len(proof.Vectors); i++ {
		name := "native-vector-" + string(rune('0'+i))
		if i == len(proof.Vectors) {
			name = "actual-login-fixture"
		}
		t.Run(name, func(t *testing.T) {
			c, _, _ := newDispatchTestClient()
			var raw, plain []byte
			var key uint32
			var probe string
			if i < len(proof.Vectors) {
				v := proof.Vectors[i]
				plain, e = hex.DecodeString(v.Payload)
				if e != nil {
					t.Fatal(e)
				}
				key, probe = v.Key, v.Probe
				encrypted, e := wire.EncryptPayload(c.keys, 1, plain)
				if e != nil {
					t.Fatal(e)
				}
				raw, e = wire.ServerFrame(1, 1, encrypted)
				if e != nil {
					t.Fatal(e)
				}
			} else {
				for j := range c.keys {
					c.keys[j] = byte(j%127 + 1)
				}
				raw, e = os.ReadFile("testdata/login-normal22.bin")
				if e != nil {
					t.Fatal(e)
				}
				sum := sha256.Sum256(raw)
				if hex.EncodeToString(sum[:]) != proof.Fixture.SHA256 {
					t.Fatal("LOGIN fixture differs from native proof")
				}
				plain, e = wire.DecryptPayload(c.keys, 1, raw[wire.ServerHeaderSize:])
				if e != nil {
					t.Fatal(e)
				}
				key, probe = proof.Fixture.Key, proof.Fixture.Probe
			}
			c.responses = map[uint16][]byte{1: raw}
			actual, e := c.townRobotNativeKey()
			if e != nil || actual != key {
				t.Fatalf("key differs from native store: %v", e)
			}
			native, e := hex.DecodeString(probe)
			if e != nil {
				t.Fatal(e)
			}
			old := binary.LittleEndian.Uint32(plain[7:11])
			if old == actual {
				t.Fatal("vector does not distinguish the former offset")
			}
			if _, _, p, e := wire.DecodeNativeUDP115(native, old); e == nil && bytes.Equal(p, []byte{0, 0, 0, 0, 4, 1, 3, 0}) {
				t.Fatal("former offset accepted native probe")
			}
			results := make(chan []byte, 1)
			events := make(chan map[string]any, 32)
			tr := &townRobotTransport{ownerActor: 3, ownerKey: actual, event: func(v map[string]any) { events <- v }, send: func(kind byte, id uint16, p []byte) error {
				if kind != 0 || id != 2373 {
					t.Error("unexpected completion")
				}
				results <- bytes.Clone(p)
				return nil
			}}
			defer tr.close()
			if e = tr.add(65534); e != nil {
				t.Fatal(e)
			}
			peer := tr.peers[0]
			udp, e := net.DialUDP("udp4", nil, peer.conn.LocalAddr().(*net.UDPAddr))
			if e != nil {
				t.Fatal(e)
			}
			defer udp.Close()
			// Prove that rejected packets are observable without logging raw data/key.
			bad := bytes.Clone(native)
			bad[len(bad)-1] ^= 1
			for j := 0; j < 10; j++ {
				if _, e = udp.Write(bad); e != nil {
					t.Fatal(e)
				}
			}
			if _, e = udp.Write(native); e != nil {
				t.Fatal(e)
			}
			udp.SetReadDeadline(time.Now().Add(time.Second))
			buf := make([]byte, 512)
			n, e := udp.Read(buf)
			if e != nil {
				t.Fatal("native LOGIN -> UDP receive failed", e)
			}
			slot, _, p, e := wire.DecodeNativeUDP115(buf[:n], peer.key)
			expected, _ := protocol.PartyCheckConnectUDP115(1, 4, 1, 65534)
			if e != nil || slot != 1 || !bytes.Equal(p, expected) {
				t.Fatal("native probe reply mismatch", e)
			}
			rejects := 0
			deadline := time.After(time.Second)
		eventLoop:
			for {
				select {
				case v := <-events:
					if v["kind"] == "town_robot_party_udp_check" {
						break eventLoop
					}
					if v["kind"] != "town_robot_party_udp_rejected" || v["stage"] != "decode" {
						t.Fatal("unexpected diagnostic", v)
					}
					for _, k := range []string{"key", "owner_key", "raw", "payload"} {
						if _, ok := v[k]; ok {
							t.Fatal("sensitive diagnostic field")
						}
					}
					rejects++
				case <-deadline:
					t.Fatal("missing native probe diagnostic")
				}
			}
			if rejects != 8 {
				t.Fatalf("rejection diagnostics are not bounded: %d", rejects)
			}
			select {
			case <-results:
				t.Fatal("completed without native TCP confirmation")
			default:
			}
			if e = tr.check(protocol.PartyConnectionCheck115{Mode: 1, Slot: 1, Mask: 2}); e != nil {
				t.Fatal(e)
			}
			select {
			case result := <-results:
				expected, _ := protocol.PartyDirectConnectionResult115([]byte{2})
				if !bytes.Equal(result, expected) {
					t.Fatal("wrong connection completion")
				}
			case <-time.After(time.Second):
				t.Fatal("missing completion after both directions")
			}
		})
	}
}

func TestTownRobotLoginKeyRejectsMissingResponse(t *testing.T) {
	c, _, _ := newDispatchTestClient()
	if _, e := c.townRobotNativeKey(); e == nil {
		t.Fatal("accepted missing native login")
	}
	c.responses = map[uint16][]byte{1: {1, 1, 0}}
	if _, e := c.townRobotNativeKey(); e == nil {
		t.Fatal("accepted truncated native login")
	}
}
