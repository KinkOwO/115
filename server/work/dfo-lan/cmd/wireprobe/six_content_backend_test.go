package main

import (
	"dfolan/internal/catalog"
	"dfolan/internal/channelrefresh"
	"dfolan/internal/database"
	"dfolan/internal/game/protocol"
	"dfolan/internal/game/wire"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"strings"
	"testing"
	"time"
)

// Runs the actual gateway with current source rules and encrypted output,
// without starting DFO, opening a player DB or claiming scene acceptance.
func TestRequestedRaidBackendCreateAndStartNativePVF(t *testing.T) {
	path := os.Getenv("DFO_PVF_CORE_TEST_ARCHIVE")
	if path == "" {
		t.Skip("native PVF archive required")
	}
	a, err := catalog.OpenTestArchiveCached(path, os.Getenv("DFO_PVF_CORE_TEST_SHA256"))
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
	t.Setenv("DFO_RAID_OPEN_EVENTS_PROBE", "1")
	for _, test := range []struct {
		channel uint32
		mode    byte
		allowed bool
	}{
		{93, 0, true}, {98, 0, true}, {107, 0, true}, {111, 0, true},
		{112, 0, true}, {112, 1, true}, {112, 2, false}, {112, 3, false},
		{120, 0, false}, {120, 1, false}, {120, 2, true}, {120, 3, true},
	} {
		t.Run(fmt.Sprintf("%d/%d", test.channel, test.mode), func(t *testing.T) {
			typ := test.channel
			server, peer := net.Pipe()
			defer server.Close()
			defer peer.Close()
			peer.SetDeadline(time.Now().Add(5 * time.Second))
			keys := make([]byte, wire.SessionKeyBytes)
			var refusal string
			var created map[string]any
			var startBlocked map[string]any
			log := func(e map[string]any) {
				if e["kind"] == "raid_entry_refused" {
					refusal, _ = e["reason"].(string)
				}
				if e["kind"] == "raid_created_waiting" {
					created = e
				}
				if e["kind"] == "delezie_start_blocked" {
					startBlocked = e
				}
			}
			runtime := &gatewayRuntime{raidEntrances: entries}
			origin := database.WorldPosition{Town: entries[typ].Waiting.TownID, Area: 1, X: 50, Y: 60}
			w := &worldSession{role: database.Character{ID: 1, WireID: 9, Profession: 16, Name: "test", State: []byte(`{"level":115,"advancement":3,"awakening":3}`)}, level: 115, channelType: typ, state: database.WorldState{Position: origin}}
			c := &gameConnection{gatewayRuntime: runtime, worldState: w, channel: typ, channelCfg: channelrefresh.Config{ServerID: 1}, bootstrapped: true, event: log, output: newConnectionOutput(server, keys, "test", log)}
			kind, ok := protocol.RaidKindForChannel(typ)
			if !ok {
				t.Fatal("missing native kind")
			}
			p := binary.LittleEndian.AppendUint32([]byte{kind}, 4)
			p = append(p, []byte("test")...)
			p = append(p, 0, 0, 0, 7, test.mode, 0)
			p = binary.LittleEndian.AppendUint32(p, 0)
			result := make(chan dispatchAction, 1)
			go func() {
				result <- c.dispatchRaidEntrance(&clientRequest{frame: wire.Frame{Type: 1, ID: 656}, plaintext: p, verified: true})
			}()
			if !test.allowed {
				frame := readDispatchServerFrame(t, peer)
				body, err := wire.DecryptPayload(keys, 656, frame[wire.ServerHeaderSize:])
				if err != nil || binary.LittleEndian.Uint16(frame[1:3]) != 656 || len(body) == 0 || body[0] != 0 || <-result != dispatchHandled || !strings.Contains(refusal, "difficulty") {
					t.Fatalf("cross-mode create did not refuse: %x %v reason=%s", body, err, refusal)
				}
				if _, owned := runtime.raidTeams.get(1, typ); owned || runtime.raidTeams.next != 0 || w.raidWaiting || w.state.Position != origin {
					t.Fatal("cross-mode create changed team or position")
				}
				return
			}
			for _, id := range []uint16{578, 656, 23, 24} {
				frame := readDispatchServerFrame(t, peer)
				if binary.LittleEndian.Uint16(frame[1:3]) != id {
					t.Fatalf("create sequence expected %d: %x; refusal=%s", id, frame[:3], refusal)
				}
				if id == 656 {
					body, err := wire.DecryptPayload(keys, id, frame[wire.ServerHeaderSize:])
					if err != nil || len(body) == 0 || body[0] != 1 {
						t.Fatalf("create response %x: %v", body, err)
					}
				}
				if id == 578 && kind == 14 {
					body, err := wire.DecryptPayload(keys, id, frame[wire.ServerHeaderSize:])
					if err != nil || len(body) < 16 {
						t.Fatalf("owned details decode: %v", err)
					}
					// 144CDBF80's variable title precedes kind/state, leader and
					// nine u8 fields ending at native outer+129 (difficulty).
					titleLength := int(binary.LittleEndian.Uint32(body[12:16]))
					difficultyAt := 16 + titleLength + 3 + 4 + 8
					if difficultyAt >= len(body) || body[difficultyAt] != test.mode {
						t.Fatalf("owned details did not echo mode%d at native difficulty slot: %x", test.mode, body)
					}
				}
			}
			if <-result != dispatchHandled || !w.raidWaiting || w.state.Position.Area != 2 {
				t.Fatal("waiting entry failed")
			}
			team, owned := runtime.raidTeams.get(1, typ)
			if !owned || team.Recruitment.Create.Kind != kind || team.Recruitment.Create.Options[4] != test.mode {
				t.Fatal("team not owned or wrong mode")
			}
			if kind == 14 {
				mode, err := entries[typ].Delezie.ModeForChannel(typ, catalog.DelezieDifficulty(test.mode))
				if err != nil || created["difficulty"] != mode.Difficulty.Name() || created["phase_source"] != mode.Phase.Table.Path {
					t.Fatalf("mode/source audit was lost: %v err=%v", created, err)
				}
			}
			for _, id := range []uint16{658, 2089} {
				startBody := make([]byte, 8)
				if id == 2089 {
					startBody = make([]byte, 18)
					binary.LittleEndian.PutUint32(startBody[14:], 1)
				}
				go func() {
					result <- c.dispatchRaidEntrance(&clientRequest{frame: wire.Frame{Type: 1, ID: id}, plaintext: startBody, verified: true})
				}()
				frame := readDispatchServerFrame(t, peer)
				body, err := wire.DecryptPayload(keys, id, frame[wire.ServerHeaderSize:])
				if err != nil || binary.LittleEndian.Uint16(frame[1:3]) != id || len(body) == 0 || body[0] != 0 || <-result != dispatchHandled {
					t.Fatalf("unsupported start did not refuse: %x %v", body, err)
				}
				if !strings.Contains(refusal, "executor is not implemented") || w.activeDungeon != nil || w.bakalOpening != nil {
					t.Fatalf("unsupported phase changed scene: %s", refusal)
				}
				if kind == 14 {
					mode := entries[typ].Delezie.Modes[catalog.DelezieDifficulty(test.mode)]
					supported := []int{2, 1, 0, 0}[test.mode]
					if startBlocked["mode"] != mode.Difficulty.Name() || startBlocked["source"] != mode.Phase.Table.Path || startBlocked["events_total"] != len(mode.Phases[0].Events) || startBlocked["events_supported"] != supported || startBlocked["initial_notifications_ready"] != false || startBlocked["first_unresolved_reason"] == "" || !strings.Contains(refusal, "initial object/HP notification contract incomplete") {
						t.Fatalf("owned start did not inspect full selected source: %v refusal=%s", startBlocked, refusal)
					}
					got, ok := runtime.raidTeams.get(1, typ)
					if !ok || got.Recruitment.State != team.Recruitment.State || got.Recruitment.MemberPosition != team.Recruitment.MemberPosition || !w.raidWaiting || w.state.Position.Area != 2 {
						t.Fatal("source preflight mutated the waiting team")
					}
				}
			}
			if kind == 14 {
				var savedState map[string]any
				if err := json.Unmarshal(w.role.State, &savedState); err != nil {
					t.Fatal(err)
				}
				savedState["delezie_raid_rewards"] = map[string]any{"Week": database.IspinsWeekStart(time.Now()).Format(time.RFC3339), "Clears": 1, "Rewards": 1}
				w.role.State, err = json.Marshal(savedState)
				if err != nil {
					t.Fatal(err)
				}
				position, nextID := w.state.Position, runtime.raidTeams.next
				for _, id := range []uint16{656, 658, 2089} {
					body := make([]byte, 8)
					if id == 656 {
						body = p
					} else if id == 2089 {
						body = make([]byte, 18)
						binary.LittleEndian.PutUint32(body[14:], 1)
					}
					go func() {
						result <- c.dispatchRaidEntrance(&clientRequest{frame: wire.Frame{Type: 1, ID: id}, plaintext: body, verified: true})
					}()
					frame := readDispatchServerFrame(t, peer)
					reply, err := wire.DecryptPayload(keys, id, frame[wire.ServerHeaderSize:])
					if err != nil || binary.LittleEndian.Uint16(frame[1:3]) != id || len(reply) == 0 || reply[0] != 0 || <-result != dispatchHandled || !strings.Contains(refusal, "本周") {
						t.Fatalf("source weekly cap was not enforced for CMD%d: %x err=%v reason=%s", id, reply, err, refusal)
					}
					after, owned := runtime.raidTeams.get(w.role.ID, typ)
					if !owned || after.Recruitment.ID != team.Recruitment.ID || runtime.raidTeams.next != nextID || w.state.Position != position || w.activeDungeon != nil {
						t.Fatal("weekly refusal mutated owned team or scene")
					}
				}
			}
			t.Logf("channel=%d source=%s kind=%d difficulty=%d create=accepted waiting=%d/2 start=unsupported", typ, entries[typ].Path, kind, test.mode, w.state.Position.Town)
		})
	}
}
