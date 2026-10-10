package main

import (
	"dfolan/internal/catalog"
	"dfolan/internal/catalog/pvf"
	"dfolan/internal/channelrefresh"
	"dfolan/internal/database"
	"dfolan/internal/game/protocol"
	"dfolan/internal/game/wire"
	"encoding/binary"
	"fmt"
	"net"
	"strings"
	"testing"
	"time"
)

func delezieModeEntryFixture(t *testing.T) catalog.RaidEntrance {
	t.Helper()
	rules := &catalog.DelezieRaidRules{
		Modes: map[catalog.DelezieDifficulty]*catalog.DelezieModeRules{},
		ChannelModes: map[uint32][]catalog.DelezieDifficulty{
			112: {catalog.DelezieNormal, catalog.DelezieHard},
			120: {catalog.DelezieSingle, catalog.DelezieMatch},
		},
	}
	for difficulty := catalog.DelezieNormal; difficulty <= catalog.DelezieMatch; difficulty++ {
		tree, err := pvf.NewCTPTree(&pvf.CTPTable{Path: "fixture/delezieraid." + difficulty.Name() + ".ctp"})
		if err != nil {
			t.Fatal(err)
		}
		rules.Modes[difficulty] = &catalog.DelezieModeRules{Difficulty: difficulty, Phase: tree}
	}
	return catalog.RaidEntrance{Path: "fixture/delezieraid.normal.ctp", MemberMax: 12, StartMinimum: 1, PartyMax: 3, Delezie: rules,
		Waiting: catalog.TownArea{TownID: 228, AreaID: 2, Walkable: [][4]int32{{10, 20, 300, 400}}}}
}

func TestDelezieModeAdmissionFollowsSourceBinding(t *testing.T) {
	entry := delezieModeEntryFixture(t)
	request := protocol.RaidCreateRequest{Kind: 14, Options: [6]byte{0, 0, 0, 7, 2}}
	if _, err := delezieModeForTeam(entry, 112, request); err == nil {
		t.Fatal("single mode borrowed normal channel rules")
	}
	mode, err := delezieModeForTeam(entry, 120, request)
	if err != nil || mode != entry.Delezie.Modes[catalog.DelezieSingle] {
		t.Fatal("single channel did not select its own source", err)
	}
	// Change the parsed source binding. Admission must change without a
	// second hard-coded channel/difficulty table in the gateway.
	entry.Delezie.ChannelModes[112] = []catalog.DelezieDifficulty{catalog.DelezieSingle}
	if _, err = delezieModeForTeam(entry, 112, request); err != nil {
		t.Fatal("changed source binding ignored", err)
	}
	delete(entry.Delezie.Modes, catalog.DelezieSingle)
	if _, err = delezieModeForTeam(entry, 112, request); err == nil {
		t.Fatal("missing mode silently fell back")
	}
	entry.Delezie.Modes[catalog.DelezieSingle] = entry.Delezie.Modes[catalog.DelezieNormal]
	if _, err = delezieModeForTeam(entry, 112, request); err == nil {
		t.Fatal("mismatched mode identity accepted")
	}
	entry.Delezie = nil
	if _, err = delezieModeForTeam(entry, 112, request); err == nil {
		t.Fatal("missing source accepted")
	}
}

func TestDelezieInvalidCreatePreservesTeamAndWorld(t *testing.T) {
	t.Setenv("DFO_RAID_OPEN_EVENTS_PROBE", "1")
	for _, test := range []struct {
		channel uint32
		mode    byte
	}{
		{112, 2}, {112, 3}, {120, 0}, {120, 1}, {112, 4}, {120, 255},
	} {
		t.Run(fmt.Sprintf("%d/%d", test.channel, test.mode), func(t *testing.T) {
			server, peer := net.Pipe()
			defer server.Close()
			defer peer.Close()
			peer.SetDeadline(time.Now().Add(5 * time.Second))
			keys := make([]byte, wire.SessionKeyBytes)
			var refusal string
			log := func(e map[string]any) {
				if e["kind"] == "raid_entry_refused" {
					refusal, _ = e["reason"].(string)
				}
			}
			entry := delezieModeEntryFixture(t)
			runtime := &gatewayRuntime{raidEntrances: map[uint32]catalog.RaidEntrance{test.channel: entry}}
			origin := database.WorldPosition{Town: 228, Area: 1, X: 50, Y: 60}
			w := &worldSession{role: database.Character{ID: 1, WireID: 9, Name: "test", Profession: 16, State: []byte(`{"level":115,"advancement":3,"awakening":3}`)}, level: 115, channelType: test.channel, state: database.WorldState{Position: origin}}
			c := &gameConnection{gatewayRuntime: runtime, worldState: w, channel: test.channel, channelCfg: channelrefresh.Config{ServerID: 1}, bootstrapped: true, event: log, output: newConnectionOutput(server, keys, "test", log)}
			foreign, err := runtime.raidTeams.create(2, test.channel, 1, protocol.RaidRecruitment{Create: protocol.RaidCreateRequest{Kind: 14, Title: "foreign"}}, origin)
			if err != nil {
				t.Fatal(err)
			}
			allocated := runtime.raidTeams.next
			body := binary.LittleEndian.AppendUint32([]byte{14}, 4)
			body = append(body, "test"...)
			body = append(body, 0, 0, 0, 7, test.mode, 0)
			body = binary.LittleEndian.AppendUint32(body, 0)
			result := make(chan dispatchAction, 1)
			go func() {
				result <- c.dispatchRaidEntrance(&clientRequest{frame: wire.Frame{Type: 1, ID: 656}, plaintext: body, verified: true})
			}()
			frame := readDispatchServerFrame(t, peer)
			payload, err := wire.DecryptPayload(keys, 656, frame[wire.ServerHeaderSize:])
			if err != nil || frame[0] != 1 || binary.LittleEndian.Uint16(frame[1:3]) != 656 || len(payload) == 0 || payload[0] != 0 || <-result != dispatchHandled {
				t.Fatalf("invalid mode was not refused: %x %v", payload, err)
			}
			if !strings.Contains(refusal, "difficulty") {
				t.Fatalf("wrong refusal: %s", refusal)
			}
			if _, owned := runtime.raidTeams.get(1, test.channel); owned || runtime.raidTeams.next != allocated || w.state.Position != origin || w.raidWaiting || c.ownedRaidID != 0 || w.activeDungeon != nil {
				t.Fatal("refused create allocated a team or changed world state")
			}
			if got, ok := runtime.raidTeams.get(2, test.channel); !ok || got.Recruitment.ID != foreign.Recruitment.ID {
				t.Fatal("refused create changed another owner's team")
			}
		})
	}
}

func TestDelezieStartRevalidatesOwnedSourceMode(t *testing.T) {
	t.Setenv("DFO_RAID_OPEN_EVENTS_PROBE", "1")
	for _, id := range []uint16{658, 2089} {
		t.Run(fmt.Sprint(id), func(t *testing.T) {
			server, peer := net.Pipe()
			defer server.Close()
			defer peer.Close()
			peer.SetDeadline(time.Now().Add(5 * time.Second))
			keys := make([]byte, wire.SessionKeyBytes)
			var refusal string
			log := func(e map[string]any) {
				if e["kind"] == "raid_entry_refused" {
					refusal, _ = e["reason"].(string)
				}
			}
			entry := delezieModeEntryFixture(t)
			runtime := &gatewayRuntime{raidEntrances: map[uint32]catalog.RaidEntrance{112: entry}}
			waiting := database.WorldPosition{Town: 228, Area: 2, X: 100, Y: 200}
			recruitment := protocol.RaidRecruitment{Create: protocol.RaidCreateRequest{Kind: 14, Title: "hard", Options: [6]byte{0, 0, 0, 7, 1}}, Leader: 9, MemberCount: 1}
			team, err := runtime.raidTeams.create(1, 112, 1, recruitment, waiting)
			if err != nil {
				t.Fatal(err)
			}
			w := &worldSession{role: database.Character{ID: 1, WireID: 9}, channelType: 112, raidWaiting: true, state: database.WorldState{Position: waiting}}
			c := &gameConnection{gatewayRuntime: runtime, worldState: w, channel: 112, bootstrapped: true, event: log, output: newConnectionOutput(server, keys, "test", log), raidOwnerRole: 1, ownedRaidID: team.Recruitment.ID}
			// A previously valid team's requested mode is no longer declared
			// by the bound source. Start must not select normal as a fallback.
			entry.Delezie.ChannelModes[112] = []catalog.DelezieDifficulty{catalog.DelezieNormal}
			body := make([]byte, 8)
			if id == 2089 {
				body = make([]byte, 18)
				binary.LittleEndian.PutUint32(body[14:], 1)
			}
			result := make(chan dispatchAction, 1)
			go func() {
				result <- c.dispatchRaidEntrance(&clientRequest{frame: wire.Frame{Type: 1, ID: id}, plaintext: body, verified: true})
			}()
			frame := readDispatchServerFrame(t, peer)
			payload, err := wire.DecryptPayload(keys, id, frame[wire.ServerHeaderSize:])
			if err != nil || binary.LittleEndian.Uint16(frame[1:3]) != id || len(payload) == 0 || payload[0] != 0 || <-result != dispatchHandled || !strings.Contains(refusal, "does not declare difficulty 1") {
				t.Fatalf("source mode invalidation was ignored: %x %v reason=%s", payload, err, refusal)
			}
			if got, ok := runtime.raidTeams.get(1, 112); !ok || got.Recruitment.ID != team.Recruitment.ID || got.Recruitment.State != team.Recruitment.State || w.state.Position != waiting || !w.raidWaiting || w.activeDungeon != nil {
				t.Fatal("refused start changed team or world")
			}
		})
	}
}
