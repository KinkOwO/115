package main

import (
	"dfolan/internal/catalog"
	"dfolan/internal/channelrefresh"
	"dfolan/internal/database"
	"dfolan/internal/game/wire"
	"encoding/binary"
	"fmt"
	"net"
	"os"
	"testing"
	"time"
)

func readRaidRoomFrame(t *testing.T, peer net.Conn, keys []byte, kind byte, id uint16) []byte {
	t.Helper()
	frame := readDispatchServerFrame(t, peer)
	if frame[0] != kind || binary.LittleEndian.Uint16(frame[1:3]) != id {
		t.Fatalf("room sequence expected %d/%d, got %x", kind, id, frame[:3])
	}
	body, err := wire.DecryptPayload(keys, id, frame[wire.ServerHeaderSize:])
	if err != nil {
		t.Fatal(err)
	}
	return body
}

func TestDelezieRoomsFullGatewayCycleNativePVF(t *testing.T) {
	archive := os.Getenv("DFO_PVF_CORE_TEST_ARCHIVE")
	if archive == "" {
		t.Skip("native PVF archive required")
	}
	a, err := catalog.OpenTestArchiveCached(archive, os.Getenv("DFO_PVF_CORE_TEST_SHA256"))
	if err != nil {
		t.Fatal(err)
	}
	directory, err := catalog.ImportChannelDirectory(a)
	if err != nil {
		t.Fatal(err)
	}
	entries, err := catalog.ImportRaidEntrances(a, &directory)
	if err != nil {
		t.Fatal(err)
	}
	worldCatalog, err := catalog.ImportWorldRuntime(a)
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("DFO_RAID_OPEN_EVENTS_PROBE", "1")
	for _, typ := range []uint32{112, 120} {
		entry := entries[typ]
		for _, difficulty := range entry.Delezie.ChannelModes[typ] {
			t.Run(fmt.Sprintf("%d/%s", typ, difficulty.Name()), func(t *testing.T) {
				server, peer := net.Pipe()
				defer server.Close()
				defer peer.Close()
				peer.SetDeadline(time.Now().Add(10 * time.Second))
				keys := make([]byte, wire.SessionKeyBytes)
				log := func(map[string]any) {}
				x, y := entry.Waiting.Spawn()
				orphan := database.WorldPosition{Town: entry.Waiting.TownID, Area: entry.Waiting.AreaID, X: x, Y: y}
				stored := &raidRoomWorldStore{saved: database.WorldState{Position: orphan, Revision: 27}}
				w := raidRoomWorld(entry, stored)
				w.channelType = typ
				w.service.Catalog = worldCatalog
				if err := w.enter(raidRoomRole(), orphan); err != nil {
					t.Fatal(err)
				}
				prep := w.state.Position
				if prep.Town != entry.Recruiting.TownID || prep.Area != entry.Recruiting.AreaID {
					t.Fatal("fresh connection restored orphan waiting room")
				}
				runtime := &gatewayRuntime{raidEntrances: entries}
				c := &gameConnection{gatewayRuntime: runtime, worldState: w, channel: typ, channelCfg: channelrefresh.Config{ServerID: 1}, bootstrapped: true, event: log, output: newConnectionOutput(server, keys, "test", log)}
				result := make(chan dispatchAction, 1)
				dispatch := func(id uint16, body []byte) {
					go func() {
						result <- c.dispatchRaidEntrance(&clientRequest{frame: wire.Frame{Type: 1, ID: id}, plaintext: body, verified: true})
					}()
				}
				body := binary.LittleEndian.AppendUint32([]byte{14}, 4)
				body = append(body, []byte("room")...)
				body = append(body, 0, 0, 0, 7, byte(difficulty), 0)
				body = binary.LittleEndian.AppendUint32(body, 0)
				for cycle := 0; cycle < 2; cycle++ {
					dispatch(656, body)
					readRaidRoomFrame(t, peer, keys, 0, 578)
					ack := readRaidRoomFrame(t, peer, keys, 1, 656)
					if len(ack) < 5 || ack[0] != 1 {
						t.Fatal("room creation rejected")
					}
					id := binary.LittleEndian.Uint32(ack[1:5])
					place := readRaidRoomFrame(t, peer, keys, 0, 23)
					scene := readRaidRoomFrame(t, peer, keys, 0, 24)
					if <-result != dispatchHandled || !w.raidWaiting || binary.LittleEndian.Uint32(place[2:6]) != entry.Waiting.TownID || binary.LittleEndian.Uint32(place[6:10]) != entry.Waiting.AreaID || binary.LittleEndian.Uint32(scene) != entry.Waiting.TownID || binary.LittleEndian.Uint32(scene[4:8]) != entry.Waiting.AreaID || !w.privateArea(w.state.Position) {
						t.Fatal("owned waiting scene not sent")
					}
					team, owned := c.ownedRaidTeam()
					if !owned || team.Recruitment.ID != id || team.Recruitment.MemberArea != uint16(entry.Waiting.AreaID) || team.Recruitment.Create.Options[4] != byte(difficulty) {
						t.Fatal("room ownership or native member area lost")
					}
					// Reopen more than the request sampling limit, alternating both
					// native query branches, and consume the completion notification.
					for i := 0; i < BodySampleLimit+2; i++ {
						mode := byte(i%2 + 1)
						if !observedGameRequest(1353) {
							t.Fatal("repeated room query would stop being decoded")
						}
						dispatch(1353, binary.LittleEndian.AppendUint32([]byte{mode}, id))
						info := readRaidRoomFrame(t, peer, keys, 1, 1353)
						complete := readRaidRoomFrame(t, peer, keys, 0, 1220)
						if <-result != dispatchHandled || len(info) < 6 || info[0] != 1 || info[1] != mode || binary.LittleEndian.Uint32(info[2:6]) != id || len(complete) == 0 || complete[0] != 0 {
							t.Fatal("room query did not complete")
						}
					}
					assignment := binary.LittleEndian.AppendUint32(nil, 0)
					assignment = binary.LittleEndian.AppendUint32(assignment, uint32(w.role.WireID))
					assignment = binary.LittleEndian.AppendUint32(assignment, entry.MemberMax-1)
					dispatch(661, assignment)
					readRaidRoomFrame(t, peer, keys, 0, 578)
					if assigned := readRaidRoomFrame(t, peer, keys, 1, 661); assigned[0] != 1 || <-result != dispatchHandled {
						t.Fatal("member assignment failed")
					}
					team, owned = c.ownedRaidTeam()
					if !owned || uint32(team.Recruitment.MemberPosition) != entry.MemberMax-1 {
						t.Fatal("source member limit not applied")
					}
					dispatch(657, []byte{2, 0, 0, 0, 0, 0, 0, 0})
					leave := readRaidRoomFrame(t, peer, keys, 1, 657)
					place = readRaidRoomFrame(t, peer, keys, 0, 23)
					scene = readRaidRoomFrame(t, peer, keys, 0, 24)
					readRaidRoomFrame(t, peer, keys, 0, 577)
					if <-result != dispatchHandled || leave[0] != 1 || leave[1] != 0 || w.raidWaiting || w.state.Position != prep || c.ownedRaidID != 0 || c.raidOwnerRole != 0 || binary.LittleEndian.Uint32(place[6:10]) != entry.Recruiting.AreaID || binary.LittleEndian.Uint32(scene[4:8]) != entry.Recruiting.AreaID {
						t.Fatal("leave did not restore preparation")
					}
					if _, owned := runtime.raidTeams.get(w.role.ID, typ); owned {
						t.Fatal("leave retained team")
					}
				}
				if stored.saved.Position != orphan || w.state.Revision != 27 {
					t.Fatal("room cycle rewrote saved world position")
				}
				t.Logf("%s: preparation %d/%d -> waiting %d/%d -> queries/assignment -> leave/recreate passed", difficulty.Name(), prep.Town, prep.Area, entry.Waiting.TownID, entry.Waiting.AreaID)
			})
		}
	}
}
