package main

import (
	"bytes"
	"dfolan/internal/database"
	"dfolan/internal/game/protocol"
	"dfolan/internal/game/wire"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestTownRobotBothRechallengeRoutesRetainSeatsAcrossRuns(t *testing.T) {
	for _, option := range []byte{0, protocol.SettlementExitSeamless} {
		for _, count := range []int{1, 3} {
			t.Run(string(rune('0'+option))+"/"+string(rune('0'+count)), func(t *testing.T) {
				c, f := robotSettlementFixture(t)
				w := c.worldState
				p := f.parties[w.peer]
				for i := 1; i < count; i++ {
					b := *p.bots[0]
					peer := *b.peer
					peer.actorID -= uint16(i)
					b.peer = &peer
					p.bots = append(p.bots, &b)
					if err := p.transport.add(peer.actorID); err != nil {
						t.Fatal(err)
					}
				}
				partyID, identity := p.id, p.identity
				actors := []uint16{w.peer.actorID}
				for _, b := range p.bots {
					actors = append(actors, b.peer.actorID)
				}
				captured := robotLiveResults(t)[0]
				result := bytes.Clone(captured[:18])
				result[3] = byte(len(actors))
				for _, actor := range actors[2:] {
					row := make([]byte, 7)
					binary.LittleEndian.PutUint16(row, actor)
					result = append(result, row...)
				}
				result = append(result, captured[18:120]...)
				for cycle := 0; cycle < 3; cycle++ {
					old, previous := w.activeDungeon, p.transport
					previous.mu.Lock()
					previous.completed = true
					for _, peer := range previous.peers {
						peer.nextReliableIn = 81
						peer.nextReliableOut = 91
						peer.pendingReliable = map[uint32]*robotReliablePending{90: {raw: []byte{1}}}
					}
					previous.mu.Unlock()
					if _, err := w.dungeonResult(result); err != nil {
						t.Fatal(err)
					}
					w.resultSent = true
					pending, route, err := w.settlementExit([]byte{1, option, 1})
					if err != nil {
						t.Fatal(err)
					}
					next := p.transport
					if next == previous || !previous.closed || next.closed || next.completed || f.parties[w.peer] != p || p.id != partyID || p.identity != identity || pending == nil || pending.RunID == old.RunID || pending.Completed() {
						t.Fatal("restart replaced party or reused old run/transport")
					}
					if countPacket(route, 9) != 2 || countPacket(route, 11) != 1 || countPacket(route, 10) != 0 || countPacket(route, 24) != 0 {
						t.Fatal("rechallenge roster path", route)
					}
					var roster []byte
					last := 0
					for _, packet := range route {
						if packet.Kind == 0 && (packet.ID == 27 || packet.ID == 11 || packet.ID == 9 || packet.ID == 28) {
							order := map[string]int{"dungeon_rechallenge_state": 1, "robot_retry_clear_native_party": 2, "robot_retry_peer_endpoints": 3, "robot_retry_party_roster": 4}[packet.Name]
							if packet.ID == 27 {
								order = 1
							}
							if packet.ID == 28 {
								order = 5
							}
							if order <= last {
								t.Fatal("native constructors out of order")
							}
							last = order
						}
						if packet.Name == "robot_retry_clear_native_party" {
							want, err := protocol.OrdinaryPartyGone115(partyID, identity)
							if err != nil || !bytes.Equal(packet.Payload, want) {
								t.Fatal("native cleanup scope changed", err)
							}
						}
						if packet.Name == "robot_retry_party_roster" {
							roster = packet.Payload
						}
					}
					if roster[69] != byte(count+1) {
						t.Fatal("incomplete roster")
					}
					for i, actor := range actors {
						if roster[70+i*17] != byte(i) || binary.LittleEndian.Uint16(roster[71+i*17:]) != actor || next.ownerActor != actors[0] {
							t.Fatal("identity/seat changed")
						}
						if i > 0 {
							peer := next.peers[i-1]
							if peer.actor != actor || peer.nextReliableIn != 0 || peer.nextReliableOut != 0 || len(peer.pendingReliable) != 0 {
								t.Fatal("old reliable generation carried over")
							}
						}
					}
					if cycle == 0 && option == 0 && os.Getenv("DFO_R33_NATIVE_FIXTURES") != "" {
						body, _ := json.Marshal(map[string]any{"actors": actors, "payload_hex": hex.EncodeToString(roster)})
						if err := os.WriteFile(filepath.Join(os.Getenv("DFO_R33_NATIVE_FIXTURES"), "r33-roster-"+string(rune('0'+count))+".json"), body, 0600); err != nil {
							t.Fatal(err)
						}
					}
					w.activeDungeon = pending
					w.resetCards()
					w.completionSent = false
					w.resultSent = false
					w.leaveScene()
					loaded, err := w.finishDungeonLoading(make([]byte, 16))
					if err != nil || countPacket(loaded, 29) != count || !p.dungeon || p.entryPrepared {
						t.Fatal("restarted room omitted teammate", err)
					}
					pending.MarkCompleted()
					pending.StartedAt = time.Now().Add(-time.Minute)
					w.completionSent = true
				}
				bag, err := w.store.CharacterEventReceipt(t.Context(), w.account, w.role.ID, "clear:"+w.activeDungeon.RunID)
				if err != database.ErrNotFound || len(bag) != 0 {
					t.Fatal("newly marked run unexpectedly received old rewards", err)
				}
			})
		}
	}
}

func TestTownRobotRechallengeBuffersSyncUntilBothLoadAndConnection(t *testing.T) {
	for _, first := range []string{"loaded", "connected"} {
		t.Run(first, func(t *testing.T) {
			tr := &townRobotTransport{ownerActor: 3, ownerKey: 1, event: func(map[string]any) {}, send: func(byte, uint16, []byte) error { return nil }}
			defer tr.close()
			if err := tr.add(65534); err != nil {
				t.Fatal(err)
			}
			remote, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1)})
			if err != nil {
				t.Fatal(err)
			}
			defer remote.Close()
			peer := tr.peers[0]
			peer.remote = remote.LocalAddr().(*net.UDPAddr)
			tr.setRoom(nil)
			body := []byte{1, 192, 0, 0, 0, 3, 0, 0, 0, 7, 0, 0, 0}
			want, err := protocol.RobotRoomSync115(body, 1)
			if err != nil {
				t.Fatal(err)
			}
			tr.roomSync(peer, 1, body)
			if first == "loaded" {
				tr.setRoom(&[2]uint16{0, 0})
			} else {
				tr.mu.Lock()
				peer.received, peer.reported = 2, 2
				err = tr.finishLocked()
				tr.mu.Unlock()
				if err != nil {
					t.Fatal(err)
				}
			}
			tr.mu.Lock()
			if peer.roomSync[192] == nil || len(peer.pendingReliable) != 0 {
				t.Fatal("sync released before both gates")
			}
			tr.mu.Unlock()
			if first == "loaded" {
				tr.mu.Lock()
				peer.received, peer.reported = 2, 2
				err = tr.finishLocked()
				tr.mu.Unlock()
				if err != nil {
					t.Fatal(err)
				}
			} else {
				tr.setRoom(&[2]uint16{0, 0})
			}
			tr.mu.Lock()
			defer tr.mu.Unlock()
			if len(peer.roomSync) != 0 || len(peer.pendingReliable) != 1 {
				t.Fatal("initial state lost or replayed twice")
			}
			frame, err := wire.DecodeNativeUDPDatagram115(peer.pendingReliable[0].raw, peer.key)
			if err != nil {
				t.Fatal(err)
			}
			messages, err := wire.SplitNativeUDPReliable115(frame[0].Payload)
			if err != nil || len(messages) != 1 || !bytes.Equal(messages[0], want) {
				t.Fatal("native initial state changed", err)
			}
		})
	}
}
