package main

import (
	"dfolan/internal/catalog"
	"dfolan/internal/database"
	"dfolan/internal/dungeon"
	"dfolan/internal/game/protocol"
	"dfolan/internal/raid"
	"encoding/binary"
	"os"
	"testing"
	"time"
)

func TestBakalScriptDungeonStateRetainsNativePhaseAndTrailer(t *testing.T) {
	w := &worldSession{bakalOpening: &raid.BakalOpening{}, bakalRules: &catalog.BakalRaidRules{}}
	plan, err := w.bakalScriptPlan([]raid.BakalEffect{{Op: "dungeon", ID: 100003149, Kind: "clear"}})
	if err != nil || len(plan) != 1 {
		t.Fatal(plan, err)
	}
	p := plan[0]
	if p.ID != 572 || len(p.Payload) != 11 || p.Payload[0] != 0 || p.Payload[1] != 1 || binary.LittleEndian.Uint32(p.Payload[2:]) != 100003149 || p.Payload[6] != 1 || binary.LittleEndian.Uint32(p.Payload[7:]) != 0 {
		t.Fatalf("invalid source state notification %x", p.Payload)
	}
}

func TestBakalNativeHalfHealthStageWarpAndFinalRoute(t *testing.T) {
	path := os.Getenv("DFO_PVF_CORE_TEST_ARCHIVE")
	if path == "" {
		t.Skip("native PVF required")
	}
	a, err := catalog.OpenTestArchiveCached(path, "")
	if err != nil {
		t.Fatal(err)
	}
	dir, err := catalog.ImportChannelDirectory(a)
	if err != nil {
		t.Fatal(err)
	}
	entries, err := catalog.ImportRaidEntrances(a, &dir)
	if err != nil {
		t.Fatal(err)
	}
	entry := entries[82]
	var ids []uint32
	for _, d := range entry.Bakal.Dungeons {
		ids = append(ids, d.ID)
	}
	dc, err := catalog.ImportDungeons(a, ids)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	opening, err := raid.PrepareBakalOpening(entry, []raid.Member{{Actor: 2, Position: 1}}, now.Add(-3*time.Second))
	if err != nil {
		t.Fatal(err)
	}
	if err = opening.Activate(now); err != nil {
		t.Fatal(err)
	}
	for _, slot := range []uint32{12, 26, 40} {
		loc := entry.Bakal.Locations[slot]
		m, _ := opening.InitialMonster(slot)
		if _, err = opening.EnterDungeon(loc.Dungeon, now, true); err != nil {
			t.Fatal(err)
		}
		if err = opening.DefeatMonster(slot, m.ID, now); err != nil {
			t.Fatal(err)
		}
	}
	if _, err = opening.EnterDungeon(100003149, now, true); err != nil {
		t.Fatal(err)
	}
	first, _ := opening.InitialMonster(24)
	w := &worldSession{role: database.Character{ID: 2, WireID: 2, State: []byte(`{"level":115}`)}, level: 115, channelType: 82, raidWaiting: true, soloPartyReady: true, bakalParty: 1, bakalTown: 152, bakalOpening: opening, bakalRules: entry.Bakal, dungeons: &dc, state: database.WorldState{Position: database.WorldPosition{Town: 152, Area: 2}}}
	pc, err := w.bakalPortalCatalog(protocol.DungeonSelection{ID: 100003149}, [2]int32{int32(first.Grid[0]), int32(first.Grid[1])})
	if err != nil {
		t.Fatal(err)
	}
	s, err := dungeon.Select(*pc, protocol.DungeonSelection{ID: 100003149, Party: 65535}, 115, nil)
	if err != nil {
		t.Fatal(err)
	}
	s.Loaded = true
	if _, _, err = s.AddRaidBoss(first); err != nil {
		t.Fatal(err)
	}
	w.activeDungeon = s
	if err = opening.ReportHealth(24, 5000, now); err != nil {
		t.Fatal(err)
	}
	next, plan, err := w.moveDungeonRoomDecoded(protocol.DungeonRoomTransition{Dungeon: 100003149, Position: first.SecondGrid})
	if err != nil || next == nil || [2]byte{next.Room.X, next.Room.Y} != first.SecondGrid || len(plan) == 0 {
		t.Fatal("native live-actor phase warp rejected", err)
	}
	w.activeDungeon = next
	next.Loaded = true
	second, _ := opening.InitialMonster(24)
	if _, _, err = next.AddRaidBoss(second); err != nil {
		t.Fatal(err)
	}
	if err = opening.DefeatMonster(24, second.ID, now); err != nil {
		t.Fatal(err)
	}
	plan, err = w.bakalScriptPlan(opening.DrainEffects())
	if err != nil || w.activeDungeon == nil || w.activeDungeon.Definition.ID != opening.FinalDungeon() {
		t.Fatal("native final route missing", err)
	}
	mapSent := false
	for _, p := range plan {
		mapSent = mapSent || p.ID == 29
		if p.Kind == 1 {
			t.Fatalf("unsolicited command ACK in native final move: %d", p.ID)
		}
	}
	if !mapSent {
		t.Fatal("final room never received native map data")
	}

	phaseNotified := false
	for _, p := range plan {
		phaseNotified = phaseNotified || p.ID == 570 && len(p.Payload) == 9 && binary.LittleEndian.Uint32(p.Payload[1:]) == entry.Bakal.Symbols["[BAKAL PHASE]"] && binary.LittleEndian.Uint32(p.Payload[5:]) == 2
	}
	if !phaseNotified {
		t.Fatal("ending phase absent from native notification plan")
	}
	if opening.SymbolValues()[entry.Bakal.Symbols["[BAKAL PHASE]"]] != 2 {
		t.Fatal("ending phase never enabled native skip UI")
	}
	final := w.activeDungeon
	if final.Room.X == final.Maze.Boss[0] && final.Room.Y == final.Maze.Boss[1] {
		t.Fatal("test must cover first ending room, not maze boss room")
	}
	if _, err = w.finishDungeonLoading(make([]byte, 16)); err != nil {
		t.Fatal(err)
	}
	final.Loaded = true
	if _, err = w.completeDungeon(); err != nil {
		t.Fatal(err)
	}
	if opening.Ended() != "" {
		t.Fatal("living ending dummy prematurely completed raid")
	}
	found := false
	for _, actor := range final.Monsters {
		if actor.Template == final.Definition.SourceBoss && actor.Rank == 3 {
			found = true
			if _, err = final.ConfirmDeath(uint32(actor.Entity), 2, 2); err != nil {
				t.Fatal(err)
			}
		}
	}
	if !found {
		t.Fatalf("source ending dummy missing: hunt=%d actors=%+v", final.Definition.SourceBoss, final.Monsters)
	}
	if final.Completed() {
		t.Fatal("ordinary maze completion unexpectedly accepted first room")
	}
	end, err := w.completeDungeon()
	if err != nil || opening.Ended() != "clear" || w.activeDungeon != nil {
		t.Fatal("confirmed ending dummy failed native clear and camp return", err, opening.Ended())
	}
	ended := false
	for _, p := range end {
		ended = ended || p.Name == "bakal_script_phase_ended"
	}
	if !ended {
		t.Fatal("no end-phase notification")
	}
	if repeat, err := w.bakalConfirmDefeats(); err != nil || len(repeat) != 0 {
		t.Fatal("ending replay duplicated transition", err)
	}

	// Once durable settlement is acknowledged, the serial tick releases the
	// owned team's active state so the next start cannot remain stuck at state 2.
	run, _, ready := opening.SettlementIdentity()
	if !ready {
		t.Fatal("completed ending has no settlement identity")
	}
	w.role.ID = 17
	runtime := &gatewayRuntime{}
	team, err := runtime.raidTeams.create(17, 82, 1, protocol.RaidRecruitment{State: 2}, database.WorldPosition{})
	if err != nil {
		t.Fatal(err)
	}
	c := &gameConnection{gatewayRuntime: runtime, worldState: w, channel: 82, event: func(map[string]any) {}}
	w.bakalSettledRun = "previous-run"
	if err = c.releaseEndedBakalOpening(); err != nil || w.bakalOpening == nil {
		t.Fatal("pending clear discarded before its own durable settlement", err)
	}
	w.bakalSettledRun = run
	if err = c.tickBakalOpening(time.Now()); err != nil {
		t.Fatal(err)
	}
	if team, ok := runtime.raidTeams.get(17, 82); !ok || team.Recruitment.State != 0 {
		t.Fatal("completed raid still owns active registry state")
	}
	if w.bakalOpening != nil || w.bakalRules != nil {
		t.Fatal("settled clear still blocks next start")
	}
	_ = team
}
