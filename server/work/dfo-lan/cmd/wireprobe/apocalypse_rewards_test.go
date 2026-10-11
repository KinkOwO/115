package main

import (
	"context"
	"dfolan/internal/catalog"
	"dfolan/internal/catalog/pvf"
	"dfolan/internal/database"
	"dfolan/internal/inventory"
	"dfolan/internal/legion"
	"dfolan/internal/loot"
	"dfolan/internal/workflow"
	"encoding/binary"
	"encoding/json"
	"strings"
	"testing"
	"time"
)

type apocalypseGrantStore struct {
	role     database.Character
	receipts map[string]json.RawMessage
}

func (s *apocalypseGrantStore) CommitCharacterEvent(_ context.Context, account, id int64, version, key, model string, apply func(database.Character) (json.RawMessage, json.RawMessage, error)) (database.Character, bool, error) {
	if _, ok := s.receipts[key]; ok {
		return s.role, false, nil
	}
	raw, receipt, err := apply(s.role)
	if err != nil {
		return s.role, false, err
	}
	s.role.State, s.receipts[key] = raw, receipt
	return s.role, true, nil
}
func (s *apocalypseGrantStore) CharacterEventReceipt(_ context.Context, _, _ int64, key string) (json.RawMessage, error) {
	return s.receipts[key], nil
}

func terminalApocalypseFixture(t *testing.T) (*legionSession, *worldSession, *workflow.ApocalypseRewardService, *apocalypseGrantStore) {
	t.Helper()
	s, w := directDeathFixture(t)
	var err error
	s.plan, err = legion.PlanForChoice(s.catalog, s.clock, 0)
	if err != nil {
		t.Fatal(err)
	}
	s.session.Operation, w.apocalypseRun.Operation, s.projection[4] = s.plan.OperationID, s.plan.OperationID, 0
	advance, err := s.prepareCombatDeathAdvance(w)
	if err != nil || advance.Route == nil {
		t.Fatal(err)
	}
	w.apocalypseRun = advance.Route
	s.adoptProjection(advance.Packets)
	third, err := s.prepareCombatPortal(w, followingPortal(t, s, w))
	if err != nil {
		t.Fatal(err)
	}
	w.apocalypseRun, w.activeDungeon = third.Route, third.Combat
	s.adoptProjection(third.Packets)
	w.activeDungeon.Loaded = true
	m := w.activeDungeon.Monsters[0]
	m.Template, m.Rank, m.Team = third.Route.Entry.Bosses[third.Route.PhaseKey], 3, 100
	w.activeDungeon.Monsters = append(w.activeDungeon.Monsters[:0:0], m)
	w.activeDungeon.Definition.SourceBoss = m.Template
	w.activeDungeon.Dead = map[uint16]bool{m.Entity: true}
	w.deathSent = map[uint16]bool{m.Entity: true}
	w.role.ConfigVersion = w.dungeons.Source.SaveIdentity()
	w.role.State = json.RawMessage(`{"keep":true}`)
	r, err := legion.NormalClearReward(s.catalog, s.plan.OperationID)
	if err != nil {
		t.Fatal(err)
	}
	store := &apocalypseGrantStore{role: w.role, receipts: map[string]json.RawMessage{}}
	svc := &workflow.ApocalypseRewardService{Store: store, Awarder: &inventory.Awarder{Catalog: catalog.LootCatalog{Source: w.dungeons.Source, Items: map[uint32]catalog.LootItem{r.Template: {Kind: "stackable", StackableType: "[booster]", StackLimit: 100}}}, Rules: inventory.BagRules{Source: w.dungeons.Source.Checksum, Slots: map[string][2]uint16{"[booster]": {3, 4}}}}}
	s.contents.Source = w.dungeons.Source
	s.contents.SHA256 = strings.Repeat("e", 64)
	c := s.contents.Contents["Apocalypse"]
	c.Rewards = []catalog.LegionRewardRow{{Difficulty: "normal", Kind: "basic", Values: []int64{1000000, 77, 1}}, {Difficulty: "expert", Kind: "basic", Values: []int64{1000000, 77, 1}}}
	s.contents.Contents["Apocalypse"] = c
	svc.Awarder.Catalog.Items[77] = catalog.LootItem{Kind: "stackable", StackableType: "[booster]"}
	svc.Awarder.Catalog.Items[99] = catalog.LootItem{Kind: "stackable", StackableType: "[material]", StackLimit: 1000, Script: catalog.ScriptRecord{Cells: []pvf.Token{}}}
	svc.Awarder.Rules.Slots["[material]"] = [2]uint16{5, 6}
	w.loot = &loot.Service{Catalog: svc.Awarder.Catalog, RewardBoxes: bundleTestBoxes{77: {Pools: []loot.RewardBoxPool{{Draws: 1, Candidates: []loot.RewardBoxCandidate{{Template: 99, Weight: 1000000, Count: 20}}}}}}}
	return s, w, svc, store
}

func TestApocalypseTerminalClearRequiresEverySourceDeathAndOwner(t *testing.T) {
	for i, change := range []func(*legionSession, *worldSession){
		func(s *legionSession, w *worldSession) { w.apocalypseRun.Cleared[1] = false },
		func(s *legionSession, w *worldSession) { w.apocalypseRun.Cleared[2] = false },
		func(s *legionSession, w *worldSession) { w.deathSent = nil },
		func(s *legionSession, w *worldSession) { w.activeDungeon.Dead = nil },
		func(s *legionSession, w *worldSession) { w.apocalypseRun.PendingPhase = 4 },
		func(s *legionSession, w *worldSession) { w.apocalypseRun.Character++ },
		func(s *legionSession, w *worldSession) { w.activeDungeon.Definition.SourceBoss++ },
		func(s *legionSession, w *worldSession) { s.catalog.Operations[0].Reward.Values[1]++ },
	} {
		s, w, svc, store := terminalApocalypseFixture(t)
		change(s, w)
		out, _ := s.prepareApocalypseClear(w, svc, time.Now())
		if len(out.Packets) != 0 || out.Route != nil || len(store.receipts) != 0 {
			t.Fatal("unowned/incomplete clear awarded", i)
		}
	}
}

func TestApocalypseClearAwardsBeforeUIAndRewardEndReturnsWornInventory(t *testing.T) {
	s, w, svc, store := terminalApocalypseFixture(t)
	old := w.apocalypseRun
	out, err := s.prepareApocalypseClear(w, svc, w.apocalypseNavigation.StartedAt.Add(time.Minute))
	if err != nil || out.Route == nil || !out.Route.RewardShown || w.apocalypseRun != old || old.RewardShown {
		t.Fatal("clear adopted before delivery", err)
	}
	if len(store.receipts) != 2 || len(out.Packets) != 4 || out.Packets[0].ID != 14 || out.Packets[1].ID != legion.NotiLegionInfo || out.Packets[2].ID != legion.NotiClearRewardBasic || out.Packets[3].ID != legion.NotiClearRewardAdditional {
		t.Fatal("committed reward/UI chain lost")
	}
	w.apocalypseRun = out.Route
	s.adoptProjection(out.Packets)
	if repeat, err := s.prepareApocalypseClear(w, svc, time.Now()); err != nil || len(repeat.Packets) != 0 || len(store.receipts) != 2 {
		t.Fatal("duplicate death re-awarded", err)
	}
	req := legion.RewardEndRequest{First: uint32(legion.ApocalypseContent), Second: legion.ApocalypseRewardOutcome(s.projection[4]), Flag: out.Route.PhaseKey}
	for _, bad := range []legion.RewardEndRequest{{First: req.First + 1, Second: req.Second, Flag: req.Flag}, {First: req.First, Second: req.Second + 1, Flag: req.Flag}, {First: req.First, Second: req.Second, Flag: 0}} {
		if done, err := s.prepareApocalypseRewardEnd(w, bad, svc); err == nil || done.ReturnWaiting || len(done.Packets) != 0 {
			t.Fatal("unbound reward end acknowledged", bad)
		}
	}
	done, err := s.prepareApocalypseRewardEnd(w, req, svc)
	if err != nil || !done.ReturnWaiting || done.Packets[0].ID != legion.CmdRewardEnd || done.Packets[0].Kind != 1 || binary.LittleEndian.Uint32(done.Packets[1].Payload[5:]) != 0 {
		t.Fatal("reward end did not prepare common waiting return", err)
	}
	for _, p := range done.Packets {
		if p.ID == 72 && p.Kind == 1 {
			t.Fatal("reward end fabricated a retreat ACK")
		}
	}
	foundWorn := false
	for _, p := range done.Packets {
		foundWorn = foundWorn || p.Name == "apocalypse_waiting_worn_inventory_restored"
	}
	if !foundWorn || w.apocalypseRun != out.Route || !w.activeDungeon.Loaded {
		t.Fatal("return lost worn inventory or adopted before delivery")
	}
}
