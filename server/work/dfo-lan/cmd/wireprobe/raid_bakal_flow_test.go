package main

import (
	"dfolan/internal/catalog"
	"dfolan/internal/database"
	"dfolan/internal/game/protocol"
	"dfolan/internal/game/wire"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net"
	"testing"
	"time"
)

func TestBakalRealSoloStartCountdownAndNotificationSequence(t *testing.T) {
	t.Setenv("DFO_RAID_OPEN_EVENTS_PROBE", "1")
	server, peer := net.Pipe()
	defer server.Close()
	defer peer.Close()
	peer.SetDeadline(time.Now().Add(5 * time.Second))
	keys := make([]byte, wire.SessionKeyBytes)
	log := func(e map[string]any) {}
	rules := catalog.RaidEntrance{MemberMax: 12, StartMinimum: 1, PartyMax: 3, Bakal: &catalog.BakalRaidRules{PhaseMax: 1, StartDelay: 3, TimeLimit: 9999, Symbols: map[string]uint32{"[BAKAL ANGER]": 1}, Dungeons: []catalog.RaidPhaseDungeon{{ID: 100003149, State: "open"}}, InitialVariables: map[string]int32{"[MONSTER MAX HP]": 10000}, Monsters: []catalog.RaidMonsterPlacement{{Location: 24, Kind: "bakal"}}, Slots: map[uint32]catalog.BakalRaidSlot{24: {ID: 24, Kind: "bakal", Dungeon: 100003149, Maps: []uint32{77}}, 52: {ID: 52, Kind: "camp", TownArea: 2}}, MonsterDefinitions: map[string]catalog.BakalRaidMonster{"bakal": {Kind: "bakal", ID: 109014482, SpecificMap: -1}}}}
	runtime := &gatewayRuntime{raidEntrances: map[uint32]catalog.RaidEntrance{82: rules}}
	team, err := runtime.raidTeams.create(1, 82, 1, protocol.RaidRecruitment{Create: protocol.RaidCreateRequest{Kind: 8, Title: "test"}, Leader: 9, LeaderName: "owner", MemberCount: 1, LeaderLevel: 115}, database.WorldPosition{})
	if err != nil {
		t.Fatal(err)
	}
	w := &worldSession{role: database.Character{ID: 1, WireID: 9}, channelType: 82, raidWaiting: true}
	c := &gameConnection{gatewayRuntime: runtime, worldState: w, channel: 82, ownedRaidID: team.Recruitment.ID, bootstrapped: true, event: log, output: newConnectionOutput(server, keys, "test", log)}
	result := make(chan dispatchAction, 1)
	vote, _ := hex.DecodeString("98f55f0000000000feffffffff00010000000000000000000000000000000000")
	go func() {
		result <- c.dispatchRaidEntrance(&clientRequest{frame: wire.Frame{Type: 1, ID: 2089}, verified: true, plaintext: vote})
	}()
	for i, id := range []uint16{2343, 2344, 578, 574, 581, 2089} {
		frame := readDispatchServerFrame(t, peer)
		if binary.LittleEndian.Uint16(frame[1:3]) != id || (i == 5 && frame[0] != 1) {
			t.Fatalf("start wire sequence: %x", frame[:3])
		}
		body, e := wire.DecryptPayload(keys, id, frame[wire.ServerHeaderSize:])
		if e != nil {
			t.Fatal(e)
		}
		if id == 574 && (len(body) < 2 || body[0] != 1 || body[1] != 0) {
			t.Fatalf("incorrect preparation state: %x", body)
		}
	}
	if <-result != dispatchHandled || w.bakalOpening == nil {
		t.Fatal("solo preparation not owned")
	}
	// Repeating Start cannot reinitialize the countdown or add members.
	go func() {
		result <- c.dispatchRaidEntrance(&clientRequest{frame: wire.Frame{Type: 1, ID: 2089}, verified: true, plaintext: vote})
	}()
	f := readDispatchServerFrame(t, peer)
	b, e := wire.DecryptPayload(keys, 2089, f[wire.ServerHeaderSize:])
	if e != nil || f[0] != 1 || binary.LittleEndian.Uint16(f[1:3]) != 2089 || b[0] != 0 || <-result != dispatchHandled {
		t.Fatal("repeated solo start accepted")
	}
	team, _ = runtime.raidTeams.get(1, 82)
	if team.Recruitment.MemberCount != 1 || team.Recruitment.MemberPosition != 1 || team.Recruitment.State != 1 {
		t.Fatalf("real solo assignment/state: %+v", team.Recruitment)
	}
	if err = c.tickBakalOpening(w.bakalOpening.ReadyAt().Add(-time.Second)); err != nil || w.bakalOpening.Active() {
		t.Fatal("countdown bypass", err)
	}
	finish := make(chan error, 1)
	go func() { finish <- c.tickBakalOpening(w.bakalOpening.ReadyAt()) }()
	for _, id := range []uint16{574, 2285, 2286, 2288, 572, 584, 570} {
		frame := readDispatchServerFrame(t, peer)
		if frame[0] != 0 || binary.LittleEndian.Uint16(frame[1:3]) != id {
			t.Fatalf("activation wire sequence: %x", frame[:3])
		}
		body, e := wire.DecryptPayload(keys, id, frame[wire.ServerHeaderSize:])
		if e != nil {
			t.Fatal(e)
		}
		if id == 574 && (body[0] != 2 || body[1] != 0) {
			t.Fatalf("Bakal native active state: %x", body)
		}
		if id == 572 && (len(body) < 11 || body[0] != 0 || body[1] != 1 || binary.LittleEndian.Uint32(body[2:6]) != 100003149 || body[6] != 0) {
			t.Fatalf("Bakal portal requires source-open dungeon state zero: %x", body)
		}
		if id == 2286 && (body[0] != 1 || binary.LittleEndian.Uint32(body[13:17]) != 10000) {
			t.Fatalf("source health lost: %x", body)
		}
		if id == 584 && (len(body) < 9 || body[0] != 0 || binary.LittleEndian.Uint32(body[1:5]) != 9999 || binary.LittleEndian.Uint32(body[5:9]) != 0) {
			t.Fatalf("native timer reader needs two integers: %x", body)
		}
	}
	if err = <-finish; err != nil {
		t.Fatal(err)
	}
	if err = c.tickBakalOpening(w.bakalOpening.ReadyAt().Add(time.Second)); err != nil {
		t.Fatal("replayed activation", err)
	}
	team, _ = runtime.raidTeams.get(1, 82)
	if team.Recruitment.State != 2 || team.Recruitment.MemberCount != 1 {
		t.Fatal("active real solo state lost")
	}
	// A failed run must release both registry state and the old opening;
	// otherwise assignment and Start still reject this real waiting-room owner.
	failed := w.bakalOpening
	if _, err = failed.Tick(failed.ReadyAt().Add(10000 * time.Second)); err != nil || failed.Ended() != "fail" {
		t.Fatal("failed run not ended", err)
	}
	if err = c.releaseEndedBakalOpening(); err != nil || w.bakalOpening != nil || w.bakalRules != nil {
		t.Fatal("failed run retained old battle", err)
	}
	go func() {
		result <- c.dispatchRaidEntrance(&clientRequest{frame: wire.Frame{Type: 1, ID: 2089}, verified: true, plaintext: vote})
	}()
	for _, id := range []uint16{2343, 2344, 574, 581, 2089} {
		frame := readDispatchServerFrame(t, peer)
		if binary.LittleEndian.Uint16(frame[1:3]) != id {
			t.Fatalf("failed owner restart did not reach countdown: %x", frame[:3])
		}
	}
	if <-result != dispatchHandled || w.bakalOpening == nil || w.bakalOpening == failed || w.bakalOpening.Active() {
		t.Fatal("restart did not own a fresh countdown")
	}
	runtime.raidTeams.leave(1, team.Recruitment.ID)
	w.bakalOpening, w.bakalRules = nil, nil
	if err = c.tickBakalOpening(time.Now().Add(time.Hour)); err != nil {
		t.Fatal("late tick after leave", err)
	}
}

func TestBakalWaitingRoomCannotEnterUnstartedOrForeignDungeon(t *testing.T) {
	w := &worldSession{channelType: 82, raidWaiting: true, role: database.Character{ID: 1}}
	if _, err := w.dungeonGate(binary.LittleEndian.AppendUint32(binary.LittleEndian.AppendUint32(nil, 100003149), 0)); err == nil {
		t.Fatal("unstarted waiting room admitted battlefield")
	}
	for _, id := range []uint16{658, 2073, 2089} {
		if !observedGameRequest(id) {
			t.Fatalf("native raid request %d may be throttled by body sampling", id)
		}
	}
}

func TestBakalValidationReentryPreservesWeeklyRecord(t *testing.T) {
	now := time.Now()
	raw := json.RawMessage(fmt.Sprintf(`{"bakal_raid_rewards":{"Week":%q,"Clears":1,"Rewards":1}}`, database.IspinsWeekStart(now).Format(time.RFC3339)))
	role := database.Character{State: raw}
	rules := &catalog.BakalRaidRules{WeeklyClearCount: 1, WeeklyRewardCount: 1}
	t.Setenv("DFO_RAID_OPEN_EVENTS_PROBE", "1")
	t.Setenv("DFO_BAKAL_MODE", "")
	if bakalRaidAdmission(role, rules, now) == nil {
		t.Fatal("source weekly entry cap bypassed without diagnostic profile")
	}
	t.Setenv("DFO_BAKAL_MODE", "unlimited")
	if err := bakalRaidAdmission(role, rules, now); err != nil {
		t.Fatal("weekly-cleared validation owner cannot restart", err)
	}
	if string(role.State) != string(raw) || rules.WeeklyRewardCount != 1 {
		t.Fatal("validation modified progress or reward cap")
	}
	role.State = json.RawMessage(`bad`)
	if bakalRaidAdmission(role, rules, now) == nil {
		t.Fatal("unlimited bypassed corrupted progress")
	}
	t.Setenv("DFO_RAID_OPEN_EVENTS_PROBE", "0")
	role.State = raw
	if bakalRaidAdmission(role, rules, now) == nil {
		t.Fatal("unlimited escaped diagnostic environment")
	}
}
