package database

import (
	"dfolan/internal/catalog"
	"dfolan/internal/catalog/pvf"
	"dfolan/internal/character"
	"dfolan/internal/inventory"
	"dfolan/internal/quest"
	"encoding/json"
	"fmt"
	"sync"
	"testing"
)

func TestMainlineSkipSQLiteRewardRollbackRetryAndConcurrentReplay(t *testing.T) {
	s, ctx := sqlcTestStore(t)
	pool := s.engine.(*sqliteEngine).db
	for _, m := range []func() error{func() error { return s.Migrate(ctx) }, func() error { return s.MigrateQuests(ctx) }, func() error { return s.MigrateQuestObjectives(ctx) }, func() error { return s.MigrateQuestRewards(ctx) }, func() error { return s.MigrateCharacterEvents(ctx) }} {
		if err := m(); err != nil {
			t.Fatal(err)
		}
	}
	account, err := s.DevelopmentAccount(ctx, "mainline-fixture")
	if err != nil {
		t.Fatal(err)
	}
	source := pvf.ArchiveSnapshot{}
	version := source.SaveIdentity()
	raw := json.RawMessage(`{"level":115,"unknown":{"keep":true},"experience":123,"gold":456}`)
	makeRole := func(name string) Character {
		role, e := s.CreateCharacter(ctx, Character{AccountID: account, Name: name, Request: []byte(name), ConfigVersion: version, State: raw}, 24)
		if e != nil {
			t.Fatal(e)
		}
		return role
	}
	definitions := map[uint32]catalog.QuestDefinition{}
	for _, id := range []uint32{100, 101} {
		d := catalog.QuestDefinition{ID: id, MinimumLevel: 1, MaximumLevel: 1000, Script: catalog.ScriptRecord{Cells: []pvf.Token{{Type: 3, Text: "[grade]"}, {Type: 6, Text: "[epic]"}}}}
		if id == 101 {
			d.Prerequisites = []uint32{100}
		}
		definitions[id] = d
	}
	awarder := &inventory.Awarder{Catalog: catalog.LootCatalog{Items: map[uint32]catalog.LootItem{
		20: {Kind: "stackable", StackableType: "[waste]", StackLimit: 1}, 21: {Kind: "stackable", StackableType: "[waste]", StackLimit: 1},
	}}, Rules: inventory.BagRules{Slots: map[string][2]uint16{"default": {65, 65}, "[waste]": {65, 65}}, MissingStackLimit: 1}}
	service := &quest.Service{Store: s, Catalog: catalog.QuestCatalog{Source: source, Quests: definitions}, Skip: &catalog.QuestSkip{Source: source, Blocked: map[uint32]bool{}, Items: []uint32{20, 21}}, Inventory: awarder, Progression: &character.ProgressionService{Rules: character.GrowthRules{Model: "fixture-growth"}}}
	skip := func(role Character) (quest.SkipResult, error) {
		return service.SkipAllNow(ctx, role, "fixture", "mainline test", quest.MainlineGrades(), 0, quest.FinishRewards{})
	}
	role := makeRole("Rollback")
	if _, err = skip(role); err == nil {
		t.Fatal("expected full bag failure")
	}
	for _, table := range []string{"character_quests", "character_quest_rewards", "character_events"} {
		var count int
		if err = pool.QueryRowContext(ctx, "SELECT count(*) FROM "+table+" WHERE character_id=$1", role.ID).Scan(&count); err != nil || count != 0 {
			t.Fatalf("partial %s committed %d %v", table, count, err)
		}
	}
	var after json.RawMessage
	if err = pool.QueryRowContext(ctx, `SELECT state FROM characters WHERE id=$1`, role.ID).Scan(&after); err != nil || !sameJSON(t, after, raw) {
		t.Fatalf("failed grant changed state %s %v", after, err)
	}
	awarder.Rules.Slots["default"] = [2]uint16{65, 66}
	awarder.Rules.Slots["[waste]"] = [2]uint16{65, 66}
	result, err := skip(role)
	if err != nil || len(result.Settled) != 2 || len(result.KeyItems) != 2 {
		t.Fatalf("retry result %+v err=%v", result, err)
	}
	var preserved map[string]json.RawMessage
	_ = json.Unmarshal(result.Role.State, &preserved)
	if string(preserved["unknown"]) != "{\"keep\": true}" && !sameJSON(t, preserved["unknown"], json.RawMessage(`{"keep":true}`)) {
		t.Fatalf("unknown state lost %s", result.Role.State)
	}
	if string(preserved["experience"]) != "123" || string(preserved["gold"]) != "456" {
		t.Fatalf("skip paid XP/gold %s", result.Role.State)
	}
	replay, err := skip(role) // Deliberately stale state; the locked aggregate wins.
	if err != nil || len(replay.Settled) != 0 || len(replay.KeyItems) != 0 {
		t.Fatalf("repeat paid twice %+v %v", replay, err)
	}
	bag, _ := inventory.ReadBag(replay.Role.State)
	if len(bag.Items) != 2 {
		t.Fatalf("repeat bag %v", bag.Items)
	}
	wrong := role
	wrong.AccountID++
	if _, err = skip(wrong); err == nil {
		t.Fatal("ownership bypass")
	}
	concurrent := makeRole("Concurrent")
	// Prime the immutable quest index before concurrent calls (normal bootstrap).
	_ = service.Index()
	var wg sync.WaitGroup
	var mu sync.Mutex
	var failures []error
	totalQuests, totalKeys := 0, 0
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			r, e := skip(concurrent)
			mu.Lock()
			defer mu.Unlock()
			if e != nil {
				failures = append(failures, e)
			}
			totalQuests += len(r.Settled)
			totalKeys += len(r.KeyItems)
		}()
	}
	wg.Wait()
	if len(failures) > 0 || totalQuests != 2 || totalKeys != 2 {
		t.Fatalf("concurrent duplicate %d %d %v", totalQuests, totalKeys, failures)
	}
	// Repair a legacy failed key receipt even with no further skippable quests.
	seeded, _, err := awarder.Grant(raw, 20, 1)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = pool.ExecContext(ctx, `UPDATE characters SET state=$2 WHERE id=$1`, role.ID, seeded); err != nil {
		t.Fatal(err)
	}
	if _, err = pool.ExecContext(ctx, `UPDATE character_events SET outcome=$2 WHERE character_id=$1 AND event_key='quest-skip-ticket-items'`, role.ID, json.RawMessage(`{"granted":[20],"failed":{"21":"full"}}`)); err != nil {
		t.Fatal(err)
	}
	repaired, err := skip(role)
	if err != nil || len(repaired.Settled) != 0 || len(repaired.KeyItems) != 1 || repaired.KeyItems[0] != 21 {
		t.Fatalf("no-target legacy retry %+v %v", repaired, err)
	}
	repaired, err = skip(role)
	if err != nil || len(repaired.KeyItems) != 0 {
		t.Fatalf("legacy repair duplicated %v %v", repaired.KeyItems, err)
	}
	var rewardCount int
	if err = pool.QueryRowContext(ctx, `SELECT count(*) FROM character_quest_rewards WHERE character_id=$1`, role.ID).Scan(&rewardCount); err != nil || rewardCount != 2 {
		t.Fatal(fmt.Sprintf("reward receipt count %d %v", rewardCount, err))
	}
}
