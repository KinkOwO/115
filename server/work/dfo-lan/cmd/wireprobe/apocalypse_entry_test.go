package main

import (
	"bytes"
	"dfolan/internal/catalog"
	"dfolan/internal/database"
	"dfolan/internal/dungeon"
	"dfolan/internal/game/wire"
	"dfolan/internal/legion"
	"dfolan/internal/testfixture"
	"dfolan/internal/world"
	"encoding/binary"
	"net"
	"os"
	"testing"
	"time"
)

func navigationEntryFixture(t *testing.T) (*legionSession, *worldSession) {
	t.Helper()
	s, w, body := apocalypsePartyFixture(t)
	dc, err := catalog.LoadDungeons(testfixture.DungeonPath(t, "dungeons.generated.json"))
	if err != nil {
		t.Fatal(err)
	}
	contents, err := catalog.LoadLegionContents("../../configs/legion-contents.generated.json", dc.Source.Checksum)
	if err != nil {
		t.Fatal(err)
	}
	s.contents = contents
	donor := dc.Dungeons[3]
	content := contents.Contents["Apocalypse"]
	for _, row := range content.Dungeons {
		d := donor
		d.ID, d.MinimumLevel, d.Mazes = row.Dungeon, uint32(content.LimitLevel), nil
		for _, maze := range donor.Mazes {
			maze.Quest = 0
			d.Mazes = append(d.Mazes, maze)
		}
		dc.Dungeons[d.ID] = d
	}
	w.dungeons = &dc
	w.state.Position = database.WorldPosition{Town: content.Waiting.Town, Area: content.Waiting.Area, X: content.Waiting.X, Y: content.Waiting.Y}
	w.service = &world.Service{Catalog: catalog.WorldCatalog{Source: dc.Source, Areas: map[string]catalog.WorldArea{catalog.AreaKey(content.Waiting.Town, content.Waiting.Area): {Walkable: [][4]int32{{0, 0, 2000, 2000}}}}}}
	w.soloPartyBootstrap = true // regression: must not replace the actual mode38 party
	s.clock, err = legion.NewApocalypseClock(s.catalog)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.handleParty(w, body, 12); err != nil {
		t.Fatal(err)
	}
	start, err := s.handle(w, startPayload(uint32(legion.ApocalypseContent)), legion.CmdStart)
	if err != nil {
		t.Fatal(err)
	}
	s.adoptProjection(start.Packets)
	operation, err := s.handle(w, apocalypseOperationBody(2, 1), legion.CmdOperationSelect)
	if err != nil {
		t.Fatal(err)
	}
	s.adoptProjection(operation.Packets)
	return s, w
}

func navigationRequest() []byte {
	p := make([]byte, legion.EnvelopeSize+8)
	binary.LittleEndian.PutUint32(p[13:], legion.OperationChannelCode)
	return p
}

func TestApocalypseNavigationUsesSourceAndPreservesParty(t *testing.T) {
	s, w := navigationEntryFixture(t)
	owner, choice, before := s.party, s.plan, w.state
	result, err := s.handle(w, navigationRequest(), legion.CmdEnterDungeon)
	if err != nil {
		t.Fatal(err)
	}
	if result.Navigation == nil || w.activeDungeon != nil || w.apocalypseNavigation != nil || s.party != owner || s.plan != choice || w.state != before {
		t.Fatal("entry prepared by mutating live ownership")
	}
	var sequence []uint16
	for _, p := range result.Packets {
		if p.ID == 9 || p.ID == 16 || p.ID == 2568 {
			t.Fatalf("unexpected ordinary party/ACK or unbound FX toggle: %d", p.ID)
		}
		if p.ID == 3 || p.ID == 27 || p.ID == 28 || p.ID == 29 {
			sequence = append(sequence, p.ID)
		}
		if p.ID == 28 && binary.LittleEndian.Uint32(p.Payload) != s.contents.Contents["Apocalypse"].Dungeons[0].Dungeon {
			t.Fatal("navigation destination ignored the source")
		}
	}
	if len(sequence) != 4 || sequence[0] != 3 || sequence[1] != 27 || sequence[2] != 28 || sequence[3] != 29 || !bytes.Equal(result.Packets[0].Payload, legion.EnterDungeonAck()) {
		t.Fatal("native loading dependencies or command-specific ACK lost", sequence)
	}
	// A source edit changes the actual selected dungeon, not just its label.
	content := s.contents.Contents["Apocalypse"]
	donor := w.dungeons.Dungeons[content.Dungeons[0].Dungeon]
	donor.ID++
	w.dungeons.Dungeons[donor.ID] = donor
	content.Dungeons[0].Dungeon = donor.ID
	s.contents.Contents["Apocalypse"] = content
	changed, err := s.handle(w, navigationRequest(), legion.CmdEnterDungeon)
	if err != nil || changed.Navigation.Definition.ID != donor.ID {
		t.Fatal("source destination change did not propagate", err)
	}
	for _, mutate := range []func(){func() { s.party = nil }, func() { s.party = owner; w.service.Catalog.Source.Checksum = "other" }} {
		mutate()
		rejected, err := s.handle(w, navigationRequest(), legion.CmdEnterDungeon)
		if err == nil || len(rejected.Packets) != 0 || rejected.Navigation != nil || w.activeDungeon != nil {
			t.Fatal("invalid admission acknowledged", err)
		}
	}
}

func TestApocalypseNavigationCannotUseOrdinaryClearOrPortal(t *testing.T) {
	w := &worldSession{activeDungeon: &dungeon.Session{}}
	w.apocalypseNavigation = w.activeDungeon
	if packets, err := w.completeDungeon(); err != nil || len(packets) != 0 {
		t.Fatal("navigation acknowledged an ordinary clear", err)
	}
	if packets, err := w.dungeonResult(nil); err == nil || len(packets) != 0 {
		t.Fatal("navigation paid ordinary rewards")
	}
	if next, packets, err := w.directMoveDungeon(make([]byte, 48)); err == nil || next != nil || len(packets) != 0 {
		t.Fatal("unowned stage portal accepted")
	}
}

func TestApocalypseNavigationNativePVF(t *testing.T) {
	path := os.Getenv("DFO_PVF_CORE_TEST_ARCHIVE")
	if path == "" {
		t.Skip("native PVF archive required")
	}
	a, err := catalog.OpenTestArchiveCached(path, os.Getenv("DFO_PVF_CORE_TEST_SHA256"))
	if err != nil {
		t.Fatal(err)
	}
	s, w := navigationEntryFixture(t)
	s.contents, err = catalog.ImportLegionContents(a)
	if err != nil {
		t.Fatal(err)
	}
	s.catalog, err = catalog.ImportApocalypse(a)
	if err != nil {
		t.Fatal(err)
	}
	s.clock, err = legion.NewApocalypseClock(s.catalog)
	if err != nil {
		t.Fatal(err)
	}
	var ids []uint32
	for _, row := range s.contents.Contents["Apocalypse"].Dungeons {
		ids = append(ids, row.Dungeon)
	}
	dc, err := catalog.ImportRuntimeDungeons(a, ids)
	if err != nil {
		t.Fatal(err)
	}
	w.dungeons = &dc
	wc, err := catalog.ImportWorldRuntime(a)
	if err != nil {
		t.Fatal(err)
	}
	w.service.Catalog = wc
	w.state.Position = database.WorldPosition{Town: s.contents.Contents["Apocalypse"].Waiting.Town, Area: s.contents.Contents["Apocalypse"].Waiting.Area, X: s.contents.Contents["Apocalypse"].Waiting.X, Y: s.contents.Contents["Apocalypse"].Waiting.Y}
	s.plan, err = legion.PlanForChoice(s.catalog, s.clock, 0)
	if err != nil {
		t.Fatal(err)
	}
	s.session.Operation = s.plan.OperationID
	s.projection[4] = byte(s.plan.OperationID - 1)
	result, err := s.handle(w, navigationRequest(), legion.CmdEnterDungeon)
	if err != nil || result.Navigation == nil {
		t.Fatal("native navigation preparation failed", err)
	}
	n := result.Navigation
	if n.Definition.ID != ids[0] || len(n.Monsters) == 0 || !n.Definition.NoFatigue {
		t.Fatal("source map/actors/admission lost")
	}
	t.Logf("source=%s operation=%d dungeon=%d map=%d actors=%d packets=%d scene_acceptance=false", dc.Source.Checksum, s.plan.OperationID, n.Definition.ID, n.Room.Map, len(n.Monsters), len(result.Packets))
	w.activeDungeon, w.apocalypseNavigation = n, n
	w.apocalypseRun = result.Route
	s.adoptProjection(result.Packets)
	n.Loaded = true
	role, err := s.handle(w, rolePayload(1, legion.OperationChannelCode), legion.CmdRoleSelect)
	if err != nil || len(role.Packets) != 2 || binary.LittleEndian.Uint32(role.Packets[0].Payload[109:]) != 1 {
		t.Fatal("native loaded navigation did not authorize its actual scout seat", err)
	}
	if denied, err := s.handle(w, rolePayload(2, legion.OperationChannelCode), legion.CmdRoleSelect); err == nil || len(denied.Packets) != 0 {
		t.Fatal("native solo guardian bypassed the actual source quota")
	}
	s.adoptProjection(role.Packets)
	retreat, leave, err := s.prepareApocalypseRetreat(w, apocalypseRetreatBody())
	if err != nil || !leave || len(retreat) < 5 || retreat[3].ID != 23 {
		t.Fatal("native navigation retreat preparation", err)
	}
	if len(n.Monsters) != 1 || n.Monsters[0].Rank != 3 {
		t.Fatal("source navigation gate shape changed")
	}
	m := n.Monsters[0]
	n.Dead[m.Entity] = true
	w.deathSent = map[uint16]bool{m.Entity: true}
	check := make([]byte, 16)
	binary.LittleEndian.PutUint16(check, w.role.WireID)
	binary.LittleEndian.PutUint16(check[2:], m.Entity)
	advanced, err := s.prepareNavigationAdvance(w, check)
	if err != nil {
		t.Fatal("native source gate progression", err)
	}
	w.apocalypseRun = advanced.Route
	s.adoptProjection(advanced.Packets)
	d := dc.Dungeons[advanced.Route.Entry.Destinations[advanced.Route.First]]
	combat, err := s.prepareFirstCombat(w, firstPortal(advanced.Route, d.Mazes[0].Start))
	if err != nil || combat.Combat == nil || len(combat.Combat.Monsters) == 0 {
		t.Fatal("native source first combat preparation", err)
	}
	t.Logf("first-combat destination=%d dungeon=%d map=%d grid=%v actors=%d live_acceptance=false", advanced.Route.First, combat.Combat.Definition.ID, combat.Combat.Room.Map, combat.Combat.Maze.Start, len(combat.Combat.Monsters))
	w.apocalypseRun, w.activeDungeon = combat.Route, combat.Combat
	w.activeDungeon.Loaded = true
	if _, leave, err := s.prepareApocalypseRetreat(w, apocalypseRetreatBody()); err != nil || !leave {
		t.Fatal("native first combat retreat preparation", err)
	}
	// The captured next-room request names the source phase2 destination.
	// A source target death, rather than ordinary maze completion, owns it.
	m = w.activeDungeon.Monsters[0]
	if m.Template != w.activeDungeon.Definition.SourceBoss || m.Rank != 3 || m.NonCombat || m.Team == 0 {
		t.Fatal("native first combat does not contain its source hunt target")
	}
	w.activeDungeon.Dead[m.Entity] = true
	w.deathSent = map[uint16]bool{m.Entity: true}
	following, err := s.prepareCombatPortal(w, followingPortal(t, s, w))
	if err != nil || following.Combat == nil || following.Route.PhaseKey != 2 || following.Combat.Definition.ID != ids[2] || len(following.Combat.Monsters) == 0 {
		t.Fatal("native next combat preparation", err)
	}
	t.Logf("following-combat stage=%d phase_key=%d dungeon=%d map=%d actors=%d live_acceptance=false", following.Route.Stage, following.Route.PhaseKey, following.Combat.Definition.ID, following.Combat.Room.Map, len(following.Combat.Monsters))
	w.apocalypseRun, w.activeDungeon = following.Route, following.Combat
	s.adoptProjection(following.Packets)
	w.activeDungeon.Loaded = true
	m = w.activeDungeon.Monsters[0]
	if m.Template != w.activeDungeon.Definition.SourceBoss || m.Template != following.Route.Entry.Bosses[2] {
		t.Fatal("second combat target differs from same-source COS/DGN")
	}
	w.activeDungeon.Dead[m.Entity] = true
	w.deathSent = map[uint16]bool{m.Entity: true}
	nextTarget, err := s.prepareCombatDeathAdvance(w)
	if err != nil || nextTarget.Route == nil || nextTarget.Route.Stage != 3 || nextTarget.Route.PendingPhase != 3 || nextTarget.Route.PhaseKey != 2 {
		t.Fatal("native last ACT did not authorize pending third combat", err)
	}
	w.apocalypseRun = nextTarget.Route
	s.adoptProjection(nextTarget.Packets)
	third, err := s.prepareCombatPortal(w, followingPortal(t, s, w))
	if err != nil || third.Combat == nil || third.Route.Stage != 3 || third.Route.PhaseKey != 3 || third.Route.PendingPhase != 0 || third.Combat.Definition.ID != ids[3] {
		t.Fatal("native pending next-phase portal did not load third combat", err)
	}
	t.Logf("third-combat stage=%d dungeon=%d map=%d actors=%d live_acceptance=false", third.Route.Stage, third.Combat.Definition.ID, third.Combat.Room.Map, len(third.Combat.Monsters))
	w.apocalypseRun, w.activeDungeon = third.Route, third.Combat
	w.activeDungeon.Loaded = true
	m = w.activeDungeon.Monsters[0]
	if m.Template != third.Route.Entry.Bosses[3] || m.Template != w.activeDungeon.Definition.SourceBoss {
		t.Fatal("third combat source clear target differs")
	}
	w.activeDungeon.Dead[m.Entity] = true
	w.deathSent = map[uint16]bool{m.Entity: true}
	// Current ordinary route actually exercised by the user. The previous
	// resource-only test used expert difficulty and missed this boundary.
	declared := false
	for _, move := range s.catalog.BossMoves {
		if move.Boss == m.Template {
			declared = true
			if move.AllowsOperation(1) || !move.AllowsOperation(2) || !move.AllowsOperation(3) || move.AllowsOperation(5) {
				t.Fatal("native third-boss operation condition lost")
			}
		}
	}
	if !declared {
		t.Fatal("source third-boss next behavior was not imported")
	}
	if blocked, err := s.prepareCombatDeathAdvance(w); err != nil || blocked.Route != nil || len(blocked.Packets) != 0 {
		t.Fatal("ordinary third boss published forbidden fourth target", err)
	}
	request := make([]byte, 46)
	binary.LittleEndian.PutUint32(request[13:], third.Route.Entry.Destinations[4])
	binary.LittleEndian.PutUint32(request[17:], uint32(w.activeDungeon.Difficulty))
	if blocked, err := s.prepareCombatPortal(w, request); err == nil || blocked.Combat != nil || blocked.Route != nil {
		t.Fatal("ordinary route escaped its source endpoint")
	}
	if terminal, ready, err := s.apocalypseTerminalClear(w); err != nil || !ready || terminal != w.apocalypseRun || !terminal.Cleared[1] || !terminal.Cleared[2] {
		t.Fatal("native ordinary full clear lost earlier delivered source deaths", err)
	}
	reward, err := legion.NormalClearReward(s.catalog, s.plan.OperationID)
	if err != nil {
		t.Fatal("native ordinary guaranteed reward unavailable", err)
	}
	items, err := a.Tokens("list/stackable.lst")
	if err != nil {
		t.Fatal(err)
	}
	rewardPath := ""
	for i, token := range items {
		if token.Type == 0 && uint32(token.Value) == reward.Template && i+1 < len(items) && items[i+1].Type == 6 {
			rewardPath = items[i+1].Text
			break
		}
	}
	if rewardPath == "" {
		t.Fatal("native full-clear reward absent from source item index")
	}
	item, err := a.Tokens(rewardPath)
	if err != nil {
		t.Fatal(err)
	}
	booster := false
	for _, token := range item {
		booster = booster || token.Type == 3 && token.Text == "[booster info]"
	}
	if !booster {
		t.Fatal("source clear reward is not an actual booster item")
	}
	t.Logf("ordinary full-clear source item=%d amount=%d item_path=%s grant/UI_acceptance=false", reward.Template, reward.Amount, rewardPath)
	t.Logf("ordinary source route ends at phase_key=%d; expert next action remains conditional; settlement_acceptance=false", third.Route.PhaseKey)
}

func TestApocalypseNavigationDispatchAdoptsAfterDelivery(t *testing.T) {
	s, w := navigationEntryFixture(t)
	prepared, err := s.handle(w, navigationRequest(), legion.CmdEnterDungeon)
	if err != nil {
		t.Fatal(err)
	}
	a, peer := net.Pipe()
	defer a.Close()
	defer peer.Close()
	peer.SetDeadline(time.Now().Add(5 * time.Second))
	log := func(map[string]any) {}
	c := &gameConnection{worldState: w, legionState: *s, bootstrapped: true, event: log, output: newConnectionOutput(a, make([]byte, wire.SessionKeyBytes), "test", log)}
	done := make(chan dispatchAction, 1)
	go func() {
		done <- c.dispatchLegion(&clientRequest{frame: wire.Frame{Type: 1, ID: legion.CmdEnterDungeon}, verified: true, plaintext: navigationRequest()})
	}()
	for _, p := range prepared.Packets {
		frame := readDispatchServerFrame(t, peer)
		if frame[0] != p.Kind || binary.LittleEndian.Uint16(frame[1:]) != p.ID {
			t.Fatal("entry dispatch changed native packet order")
		}
	}
	if <-done != dispatchHandled || w.activeDungeon == nil || w.apocalypseNavigation != w.activeDungeon || w.activeDungeon.Loaded {
		t.Fatal("delivery did not establish an unloaded owned map")
	}
	if duplicate, err := c.legionState.handle(w, navigationRequest(), legion.CmdEnterDungeon); err == nil || duplicate.Navigation != nil || len(duplicate.Packets) != 0 {
		t.Fatal("duplicate entry replaced the owned map")
	}
	// A write failure must never establish the map or report successful entry.
	s2, w2 := navigationEntryFixture(t)
	a2, peer2 := net.Pipe()
	defer a2.Close()
	peer2.Close()
	c2 := &gameConnection{worldState: w2, legionState: *s2, bootstrapped: true, event: log, output: newConnectionOutput(a2, make([]byte, wire.SessionKeyBytes), "test", log)}
	if c2.dispatchLegion(&clientRequest{frame: wire.Frame{Type: 1, ID: legion.CmdEnterDungeon}, verified: true, plaintext: navigationRequest()}) != dispatchClose || w2.activeDungeon != nil || w2.apocalypseNavigation != nil {
		t.Fatal("broken delivery retained a dungeon owner")
	}
}
