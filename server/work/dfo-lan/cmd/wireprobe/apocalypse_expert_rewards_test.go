package main

import (
	"dfolan/internal/catalog"
	"dfolan/internal/legion"
	"encoding/binary"
	"os"
	"testing"
	"time"
)

func TestExpertMainRouteAndGuaranteedRewardNativePVF(t *testing.T) {
	path := os.Getenv("DFO_PVF_CORE_TEST_ARCHIVE")
	if path == "" {
		t.Skip("native PVF required")
	}
	a, err := catalog.OpenTestArchiveCached(path, os.Getenv("DFO_PVF_CORE_TEST_SHA256"))
	if err != nil {
		t.Fatal(err)
	}
	cat, err := catalog.ImportApocalypse(a)
	if err != nil {
		t.Fatal(err)
	}
	clock, err := legion.NewApocalypseClock(cat)
	if err != nil {
		t.Fatal(err)
	}
	plan, err := legion.PlanForChoice(cat, clock, 1)
	if err != nil {
		t.Fatal(err)
	}
	contents, err := catalog.ImportLegionContents(a)
	if err != nil {
		t.Fatal(err)
	}
	first, ok := clock.Next(0)
	if !ok {
		t.Fatal("missing source first combat")
	}
	route, ok, err := plan.Gates.StableMainRoute(byte(first))
	if err != nil || !ok {
		t.Fatal(route, err)
	}
	var ids []uint32
	for _, row := range contents.Contents["Apocalypse"].Dungeons {
		ids = append(ids, row.Dungeon)
	}
	dc, err := catalog.ImportRuntimeDungeons(a, ids)
	if err != nil {
		t.Fatal(err)
	}
	for _, phase := range route {
		if int(phase) >= len(ids) || dc.Dungeons[ids[phase]].SourceBoss == 0 {
			t.Fatal("main flow references no same-source boss", phase)
		}
	}
	reward, err := legion.GuaranteedClearReward(cat, plan.OperationID)
	if err != nil {
		t.Fatal(err)
	}
	itemPath := ""
	ts, err := a.Tokens("list/stackable.lst")
	if err != nil {
		t.Fatal(err)
	}
	for i, token := range ts {
		if token.Type == 0 && uint32(token.Value) == reward.Template && i+1 < len(ts) {
			itemPath = ts[i+1].Text
			break
		}
	}
	if itemPath == "" {
		t.Fatal("expert reward absent from native item index")
	}
	if _, err := a.Tokens(itemPath); err != nil {
		t.Fatal(err)
	}
	t.Logf("source=%s expert complete route=%v terminal boss=%d item=%d amount=%d path=%s; live acceptance pending", a.Snapshot().Checksum, route, dc.Dungeons[ids[route[len(route)-1]]].SourceBoss, reward.Template, reward.Amount, itemPath)
}

func expertTerminalFixture(t *testing.T) (*legionSession, *worldSession) {
	t.Helper()
	s, w, _, _ := terminalApocalypseFixture(t)
	op := s.catalog.Operation(2)
	op.GateSchedule = []int64{300, 3, 1, 0, 0, 180, 3, 0, 0, 0}
	op.GateFlow = []int64{1, 3, 4, 5, -1, 3, 2, 3, 4, 5}
	s.plan, _ = legion.PlanForChoice(s.catalog, s.clock, 1)
	s.projection[4] = 1
	s.session.Operation = 2
	w.apocalypseRun.Operation = 2
	for w.apocalypseRun.PhaseKey < 5 {
		s.session.Phase = byte(w.apocalypseRun.Stage)
		out, err := s.prepareCombatPortal(w, followingPortal(t, s, w))
		if err != nil {
			t.Fatal(err)
		}
		w.activeDungeon, w.apocalypseRun = out.Combat, out.Route
		s.adoptProjection(out.Packets)
		w.activeDungeon.Loaded = true
		m := w.activeDungeon.Monsters[0]
		m.Template = w.apocalypseRun.Entry.Bosses[w.apocalypseRun.PhaseKey]
		m.Rank = 3
		m.Team = 100
		w.activeDungeon.Definition.SourceBoss = m.Template
		w.activeDungeon.Monsters = w.activeDungeon.Monsters[:1]
		w.activeDungeon.Monsters[0] = m
		w.activeDungeon.Dead = map[uint16]bool{m.Entity: true}
		w.deathSent = map[uint16]bool{m.Entity: true}
	}
	return s, w
}

func TestExpertTerminalRequiresDeliveredCompleteMainRoute(t *testing.T) {
	s, w := expertTerminalFixture(t)
	if _, ready, err := s.apocalypseTerminalClear(w); err != nil || !ready {
		t.Fatal("full main route not terminal", err)
	}
	for _, phase := range []byte{1, 2, 3, 4} {
		w.apocalypseRun.Cleared[phase] = false
		if _, ready, _ := s.apocalypseTerminalClear(w); ready {
			t.Fatal("missing source death authorized expert reward", phase)
		}
		w.apocalypseRun.Cleared[phase] = true
	}
	s.projection[2+27+12*3] = 4
	if _, ready, _ := s.apocalypseTerminalClear(w); ready {
		t.Fatal("changed delivered history authorized reward")
	}
	s.projection[2+27+12*3] = 3
	w.activeDungeon.Dead = nil
	if _, ready, _ := s.apocalypseTerminalClear(w); ready {
		t.Fatal("live Lopes authorized settlement")
	}
}

func TestExpertRewardUsesSameCommittedNativeUIChain(t *testing.T) {
	s, w := expertTerminalFixture(t)
	_, _, svc, store := terminalApocalypseFixture(t)
	op := s.catalog.Operation(2)
	op.Reward.Label = "expert"
	op.Reward.Values = []float64{10421367, 3, 1000000, 2, 1}
	s.plan, _ = legion.PlanForChoice(s.catalog, s.clock, 1)
	store.role = w.role
	out, err := s.prepareApocalypseClear(w, svc, w.apocalypseNavigation.StartedAt.Add(time.Minute))
	if err != nil || out.Route == nil || !out.Route.RewardShown || len(store.receipts) != 2 {
		t.Fatal(out.Route, err)
	}
	if binary.LittleEndian.Uint32(out.Packets[1].Payload[9:]) != 5 || binary.LittleEndian.Uint32(out.Packets[2].Payload[4:]) != 3 {
		t.Fatal("expert native outcome or source amount lost")
	}
	w.apocalypseRun = out.Route
	s.adoptProjection(out.Packets)
	if again, err := s.prepareApocalypseClear(w, svc, time.Now()); err != nil || len(again.Packets) != 0 || len(store.receipts) != 2 {
		t.Fatal("repeat expert death paid twice", err)
	}
}
