package main

import (
	"context"
	"dfolan/internal/catalog"
	"dfolan/internal/character"
	"dfolan/internal/database"
	"dfolan/internal/game/protocol"
	"dfolan/internal/inventory"
	"dfolan/internal/legion"
	"dfolan/internal/loot"
	"dfolan/internal/savecontract"
	"dfolan/internal/workflow"
	"dfolan/internal/world"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"strings"
	"testing"
	"time"
)

func TestBakalDefaultProfileAndNoticeKind(t *testing.T) {
	b, err := os.ReadFile("../../configs/pvf-default.json")
	if err != nil {
		t.Fatal(err)
	}
	var p struct{ Environment map[string]string }
	if err = json.Unmarshal(b, &p); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(","+p.Environment["DFO_PVF_CATALOGS"]+",", ",bakal-raid,") {
		t.Fatal("default profile omits native raid")
	}
	if got := bakalOutgoing([]legion.BakalFrame{{ID: 13}})[0].Kind; got != 0 {
		t.Fatalf("N13 kind=%d", got)
	}
}

func TestBakalLiveWaitingRoomCreateAndDetails(t *testing.T) {
	c, now := bakalNativeClient(t)
	w := c.worldState
	c.gatewayRuntime = &gatewayRuntime{}
	w.serverID = 1
	w.bakal = nil
	w.bakalRewards = &workflow.BakalRewardService{Rules: w.bakalRules}
	w.state.Position = database.WorldPosition{Town: 152, Area: 1}
	// Actual failed-session 656 vector, 2026-10-06 02:44:30 +08.
	body, _ := hex.DecodeString("08010000003100000007000000000000")
	handled, plan, err := c.handleBakalRequest(656, body, now)
	if err != nil || !handled || w.bakal == nil || w.bakalRecruitment == nil {
		t.Fatalf("source waiting room refused: handled=%v err=%v", handled, err)
	}
	if len(plan) != 2 || plan[0].ID != 578 || plan[0].Kind != 0 || plan[1].ID != 656 || plan[1].Kind != 1 || len(plan[1].Payload) != 5 || plan[1].Payload[0] != 1 {
		t.Fatalf("missing native create responses: %+v", plan)
	}
	id := binary.LittleEndian.Uint32(plan[1].Payload[1:])
	if id != 0x01520001 || id != w.bakalRecruitment.ID {
		t.Fatal("native wire team ID not owned")
	}
	if w.bakalRecruitment.MemberPosition != 0 || w.bakalRecruitment.MemberMax != uint32(w.bakalRules.RaidMemberMax) {
		t.Fatal("initial live leader or source member limit lost")
	}
	if w.bakalRecruitment.MemberArea != uint16(w.state.Position.Area) {
		t.Fatal("member record lost the waiting-room area")
	}
	// Verified flow: the raid is created in the left hall (waitroom, area 1)
	// and the formation is edited in the right camp after walking over. The
	// client start predicate 144CF4C80 compares the member record's area
	// with the raid script's waiting area — the record must carry the live
	// area so the camp-side 661/1353 updates read 2.
	w.state.Position.Area = 2
	w.bakalRecruitment.MemberArea = 1 // reproduce the hall-side stale record
	// Official 01:55:14 manager-work action0 assigns actor482 to position1.
	assignment, _ := hex.DecodeString("00000000e20100000100000000000000")
	binary.LittleEndian.PutUint32(assignment[4:], uint32(w.role.WireID))
	_, assigned, err := c.handleBakalRequest(661, assignment, now)
	if err != nil || len(assigned) != 2 || assigned[0].ID != 578 || binary.LittleEndian.Uint32(assigned[0].Payload[4:]) != 3 || assigned[1].ID != 661 || w.bakalRecruitment.MemberPosition != 1 {
		t.Fatalf("native formation assignment failed: %+v %v", assigned, err)
	}
	memberAreaOffset := 9 + 24 + len(w.bakalRecruitment.LeaderName)
	if binary.LittleEndian.Uint16(assigned[0].Payload[memberAreaOffset:]) != uint16(w.bakalRules.WaitingRoomArea) {
		t.Fatal("native same-area predicate would reject the sole owned member")
	}
	binary.LittleEndian.PutUint32(assignment[4:], uint32(w.role.WireID)+1)
	if _, _, err := c.handleBakalRequest(661, assignment, now); err == nil {
		t.Fatal("foreign actor assigned into owned raid")
	}
	query := binary.LittleEndian.AppendUint32([]byte{1}, id)
	query = append(query, make([]byte, 11)...)
	_, details, err := c.handleBakalRequest(1353, query, now)
	if err != nil || len(details) != 2 || details[0].ID != 1353 || details[0].Payload[0] != 1 || details[0].Payload[1] != 1 || details[1].ID != 1220 {
		t.Fatalf("owned member query failed: %+v %v", details, err)
	}
	query[1]++
	if _, _, err := c.handleBakalRequest(1353, query, now); err == nil {
		t.Fatal("accepted another raid's identity")
	}
	// The actual actor must survive the start burst; never use the actor2
	// historical fixture as runtime member data.
	start, _ := hex.DecodeString("88f25f0000000000feffffffff00010000000000000000000000000000000000")
	_, started, err := c.handleBakalRequest(2089, start, now)
	if err != nil {
		t.Fatal(err)
	}
	for _, packet := range started {
		if packet.ID == 578 && binary.LittleEndian.Uint16(packet.Payload[9:]) != w.role.WireID {
			t.Fatal("start replaced the owned actor with the fixture actor")
		}
	}
	w.bakal = nil
	w.bakalRecruitment = nil
	w.state.Position = database.WorldPosition{Town: 152, Area: 0}
	if _, _, err := c.handleBakalRequest(656, body, now); err == nil {
		t.Fatal("seriagate gate accepted as a create room")
	}
}

func TestBakalLiveKingsroadExperienceAndDeath(t *testing.T) {
	c, now := bakalNativeClient(t)
	w := c.worldState
	if _, err := w.bakalDungeonEntry(100003160, 0, nil, now, false); err != nil {
		t.Fatal(err)
	}
	w.activeDungeon.Loaded = true
	w.role.State = json.RawMessage(`{"level":115,"experience":0}`)
	p := catalog.Progression{MonsterExperience: map[uint16]uint64{}, MonsterRates: []float32{1, 1, 1, 1}, DifficultyRates: []float32{1}}
	for _, monster := range w.activeDungeon.Monsters {
		p.MonsterExperience[uint16(monster.Level)] = 100
	}
	for _, monster := range w.activeDungeon.Monsters {
		// Exercise the actual source DGN before the death plan. This is the
		// exact parser that rejected the user's eight live CMD39 requests.
		gain, err := character.GrowthMonsterGain(p, character.GrowthRules{OutsidePenalty: 1}, w.activeDungeon.Definition, monster, 115, 0)
		if err != nil || gain != 0 {
			t.Fatalf("source kingsroad experience rejected: gain=%d err=%v", gain, err)
		}
		death := make([]byte, 112)
		binary.LittleEndian.PutUint32(death, uint32(monster.Entity))
		binary.LittleEndian.PutUint16(death[4:], w.role.WireID)
		plan, err := w.monsterDeath(death, func(map[string]any) {})
		if err != nil {
			t.Fatalf("live death refused: %v", err)
		}
		found := false
		for _, packet := range plan {
			if packet.ID == 38 && packet.Kind == 0 {
				found = true
			}
		}
		if !found {
			t.Fatal("progression swallowed the death confirmation")
		}
	}
}

func TestBakalLiveUpdateControlHandshake(t *testing.T) {
	c := &gameConnection{worldState: &worldSession{}}
	// Both captured values in the user's 09:13 session and the successful
	// handover session must complete the control handshake before voting.
	for _, vector := range []string{"0100000000000000", "0000000000000000"} {
		body, _ := hex.DecodeString(vector)
		handled, plan, err := c.handleBakalRequest(2121, body, time.Now())
		if err != nil || !handled || len(plan) != 1 || plan[0].Kind != 1 || plan[0].ID != 2121 || hex.EncodeToString(plan[0].Payload) != "01" {
			t.Fatalf("native control handshake missing: vector=%s plan=%+v err=%v", vector, plan, err)
		}
	}
	for _, body := range [][]byte{nil, {2}, {1, 0}, {1, 1, 0, 0, 0, 0, 0, 0}} {
		if _, plan, err := c.handleBakalRequest(2121, body, time.Now()); err == nil || len(plan) != 0 {
			t.Fatalf("invalid control acknowledged: %x", body)
		}
	}
}

func TestBakalFormationBeforeTownMoveSynchronizesMemberArea(t *testing.T) {
	c, now := bakalNativeClient(t)
	w := c.worldState
	c.gatewayRuntime = &gatewayRuntime{}
	w.serverID = 1
	w.bakal = nil
	w.bakalRewards = &workflow.BakalRewardService{Rules: w.bakalRules}
	w.state.Position = database.WorldPosition{Town: 152, Area: 1}
	create, _ := hex.DecodeString("0803000000313131000100070100000000000000000000000000000000000000")
	if _, _, err := c.handleBakalRequest(656, create, now); err != nil {
		t.Fatal(err)
	}
	assign := binary.LittleEndian.AppendUint32(make([]byte, 4), uint32(w.role.WireID))
	assign = binary.LittleEndian.AppendUint32(assign, 1)
	assign = append(assign, make([]byte, 4)...)
	if _, _, err := c.handleBakalRequest(661, assign, now); err != nil {
		t.Fatal(err)
	}
	// User11:48:58 forms in area1, then11:49:01 walks into area2.
	w.state.Position.Area = 2
	var sent []outboundPacket
	send := func(kind byte, id uint16, body []byte) error {
		sent = append(sent, outboundPacket{Kind: kind, ID: id, Payload: body})
		return nil
	}
	if err := w.syncBakalMemberArea(send, func(map[string]any) {}); err != nil {
		t.Fatal(err)
	}
	if len(sent) != 1 || sent[0].Kind != 0 || sent[0].ID != 578 {
		t.Fatal("town move omitted native member update")
	}
	memberAreaOffset := 9 + 24 + len(w.role.Name)
	if binary.LittleEndian.Uint16(sent[0].Payload[memberAreaOffset:]) != 2 || w.bakalRecruitment.MemberPosition != 1 {
		t.Fatal("town move lost current area or formation")
	}
	if err := w.syncBakalMemberArea(send, func(map[string]any) {}); err != nil || len(sent) != 1 {
		t.Fatal("unchanged area replayed member update")
	}
	w.state.Position.Area = 1
	if err := w.syncBakalMemberArea(func(byte, uint16, []byte) error { return errors.New("write failed") }, func(map[string]any) {}); err == nil || w.bakalRecruitment.MemberArea != 2 {
		t.Fatal("failed transmission discarded pending member-area update")
	}
}

func TestBakalRoomMovementAndRepeatedLoad(t *testing.T) {
	c, now := bakalNativeClient(t)
	w := c.worldState
	if _, err := w.bakalDungeonEntry(100003160, 0, nil, now, false); err != nil {
		t.Fatal(err)
	}
	if _, _, err := c.bakalLoadingDone(nil); err != nil {
		t.Fatal(err)
	}
	_, repeat, err := c.bakalLoadingDone(nil)
	if err != nil || len(repeat) != 1 || repeat[0].ID != 2073 {
		t.Fatalf("duplicate initialization: %v %v", repeat, err)
	}
	for _, actor := range w.activeDungeon.Monsters {
		w.activeDungeon.Dead[actor.Entity] = true
	}
	// Actual CMD45 destination from the user's11:19:22 transition.
	next, plan, err := w.moveDungeonRoomDecoded(protocol.DungeonRoomTransition{Position: [2]byte{1, 0}})
	if err != nil {
		t.Fatal(err)
	}
	location, err := w.bakalLocationForRoom(next)
	if err != nil {
		t.Fatal(err)
	}
	found := 0
	for _, packet := range plan {
		if packet.ID == 2285 {
			found++
			if binary.LittleEndian.Uint32(packet.Payload[5:]) != location {
				t.Fatal("marker differs from source destination")
			}
		}
	}
	if found != 1 || w.bakalLocation != location {
		t.Fatal("ordinary move omitted raid marker")
	}
}

func TestBakalLiveNonzeroColumnPortals(t *testing.T) {
	c, now := bakalNativeClient(t)
	for _, vector := range []string{
		"0800000000000000ffffffffff5bedf50500000000010000000100000075020000b70100001e00000005000000000000",
		"0a00000000000000ffffffffff58edf5050000000003000000000000005b060000db0000000a00000019000000000000",
	} {
		body, _ := hex.DecodeString(vector)
		r, err := protocol.DecodeBakalPortal115(body)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := protocol.DecodeLegionPortal115(body); err == nil {
			t.Fatal("Bakal widened unrelated legion gate")
		}
		_, plan, err := c.enterBakalPortal(2062, body, now)
		if err != nil {
			t.Fatalf("live portal still rejected: %v", err)
		}
		s := c.worldState.activeDungeon
		if s.Definition.ID != r.Dungeon || [2]byte{s.Room.X, s.Room.Y} != [2]byte{byte(r.TargetGrid[0]), byte(r.TargetGrid[1])} {
			t.Fatal("portal reset to source starting grid instead of source target")
		}
		for _, packet := range plan {
			if packet.ID == 29 && (packet.Payload[0] != s.Room.X || packet.Payload[1] != s.Room.Y) {
				t.Fatal("N29 differs from validated source room")
			}
		}
		// Coordinates still require a real source room; accepting a decoded
		// envelope must never authorize arbitrary destination data.
		binary.LittleEndian.PutUint32(body[21:], 255)
		if _, _, err := c.enterBakalPortal(2062, body, now); err == nil {
			t.Fatal("outside-maze portal accepted")
		}
	}
}

func TestBakalClearedBossReturnsToDifferentSourceGrid(t *testing.T) {
	c, now := bakalNativeClient(t)
	w := c.worldState
	grid := [2]byte{0, 1}
	if _, err := w.bakalDungeonEntry(100003157, 0, &grid, now, false); err != nil {
		t.Fatal(err)
	}
	if _, _, err := c.bakalLoadingDone(nil); err != nil {
		t.Fatal(err)
	}
	body, _ := hex.DecodeString("00412b8c0100000089909a4501000000000000000001000000ffff6f012c01ffff640064000000000000000000000000")
	if _, _, err := c.bakalScriptWarp(body); err == nil {
		t.Fatal("uncleared boss bypassed native return gate")
	}
	for _, actor := range w.activeDungeon.Monsters {
		w.activeDungeon.Dead[actor.Entity] = true
	}
	_, plan, err := c.bakalScriptWarp(body)
	if err != nil {
		t.Fatalf("actual post-clear2070 refused: %v", err)
	}
	if [2]byte{w.activeDungeon.Room.X, w.activeDungeon.Room.Y} != [2]byte{0, 0} {
		t.Fatal("native return stayed at cleared boss grid")
	}
	found := map[uint16]bool{}
	for _, packet := range plan {
		found[packet.ID] = true
	}
	if !found[2070] || !found[29] || !found[2285] {
		t.Fatal("clear return did not restore map and raid location")
	}
}

func TestBakalFieldBossDoesNotRespawnAfterNativeReturn(t *testing.T) {
	c, now := bakalNativeClient(t)
	w := c.worldState
	grid := [2]byte{0, 1}
	if _, err := w.bakalDungeonEntry(100003157, 0, &grid, now, false); err != nil {
		t.Fatal(err)
	}
	if _, _, err := c.bakalLoadingDone(nil); err != nil {
		t.Fatal(err)
	}
	template := w.bakalRules.Monsters["basilisk"].Template
	var entity uint16
	for _, actor := range w.activeDungeon.Monsters {
		if actor.Template == template {
			entity = actor.Entity
		} else {
			w.activeDungeon.Dead[actor.Entity] = true
		}
	}
	if entity == 0 {
		t.Fatal("source field boss was never registered")
	}
	death := make([]byte, 112)
	binary.LittleEndian.PutUint32(death, uint32(entity))
	binary.LittleEndian.PutUint16(death[4:], w.role.WireID)
	if _, err := w.monsterDeath(death, func(map[string]any) {}); err != nil {
		t.Fatal(err)
	}
	if !w.bakal.IsLocationDefeated(23) {
		t.Fatal("confirmed field leader death lost from raid state")
	}
	warp, _ := hex.DecodeString("4030bc890100000089909a4501000000000000000001000000ffff6f012c01ffff640064000000000000000000000000")
	if _, _, err := c.bakalScriptWarp(warp); err != nil {
		t.Fatal(err)
	}
	_, plan, err := c.bakalLoadingDone(nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, packet := range plan {
		if packet.ID == 2194 {
			t.Fatal("native return respawned the killed source boss")
		}
	}
	// A new dungeon.Session within the SAME raid must also remember defeat.
	if _, err := w.bakalDungeonEntry(100003157, 0, &grid, now, false); err != nil {
		t.Fatal(err)
	}
	_, plan, err = c.bakalLoadingDone(nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, packet := range plan {
		if packet.ID == 2194 {
			t.Fatal("portal reentry reset raid defeat state")
		}
	}
	// A genuinely new raid resets the source placements.
	o, err := legion.PrepareBakalOpening(w.bakalRules, false)
	if err != nil {
		t.Fatal(err)
	}
	o.Start(w.role.Name, now)
	o.Tick(now.Add(time.Duration(w.bakalRules.StartDelaySecs) * time.Second))
	w.bakal = o
	if _, err := w.bakalDungeonEntry(100003157, 0, &grid, now.Add(4*time.Second), false); err != nil {
		t.Fatal(err)
	}
	_, plan, err = c.bakalLoadingDone(nil)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, packet := range plan {
		if packet.ID == 2194 {
			found = true
		}
	}
	if !found {
		t.Fatal("previous raid suppressed fresh source boss")
	}
}

func TestBakalNativeDragonEntryAwakensAndSurvivesRoomSnapshot(t *testing.T) {
	for _, name := range []string{"sparazzi", "skasa", "hisma"} {
		t.Run(name, func(t *testing.T) {
			c, now := bakalNativeClient(t)
			w := c.worldState
			var id uint32
			for _, d := range w.bakalRules.Dungeons {
				if d.Type == name {
					id = d.Index
				}
			}
			key := "[" + strings.ToUpper(name) + " AWAKEN]"
			symbol, ok := w.bakalRules.Symbols[key]
			if !ok {
				t.Fatal("source awakening symbol missing")
			}
			read := func(plan []outboundPacket) int32 {
				for _, p := range plan {
					if p.ID == 570 && len(p.Payload) == 9 && binary.LittleEndian.Uint32(p.Payload[1:]) == symbol {
						return int32(binary.LittleEndian.Uint32(p.Payload[5:]))
					}
				}
				return -1
			}
			if value := read(bakalOutgoing(w.bakal.SymbolsSnapshot())); value != 0 {
				t.Fatalf("initial state=%d", value)
			}
			plan, err := w.bakalDungeonEntry(id, 0, nil, now, false)
			if err != nil {
				t.Fatal(err)
			}
			if value := read(plan); value != 1 {
				t.Fatalf("source ON ENTER not executed: %d", value)
			}
			if value := read(bakalOutgoing(w.bakal.SymbolsSnapshot())); value != 1 {
				t.Fatal("snapshot reset awakened dragon to init0")
			}
			// Parsed source action changes execution without a Go dragon table.
			setBakalSourceAssignment(t, w.bakalRules.NormalPhase, id, key, 2)
			fresh, err := legion.PrepareBakalOpening(w.bakalRules, false)
			if err != nil {
				t.Fatal(err)
			}
			fresh.Start(w.role.Name, now)
			fresh.Tick(now.Add(time.Duration(w.bakalRules.StartDelaySecs) * time.Second))
			fresh.EnterDungeon(id, 12, now.Add(4*time.Second))
			if value := read(bakalOutgoing(fresh.SymbolsSnapshot())); value != 2 {
				t.Fatal("parsed source SET mutation ignored")
			}
			fresh.LoadingDone(id)
			if _, err := fresh.DefeatMonster(id, 12, now.Add(5*time.Second)); err != nil {
				t.Fatal(err)
			}
			if value := read(bakalOutgoing(fresh.SymbolsSnapshot())); value != 0 {
				t.Fatal("cleared dragon retained awake flag")
			}
			// HP0 fails the native entry condition even when awake still0.
			w.bakalRules.NormalPhase.InitialSymbols["["+strings.ToUpper(name)+" HP]"] = 0
			setBakalSourceAssignment(t, w.bakalRules.NormalPhase, 0, "["+strings.ToUpper(name)+" HP]", 0)
			empty, err := legion.PrepareBakalOpening(w.bakalRules, false)
			if err != nil {
				t.Fatal(err)
			}
			empty.Start(w.role.Name, now)
			empty.Tick(now.Add(time.Duration(w.bakalRules.StartDelaySecs) * time.Second))
			empty.EnterDungeon(id, 12, now.Add(4*time.Second))
			if value := read(bakalOutgoing(empty.SymbolsSnapshot())); value != 0 {
				t.Fatal("zero HP dragon awakened")
			}
		})
	}
}

func TestBakalRetreatUIBypassesOnlyRaidSettlementGate(t *testing.T) {
	c, now := bakalNativeClient(t)
	w := c.worldState
	if _, err := w.bakalDungeonEntry(100003160, 0, nil, now, false); err != nil {
		t.Fatal(err)
	}
	if _, _, err := c.bakalLoadingDone(nil); err != nil {
		t.Fatal(err)
	}
	w.service = &world.Service{}
	w.state.Position = database.WorldPosition{Town: 152, Area: 2, X: 350, Y: 246}
	body, _ := hex.DecodeString("01020100000000000000000000000000")
	handled, plan, err := c.handleBakalRequest(72, body, now)
	if err != nil || !handled {
		t.Fatalf("live retreat routed into cards: %v", err)
	}
	if w.activeDungeon != nil || w.bakalCurrent != 0 || w.bakal.Stage() != legion.BakalOpeningActive {
		t.Fatal("retreat ended raid or retained combat session")
	}
	have := map[uint16]bool{}
	for _, packet := range plan {
		have[packet.ID] = true
		if packet.ID == 72 && hex.EncodeToString(packet.Payload) != "010102" {
			t.Fatal("native retreat ACK lost state/option")
		}
		if packet.ID == 42 {
			t.Fatal("unsolicited ordinary leave ACK in native retreat")
		}
	}
	if !have[72] || !have[23] || !have[24] || !have[2285] {
		t.Fatalf("retreat missing town/camp restoration: %v", have)
	}
	if _, _, err := c.handleBakalRequest(72, body, now); err == nil {
		t.Fatal("retreat accepted without owned loaded combat")
	}
	w.bakal = nil
	if handled, _, _ := c.handleBakalRequest(72, body, now); handled {
		t.Fatal("ordinary settlement exit swallowed by raid handler")
	}
}

func TestBakalNativeClearedRoomReturn(t *testing.T) {
	c, now := bakalNativeClient(t)
	w := c.worldState
	grid := bakalNativeBossGrid(t, w, "blona")
	if _, err := w.bakalDungeonEntry(100003154, 0, &grid, now, false); err != nil {
		t.Fatal(err)
	}
	if _, _, err := c.bakalLoadingDone(nil); err != nil {
		t.Fatal(err)
	}
	body, _ := hex.DecodeString("00c14f8c0100000089909a4501000000000000000001000000ffffbc02fa00ffff640064000000000000000000000000")
	if _, _, err := c.bakalScriptWarp(body); err == nil {
		t.Fatal("live boss bypassed return gate")
	}
	for _, actor := range w.activeDungeon.Monsters {
		w.activeDungeon.Dead[actor.Entity] = true
	}
	_, plan, err := c.bakalScriptWarp(body)
	if err != nil {
		t.Fatalf("same-grid native return rejected: %v", err)
	}
	have := map[uint16]int{}
	for _, packet := range plan {
		have[packet.ID]++
	}
	if have[2070] != 1 || have[2285] != 1 || have[29] != 1 {
		t.Fatalf("native return missing: %v", have)
	}
}

func bakalNativeClient(t *testing.T) (*gameConnection, time.Time) {
	t.Helper()
	rules, err := catalog.ImportBakalRaid(catalog.OpenNativeArchive(t))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { rules.DungeonCatalog.CloseMapSource() })
	now := time.Date(2026, 10, 6, 0, 0, 0, 0, time.UTC)
	o, err := legion.PrepareBakalOpening(rules, false)
	if err != nil {
		t.Fatal(err)
	}
	o.Start("Lansmt", now)
	o.Tick(now.Add(time.Duration(rules.StartDelaySecs) * time.Second))
	w := &worldSession{bakal: o, bakalRun: "review-run", bakalRules: rules, dungeons: rules.DungeonCatalog, level: 115, channelType: 82, role: database.Character{ID: 2, AccountID: 1, WireID: 1, Name: "Lansmt", ConfigVersion: savecontract.Identity(), State: json.RawMessage(`{"level":115}`)}}
	return &gameConnection{worldState: w, event: func(map[string]any) {}}, now.Add(4 * time.Second)
}

func TestBakalNativeDirectMoveAndOwnedDeath(t *testing.T) {
	c, now := bakalNativeClient(t)
	w := c.worldState
	body, _ := hex.DecodeString("2e20d46cfc7f0000d8f7198a0158edf50500000000000000000000000003000000fc0000006400000064000000000000")
	handled, packets, err := c.handleBakalRequest(2062, body, now)
	if err != nil || !handled {
		t.Fatalf("native direct move: handled=%v err=%v", handled, err)
	}
	have := map[uint16]bool{}
	for _, p := range packets {
		have[p.ID] = true
	}
	if !have[28] || !have[29] || !have[2281] || w.activeDungeon == nil || w.bakalCurrent != 100003160 || w.bakalLocation != 22 {
		t.Fatalf("incomplete owned entry: ids=%v current=%d location=%d", have, w.bakalCurrent, w.bakalLocation)
	}
	if _, _, err := c.bakalLoadingDone(nil); err != nil {
		t.Fatal(err)
	}
	if !w.activeDungeon.Loaded {
		t.Fatal("native 2073 did not load owned session")
	}
	// Real report field +21 is a map ID, not a monster template.
	report := make([]byte, 184)
	binary.LittleEndian.PutUint32(report[17:], w.bakalCurrent)
	binary.LittleEndian.PutUint32(report[21:], w.activeDungeon.Room.Map)
	if _, _, err := c.bakalBattleReport(report, now); err != nil {
		t.Fatal(err)
	}
	if w.bakal.IsCleared(w.bakalCurrent) {
		t.Fatal("damage report cleared a dungeon")
	}
	for _, d := range w.bakalRules.Dungeons {
		if d.Type == "sparazzi" {
			grid := bakalNativeBossGrid(t, w, "sparazzi")
			if _, err := w.bakalDungeonEntry(d.Index, 0, &grid, now, false); err != nil {
				t.Fatal(err)
			}
			_, packets, err := c.bakalLoadingDone(nil)
			if err != nil {
				t.Fatal(err)
			}
			spawned := false
			for _, p := range packets {
				if p.ID == 2194 {
					spawned = true
					if binary.LittleEndian.Uint32(p.Payload[7:]) != w.bakalRules.Monsters["sparazzi"].Template || p.Payload[12] != 3 {
						t.Fatal("native dynamic boss lost template/rank")
					}
				}
			}
			if !spawned {
				t.Fatal("source named boss not spawned")
			}
			target := w.bakalRules.Monsters["sparazzi"].Template
			var entity uint16
			for _, m := range w.activeDungeon.Monsters {
				if m.Template == target {
					entity = m.Entity
				} else {
					w.activeDungeon.Dead[m.Entity] = true
				}
			}
			badReport := make([]byte, 184)
			binary.LittleEndian.PutUint32(badReport[17:], d.Index)
			binary.LittleEndian.PutUint32(badReport[21:], w.activeDungeon.Room.Map)
			c.bakalBattleReport(badReport, now)
			if w.bakal.IsCleared(d.Index) {
				t.Fatal("live boss cleared by a report")
			}
			death := make([]byte, 64)
			binary.LittleEndian.PutUint32(death, uint32(entity))
			binary.LittleEndian.PutUint16(death[4:], w.role.WireID)
			if _, err := w.monsterDeath(death, func(map[string]any) {}); err != nil {
				t.Fatal(err)
			}
			if !w.bakal.IsCleared(d.Index) {
				t.Fatal("confirmed native named death did not clear source dungeon")
			}
			break
		}
	}
}

func TestBakalNativeTwoPhasesLoadFinalDungeon(t *testing.T) {
	c, now := bakalNativeClient(t)
	w := c.worldState
	defeat := func(name string) {
		t.Helper()
		target := w.bakalRules.Monsters[name].Template
		if name == "bakal" && w.bakal.SecondPhase() {
			target = w.bakalRules.Monsters[name].SecondTemplate
		}
		var entity uint16
		for _, m := range w.activeDungeon.Monsters {
			if m.Template == target {
				entity = m.Entity
			} else {
				w.activeDungeon.Dead[m.Entity] = true
			}
		}
		if entity == 0 {
			t.Fatalf("%s actor not owned", name)
		}
		body := make([]byte, 64)
		binary.LittleEndian.PutUint32(body, uint32(entity))
		binary.LittleEndian.PutUint16(body[4:], w.role.WireID)
		if _, err := w.monsterDeath(body, func(map[string]any) {}); err != nil {
			t.Fatal(err)
		}
	}
	for _, name := range []string{"sparazzi", "skasa", "hisma", "bakal"} {
		var id uint32
		for _, d := range w.bakalRules.Dungeons {
			if d.Type == name {
				id = d.Index
			}
		}
		grid := bakalNativeBossGrid(t, w, name)
		if _, err := w.bakalDungeonEntry(id, 0, &grid, now, false); err != nil {
			t.Fatal(err)
		}
		if _, _, err := c.bakalLoadingDone(nil); err != nil {
			t.Fatal(err)
		}
		defeat(name)
	}
	if w.bakal.Stage() != legion.BakalOpeningActive {
		t.Fatal("first Bakal actor ended the raid")
	}
	warp, _ := hex.DecodeString("8060aa840100000089909a4501010000000500000001000000ffff7c01c201ffff320032000000000000000000000000")
	_, packets, err := c.bakalScriptWarp(warp)
	if err != nil {
		t.Fatal(err)
	}
	if !w.bakal.SecondPhase() || w.activeDungeon.Room.Y != w.bakalRules.Monsters["bakal"].SecondGrid[1] || len(packets) == 0 || packets[0].ID != 2070 {
		t.Fatal("native cinematic did not authorize second actor")
	}
	if _, _, err := c.bakalLoadingDone(nil); err != nil {
		t.Fatal(err)
	}
	defeat("bakal")
	if w.bakal.Stage() != legion.BakalOpeningFinal || w.activeDungeon == nil || w.activeDungeon.Definition.ID != uint32(w.bakalRules.NormalPhase.FinalClearDungeon) || w.bakalLocation != uint32(w.bakalRules.NormalPhase.SettlementTimer.Sub) {
		t.Fatalf("no final owned dungeon: stage=%v location=%d dungeon=%v", w.bakal.Stage(), w.bakalLocation, w.activeDungeon)
	}
	if _, _, err := c.bakalLoadingDone(nil); err != nil {
		t.Fatal(err)
	}
}

type bakalLedgerFake struct {
	role     database.Character
	receipts map[string]json.RawMessage
	failKey  string
}

func (s *bakalLedgerFake) CommitCharacterEvent(_ context.Context, _, _ int64, _, key, _ string, apply func(database.Character) (json.RawMessage, json.RawMessage, error)) (database.Character, bool, error) {
	if key == s.failKey {
		return s.role, false, errors.New("injected unavailable store")
	}
	if s.receipts == nil {
		s.receipts = map[string]json.RawMessage{}
	}
	if _, ok := s.receipts[key]; ok {
		return s.role, false, nil
	}
	state, receipt, err := apply(s.role)
	if err != nil {
		return s.role, false, err
	}
	s.role.State = state
	s.receipts[key] = receipt
	return s.role, true, nil
}
func (s *bakalLedgerFake) CharacterEventReceipt(_ context.Context, _, _ int64, key string) (json.RawMessage, error) {
	return s.receipts[key], nil
}

type bakalStackableBoxes struct{}

func (bakalStackableBoxes) RewardBox(uint32) (loot.RewardBox, bool) { return loot.RewardBox{}, false }
func (bakalStackableBoxes) Container(uint32) bool                   { return false }
func (bakalStackableBoxes) Item(id uint32) bool                     { return id == 10418036 }

func TestBakalSettlementRetryPaysBeforeInventory(t *testing.T) {
	c, now := bakalNativeClient(t)
	w := c.worldState
	// Confirm three source dragon dungeons before the Bakal settlement trigger.
	for _, d := range w.bakalRules.Dungeons {
		if d.Type != "" {
			loc, err := legion.BakalLocationOfDungeon(w.bakalRules, d.Index)
			if err != nil {
				t.Fatal(err)
			}
			w.bakal.EnterDungeon(d.Index, loc, now)
			w.bakal.LoadingDone(d.Index)
			if d.Type != "bakal" {
				w.bakal.DefeatMonster(d.Index, loc, now)
			}
		}
	}
	id := uint32(w.bakalRules.NormalPhase.SettlementDungeon)
	w.bakal.EnterDungeon(id, 24, now)
	w.bakal.LoadingDone(id)
	w.bakal.DefeatMonster(id, 24, now)
	lc, err := catalog.LoadLoot("../../configs/loot.next25.json")
	if err != nil {
		t.Fatal(err)
	}
	if err = lc.SupplementStackables("../../internal/catalog/testdata/item-flow.json"); err != nil {
		t.Fatal(err)
	}
	bags, err := inventory.LoadBagRules("../../configs/inventory.next29.json")
	if err != nil {
		t.Fatal(err)
	}
	gear, err := inventory.LoadEquipmentCatalog("../../configs/equipment.current35.json", lc.Source.Checksum)
	if err != nil {
		t.Fatal(err)
	}
	rules := *w.bakalRules
	rules.Rewards = []catalog.BakalRewardEntry{{Category: "party_card", Template: 10418036, Amount: 1, Weight: 1}, {Category: "squad_item", Template: 10418036, Amount: 1, Weight: 1}}
	store := &bakalLedgerFake{role: w.role, failKey: "bakal-clear:" + w.bakalRun}
	w.loot = &loot.Service{Catalog: lc, BagRules: bags, Equipment: gear, RewardBoxes: bakalStackableBoxes{}}
	w.bakalRewards = &workflow.BakalRewardService{Store: store, Loot: w.loot, Rules: &rules, Content: rules.Source}
	due := now.Add(time.Duration(rules.NormalPhase.SettlementTimer.Secs) * time.Second)
	if err = w.bakalSettleSplice(due); err == nil {
		t.Fatal("failed freeze ignored")
	}
	if frames := w.bakal.Tick(due); w.bakal.Stage() == legion.BakalOpeningEnded || len(frames) != 0 {
		t.Fatal("failed freeze ended raid")
	}
	store.failKey = "bakal-grant:" + w.bakalRun
	if err = w.bakalSettleSplice(due); err == nil {
		t.Fatal("failed grant ignored")
	}
	w.bakal.Tick(due)
	if w.bakal.Stage() == legion.BakalOpeningEnded {
		t.Fatal("failed grant ended raid")
	}
	store.failKey = ""
	if err = w.bakalSettleSplice(due); err != nil {
		t.Fatal(err)
	}
	if err = w.bakalSettleSplice(due); err != nil {
		t.Fatal("durable replay failed", err)
	}
	bag, err := inventory.ReadBag(w.role.State)
	if err != nil || len(bag.Items) != 1 || bag.Items[0].Amount != 2 {
		t.Fatalf("replay payout changed: bag=%+v err=%v", bag.Items, err)
	}
	frames := bakalOutgoing(w.bakal.Tick(due))
	if len(frames) != 4 || frames[1].ID != 13 || frames[1].Kind != 0 {
		t.Fatalf("incorrect settlement burst %+v", frames)
	}
	expected, err := w.loot.Bootstrap(workflow.LootRole(w.role))
	if err != nil || string(frames[1].Payload) != string(expected) {
		t.Fatal("inventory snapshot precedes committed payout")
	}
}

func bakalNativeBossGrid(t *testing.T, w *worldSession, name string) [2]byte {
	t.Helper()
	for _, spawn := range w.bakalRules.NormalPhase.InitMonsters {
		if spawn.Name != name {
			continue
		}
		for _, loc := range w.bakalRules.Locations {
			if loc.Index != spawn.Location {
				continue
			}
			if loc.SpecificX != nil && loc.SpecificY != nil {
				return [2]byte{byte(*loc.SpecificX), byte(*loc.SpecificY)}
			}
			return [2]byte{byte(loc.X), byte(loc.Y)}
		}
	}
	t.Fatalf("missing source placement %s", name)
	return [2]byte{}
}

func TestBakalNativeBossOnlyInSourceArena(t *testing.T) {
	c, now := bakalNativeClient(t)
	w := c.worldState
	bossGrid := bakalNativeBossGrid(t, w, "bakal")
	d := w.bakalRules.DungeonCatalog.Dungeons[uint32(w.bakalRules.NormalPhase.EnterBakalDungeon)]
	for _, room := range d.Mazes[0].Rooms {
		grid := [2]byte{room.X, room.Y}
		if _, err := w.bakalDungeonEntry(d.ID, 0, &grid, now, false); err != nil {
			t.Fatal(err)
		}
		_, plan, err := c.bakalLoadingDone(nil)
		if err != nil {
			t.Fatal(err)
		}
		count := 0
		for _, p := range plan {
			if p.ID == 2194 {
				count++
			}
		}
		want := 0
		if grid == bossGrid {
			want = 1
		}
		if count != want {
			t.Fatalf("grid %v source arena %v: boss rows %d want %d", grid, bossGrid, count, want)
		}
	}
}

func TestBakalFailedRunCanRestartWithOwnedFormation(t *testing.T) {
	c, now := bakalNativeClient(t)
	w := c.worldState
	w.bakalRecruitment = &protocol.RaidRecruitment{ID: 1, Create: protocol.RaidCreateRequest{Kind: 8, Title: "retry"}, Leader: w.role.WireID, LeaderName: w.role.Name, MemberCount: 1, MemberMax: 12, MemberPosition: 1, MemberArea: 2}
	w.bakalName = w.role.Name
	old := w.bakal
	old.Tick(now.Add(time.Duration(w.bakalRules.PhaseTimeOverSecs+1) * time.Second))
	if old.Stage() != legion.BakalOpeningFailed {
		t.Fatal("source timeout did not fail")
	}
	w.bakalFailureNotified = true
	oldRun := w.bakalRun
	start, _ := hex.DecodeString("88f25f0000000000feffffffff00010000000000000000000000000000000000")
	_, plan, err := c.bakalStartRaid(start, now.Add(2*time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if w.bakal == old || w.bakalRun == oldRun || w.bakal.Stage() != legion.BakalOpeningPreparing || w.bakalFailureNotified {
		t.Fatal("retry did not create fresh run")
	}
	if w.bakalRecruitment.MemberPosition != 1 {
		t.Fatal("retry lost formation")
	}
	have := false
	for _, p := range plan {
		if p.ID == 2089 && p.Kind == 1 {
			have = true
		}
	}
	if !have {
		t.Fatal("retry missing native start ACK")
	}
}

func TestBakalNativeMinionPresencePrecedesMapInitialization(t *testing.T) {
	c, now := bakalNativeClient(t)
	w := c.worldState
	a := catalog.OpenNativeArchive(t)
	action, err := catalog.ReadScript(a, "contents/2022/bakalraid/monster/normal/drake/action/incoming_ready.act")
	if err != nil {
		t.Fatal(err)
	}
	var checks []uint32
	for i, token := range action.Cells {
		if token.Text == "[CHECK RAID SYMBOL]" && i+1 < len(action.Cells) {
			checks = append(checks, uint32(action.Cells[i+1].Value))
		}
	}
	if len(checks) == 0 {
		t.Fatal("source incoming action has no presence guard")
	}
	symbol := w.bakalRules.Symbols["[IS EXIST BASILISK]"]
	matches := false
	for _, v := range checks {
		matches = matches || v == symbol
	}
	if !matches {
		t.Fatal("source minion no longer checks hatchery presence")
	}
	plan, err := w.bakalDungeonEntry(100003160, 0, nil, now, false)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, p := range plan {
		if p.ID == 570 && len(p.Payload) == 9 && binary.LittleEndian.Uint32(p.Payload[1:]) == symbol {
			if binary.LittleEndian.Uint32(p.Payload[5:]) != 1 {
				t.Fatal("live hatchery causes native incoming DESTROY")
			}
			found = true
		}
		if p.ID == 29 {
			if !found {
				t.Fatal("presence was not published before small monster initialization")
			}
			break
		}
	}
	if !found {
		t.Fatal("missing native presence update")
	}
	// Confirm the source actor, then keep the absence across a room snapshot.
	grid := bakalNativeBossGrid(t, w, "basilisk")
	if _, err := w.bakalDungeonEntry(100003157, 0, &grid, now, false); err != nil {
		t.Fatal(err)
	}
	if _, _, err := c.bakalLoadingDone(nil); err != nil {
		t.Fatal(err)
	}
	template := w.bakalRules.Monsters["basilisk"].Template
	var boss uint16
	for _, m := range w.activeDungeon.Monsters {
		if m.Template == template {
			boss = m.Entity
		}
	}
	// Keep a live ordinary actor: the source presence update belongs to the
	// confirmed leader, not the later ordinary-room cleanup sequence.
	w.activeDungeon.Monsters = append(w.activeDungeon.Monsters, protocol.DungeonMonster{Entity: 60000, Template: 109014490, Level: 140, Team: 100})
	death := make([]byte, 64)
	binary.LittleEndian.PutUint32(death, uint32(boss))
	binary.LittleEndian.PutUint16(death[4:], w.role.WireID)
	killed, err := w.monsterDeath(death, func(map[string]any) {})
	if err != nil {
		t.Fatal(err)
	}
	verify := func(plan []outboundPacket) {
		t.Helper()
		seen := false
		for _, p := range plan {
			if p.ID == 570 && len(p.Payload) == 9 && binary.LittleEndian.Uint32(p.Payload[1:]) == symbol {
				seen = true
				if binary.LittleEndian.Uint32(p.Payload[5:]) != 0 {
					t.Fatal("defeated hatchery presence restored")
				}
			}
		}
		if !seen {
			t.Fatal("missing source absence update")
		}
	}
	verify(killed)
	verify(bakalOutgoing(w.bakal.SymbolsSnapshot()))
	// No hardcoded symbol IDs: output follows a changed source symbol index.
	delete(w.bakalRules.Symbols, "[IS EXIST BASILISK]")
	w.bakalRules.Symbols["[IS EXIST BASILISK]"] = 9001
	symbol = 9001
	verify(bakalOutgoing(w.bakal.SymbolsSnapshot()))
}

func TestBakalNativeInitialReservationsDriveMinionPresence(t *testing.T) {
	c, now := bakalNativeClient(t)
	w := c.worldState
	// Retain source rules but change a delay and reduce its source placement
	// candidates to one; the executor must follow both rather than a fixed table.
	if len(w.bakalRules.NormalPhase.ReserveMonsters) == 0 {
		t.Fatal("missing source initial reservations")
	}
	reservation := w.bakalRules.NormalPhase.ReserveMonsters[0]
	w.bakalRules.NormalPhase.ReserveMonsters[0].Location = 19
	for i := range w.bakalRules.NormalPhase.Events {
		for j := range w.bakalRules.NormalPhase.Events[i].Behavior {
			ins := &w.bakalRules.NormalPhase.Events[i].Behavior[j]
			if ins.Op == "[RESERVE CREATE MONSTER]" && ins.Args[1].Text == reservation.Name && ins.Args[0].Value == int32(reservation.Location) {
				ins.Args[0].Value = 19
			}
		}
	}
	var target int
	for i, loc := range w.bakalRules.Locations {
		var keep []string
		for _, kind := range loc.Creatable {
			if kind != reservation.Name {
				keep = append(keep, kind)
				continue
			}
			if target == 0 {
				target = loc.Index
				keep = append(keep, kind)
			}
		}
		w.bakalRules.Locations[i].Creatable = keep
	}
	if target == 0 {
		t.Fatal("no source candidate")
	}
	fresh, err := legion.PrepareBakalOpening(w.bakalRules, false)
	if err != nil {
		t.Fatal(err)
	}
	fresh.Start(w.role.Name, now)
	ready := now.Add(time.Duration(w.bakalRules.StartDelaySecs) * time.Second)
	fresh.Tick(ready)
	value := func() uint32 {
		t.Helper()
		id := w.bakalRules.Symbols["[IS EXIST "+strings.ToUpper(reservation.Name)+"]"]
		for _, f := range fresh.SymbolsSnapshot() {
			if f.ID == 570 && binary.LittleEndian.Uint32(f.Body[1:]) == id {
				return binary.LittleEndian.Uint32(f.Body[5:])
			}
		}
		t.Fatal("no presence symbol")
		return 0
	}
	fresh.Tick(ready.Add(18 * time.Second))
	if value() != 0 {
		t.Fatal("reservation ignored changed source delay")
	}
	fresh.Tick(ready.Add(19 * time.Second))
	if value() != 1 {
		t.Fatal("reserved field leader never enabled minions")
	}
	exists := false
	for _, p := range fresh.MonsterPlacements() {
		if p.Name == reservation.Name {
			exists = true
			if p.Location != target {
				t.Fatal("reservation ignored source creatable candidates")
			}
		}
	}
	if !exists {
		t.Fatal("no owned reserved placement")
	}
	// Same-time ticks must not emit duplicate source spawns.
	for _, f := range fresh.Tick(ready.Add(19 * time.Second)) {
		if f.Name == "bakal_source_monster_created" {
			t.Fatal("duplicate reservation")
		}
	}
}

func TestBakalNativeReservedLeaderHasOwnedSourceActor(t *testing.T) {
	c, now := bakalNativeClient(t)
	w := c.worldState
	w.bakal.Tick(now.Add(20 * time.Second))
	for _, reservation := range w.bakalRules.NormalPhase.ReserveMonsters {
		var placement catalog.BakalMonsterSpawn
		for _, p := range w.bakal.MonsterPlacements() {
			if p.Name == reservation.Name {
				placement = p
				break
			}
		}
		if placement.Name == "" {
			t.Fatalf("source reservation missing %s", reservation.Name)
		}
		var loc catalog.BakalLocationInfo
		for _, v := range w.bakalRules.Locations {
			if v.Index == placement.Location {
				loc = v
				break
			}
		}
		grid := [2]byte{byte(loc.X), byte(loc.Y)}
		if _, err := w.bakalDungeonEntry(loc.Dungeon, 0, &grid, now.Add(20*time.Second), false); err != nil {
			t.Fatal(err)
		}
		_, plan, err := c.bakalLoadingDone(nil)
		if err != nil {
			t.Fatal(err)
		}
		expected := w.bakalRules.Monsters[reservation.Name].Template
		found := false
		for _, p := range plan {
			if p.ID == 2194 && len(p.Payload) == 34 && binary.LittleEndian.Uint32(p.Payload[7:]) == expected {
				found = true
			}
		}
		if !found {
			t.Fatalf("%s presence without source-owned room actor at %v", reservation.Name, grid)
		}
	}
}

func TestBakalCapturedPortalsEnterLivingSourceArenas(t *testing.T) {
	c, now := bakalNativeClient(t)
	w := c.worldState
	// Actual failing-session2062 vectors, 20261006_154036. The normal0,0
	// target must resolve to the occupied source arena before loading.
	for _, tc := range []struct{ name, hex string }{
		{"basilisk", "2b00000000000000ffffffffff55edf505000000000000000001000000dc000000f20000000a0000000a000000000000"},
		{"blona", "0d00000000000000ffffffffff52edf50500000000000000000000000054040000070100000a00000000000000000000"},
		{"sparazzi", "0100000000000000ffffffffff4fedf505000000000000000000000000ae020000b70000000a0000000a000000000000"},
		{"nympha", "0600000000000000ffffffffff56edf50500000000000000000000000092040000d60000000a0000001e000000000000"},
		{"hisma", "0100000000000000ffffffffff50edf5050000000000000000000000003a0300001e0100000000000014000000000000"},
		{"gerda", "3300000000000000ffffffffff54edf50500000000000000000000000081020000920100001e00000000000000000000"},
		{"skasa", "0100000000000000ffffffffff4eedf50500000000000000000100000027010000230100000a0000000a000000000000"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p, err := hex.DecodeString(tc.hex)
			if err != nil {
				t.Fatal(err)
			}
			expected := bakalNativeBossGrid(t, w, tc.name)
			if _, _, err := c.enterBakalPortal(2062, p, now); err != nil {
				t.Fatal(err)
			}
			s := w.activeDungeon
			if [2]byte{s.Room.X, s.Room.Y} != expected {
				t.Fatalf("normal service room selected: got%v want source arena%v", [2]byte{s.Room.X, s.Room.Y}, expected)
			}
			_, plan, err := c.bakalLoadingDone(nil)
			if err != nil {
				t.Fatal(err)
			}
			target := w.bakalRules.Monsters[tc.name].Template
			found := false
			for _, v := range plan {
				if v.ID == 2194 && len(v.Payload) == 34 && binary.LittleEndian.Uint32(v.Payload[7:]) == target {
					found = true
				}
			}
			if !found {
				t.Fatalf("%s source leader never created", tc.name)
			}
		})
	}
}

func TestBakalDefeatedPortalKeepsNormalRoom(t *testing.T) {
	c, now := bakalNativeClient(t)
	w := c.worldState
	p, _ := hex.DecodeString("2b00000000000000ffffffffff55edf505000000000000000001000000dc000000f20000000a0000000a000000000000")
	if _, _, err := c.enterBakalPortal(2062, p, now); err != nil {
		t.Fatal(err)
	}
	if _, _, err := c.bakalLoadingDone(nil); err != nil {
		t.Fatal(err)
	}
	if _, err := w.bakal.DefeatMonster(w.bakalCurrent, w.bakalLocation, now); err != nil {
		t.Fatal(err)
	}
	binary.LittleEndian.PutUint32(p[25:], 0)
	if _, _, err := c.enterBakalPortal(2062, p, now); err != nil {
		t.Fatal(err)
	}
	if [2]byte{w.activeDungeon.Room.X, w.activeDungeon.Room.Y} != [2]byte{0, 0} {
		t.Fatal("defeated portal redirected to arena")
	}
	_, plan, err := c.bakalLoadingDone(nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, packet := range plan {
		if packet.ID == 2194 {
			t.Fatal("defeated leader recreated")
		}
	}
}

func TestBakalOfficialFieldLeaderTypeAndPresencePairs(t *testing.T) {
	// Paired updates from official20261005-015111: N2286 creation immediately
	// followed by its source N570 presence. These values are protocol enums.
	for _, tc := range []struct {
		name          string
		kind, symbol  uint32
		row, presence string
	}{
		{"swan", 9, 208, "0109000000020000000000000010270000070819", "01d000000001000000"},
		{"steich", 10, 209, "010a0000001f0000000000000010270000050919", "01d100000001000000"},
		{"eclair", 11, 210, "010b000000230000000000000010270000050a19", "01d200000001000000"},
	} {
		row, err := hex.DecodeString(tc.row)
		if err != nil {
			t.Fatal(err)
		}
		symbol, err := hex.DecodeString(tc.presence)
		if err != nil {
			t.Fatal(err)
		}
		if got := legion.BakalMonsterType(tc.name); got != tc.kind || got != binary.LittleEndian.Uint32(row[1:]) {
			t.Fatalf("%s native type=%d want%d", tc.name, got, tc.kind)
		}
		if binary.LittleEndian.Uint32(symbol[1:]) != tc.symbol {
			t.Fatal("bad official presence fixture")
		}
	}
}

func setBakalSourceAssignment(t *testing.T, p *catalog.BakalPhaseRules, dungeon uint32, symbol string, value int32) {
	t.Helper()
	changed := false
	for i := range p.Events {
		r := &p.Events[i]
		match := false
		for _, ins := range r.Trigger {
			if dungeon == 0 && ins.Op == "[IF]" && ins.Args[0].Text == "[1PHASE INIT]" {
				match = true
			}
			if dungeon != 0 && ins.Op == "[ON ENTER DUNGEON]" && uint32(ins.Args[0].Value) == dungeon {
				match = true
			}
		}
		if !match {
			continue
		}
		for j := range r.Behavior {
			ins := &r.Behavior[j]
			if ins.Op == "[SET]" && ins.Args[0].Text == symbol {
				ins.Args[len(ins.Args)-1].Value = value
				changed = true
			}
		}
	}
	if !changed {
		t.Fatalf("missing source assignment %s", symbol)
	}
}

func TestBakalRuntimeCommanderEnvelopeAndMemberOwnership(t *testing.T) {
	c, now := bakalNativeClient(t)
	w := c.worldState
	grid := bakalNativeBossGrid(t, w, "basilisk")
	if _, err := w.bakalDungeonEntry(100003157, 0, &grid, now, false); err != nil {
		t.Fatal(err)
	}
	if _, _, err := c.bakalLoadingDone(nil); err != nil {
		t.Fatal(err)
	}
	// Current14254C960 static sender: thirteen opaque bytes + chosen i32.
	// No live2072 sample is present; this validates the recovered layout only.
	body := make([]byte, 17)
	binary.LittleEndian.PutUint32(body[13:], 3)
	if _, _, err := c.handleBakalRequest(2072, body, now); err == nil {
		t.Fatal("buff use without owned formation")
	}
	w.bakalRecruitment = &protocol.RaidRecruitment{MemberPosition: 5}
	handled, plan, err := c.handleBakalRequest(2072, body, now)
	if err != nil || !handled {
		t.Fatal(err)
	}
	found := false
	for _, p := range plan {
		if p.ID == 2288 {
			found = true
			if binary.LittleEndian.Uint32(p.Payload[5:]) != 3 || binary.LittleEndian.Uint32(p.Payload[9:]) != 4 {
				t.Fatal("buff targeted wrong native member")
			}
		}
	}
	if !found {
		t.Fatal("no commander notification")
	}
	if _, _, err := c.handleBakalRequest(2072, body, now); err == nil {
		t.Fatal("duplicate command spent second charge")
	}
	binary.LittleEndian.PutUint32(body[13:], 25)
	if _, _, err := c.handleBakalRequest(2072, body, now); err == nil {
		t.Fatal("unused kind accepted")
	}
}
