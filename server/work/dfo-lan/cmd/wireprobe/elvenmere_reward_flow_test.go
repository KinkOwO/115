package main

import (
	"context"
	"dfolan/internal/catalog"
	"dfolan/internal/catalog/pvf"
	"dfolan/internal/character"
	"dfolan/internal/database"
	"dfolan/internal/dungeon"
	"dfolan/internal/inventory"
	"dfolan/internal/loot"
	"dfolan/internal/savecontract"
	"dfolan/internal/testfixture"
	"dfolan/internal/workflow"
	"encoding/json"
	"strings"
	"sync"
	"testing"
)

func elvenmereRewardFixture(t *testing.T, full bool) (*worldSession, *database.TestFixture) {
	t.Helper()
	ctx := context.Background()
	f, err := database.OpenTestFixture(ctx)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := f.Close(); err != nil {
			t.Error(err)
		}
	})
	store := f.Storage()
	if err := store.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	if err := store.MigrateCharacterEvents(ctx); err != nil {
		t.Fatal(err)
	}
	account, err := store.DevelopmentAccount(ctx, "elvenmere-reward-fixture")
	if err != nil {
		t.Fatal(err)
	}
	bag := inventory.Bag{Version: "ordinary-bag-v1", Gold: 1234}
	if full {
		bag.Items = []inventory.BagItem{{Slot: 121, Template: 60000, Amount: 1}}
	}
	raw, err := inventory.SaveBag(json.RawMessage(`{"level":115,"keep":{"value":42}}`), bag)
	if err != nil {
		t.Fatal(err)
	}
	role, err := store.CreateCharacter(ctx, database.Character{AccountID: account, Name: "ElvenmereFixture", ConfigVersion: savecontract.Identity(), State: raw, Request: []byte{0}}, 24)
	if err != nil {
		t.Fatal(err)
	}
	role.WireID = 37
	end := uint16(122)
	if full {
		end = 121
	}
	w := &worldSession{account: account, role: role, level: 115, store: store,
		loot: &loot.Service{Catalog: catalog.LootCatalog{Source: pvf.ArchiveSnapshot{Checksum: savecontract.Identity()}, Items: map[uint32]catalog.LootItem{
			10336354: {ID: 10336354, Kind: "stackable", StackableType: "[material]", StackLimit: 100},
			10333027: {ID: 10333027, Kind: "stackable", StackableType: "[material]", StackLimit: 100},
			60000:    {ID: 60000, Kind: "stackable", StackableType: "[material]", StackLimit: 1},
		}}, BagRules: inventory.BagRules{Source: savecontract.Identity(), Slots: map[string][2]uint16{"[material]": {121, end}}, MissingStackLimit: 100}},
		dungeons:      &catalog.DungeonCatalog{Elvenmere: elvenmereFixtureRules(0)},
		activeDungeon: &dungeon.Session{RunID: strings.Repeat("e", 32), Definition: catalog.DungeonDefinition{ID: 100003126}, Extra: 5},
	}
	w.activeDungeon.MarkSceneCompleted()
	return w, f
}

func TestElvenmereFullBagRollsBackFloorAndFlags(t *testing.T) {
	w, f := elvenmereRewardFixture(t, true)
	before := string(w.role.State)
	plan, err := w.elvenmereTeleport(make([]byte, 25))
	if err == nil || len(plan) != 0 || w.activeDungeon.Extra != 5 || w.activeDungeon.WeeklyRewards[4] != 0 || w.activeDungeon.SeasonRewards[4] != 0 || string(w.role.State) != before {
		t.Fatal("failed floor advanced or consumed reward flags", err, plan)
	}
	if count, err := f.EventCount(context.Background(), w.role.ID); err != nil || count != 0 {
		t.Fatal("failed grant left a receipt", count, err)
	}
	saved, err := f.Storage().AdminCharacter(context.Background(), w.role.ID)
	if err != nil || string(saved.State) != before {
		t.Fatal("failed grant changed durable state", err)
	}
}

func TestElvenmereFloorCommitReplaysAndKeepsCurrentState(t *testing.T) {
	w, f := elvenmereRewardFixture(t, false)
	stale := w.role
	// A different request commits after this session's snapshot. Reward grants
	// must start from the locked current character, preserving that change.
	_, _, err := w.store.CommitCharacterEvent(context.Background(), w.account, w.role.ID, w.role.ConfigVersion, "fixture:concurrent", "fixture", func(cur database.Character) (json.RawMessage, json.RawMessage, error) {
		var fields map[string]json.RawMessage
		if err := json.Unmarshal(cur.State, &fields); err != nil {
			return nil, nil, err
		}
		fields["concurrent"] = json.RawMessage(`99`)
		state, err := json.Marshal(fields)
		return state, json.RawMessage(`{}`), err
	})
	if err != nil {
		t.Fatal(err)
	}
	plan, err := w.elvenmereTeleport(make([]byte, 25))
	if err != nil || len(plan) != 3 || plan[0].ID != 2015 || plan[1].ID != 14 || plan[2].ID != 2193 || w.activeDungeon.Extra != 6 {
		t.Fatal("committed floor response", plan, err)
	}
	if w.activeDungeon.WeeklyRewards[4] != 1 || w.activeDungeon.SeasonRewards[4] != 1 || w.role.WireID != 37 {
		t.Fatal("committed flags or actor identity missing")
	}
	// Recreate a stale connection for the same run/floor. A lost response must
	// recover from the receipt, without granting either reward twice.
	w.role = stale
	w.activeDungeon = &dungeon.Session{RunID: strings.Repeat("e", 32), Definition: catalog.DungeonDefinition{ID: 100003126}, Extra: 5}
	w.activeDungeon.MarkSceneCompleted()
	w.activeDungeon.WeeklyRewards, w.activeDungeon.SeasonRewards = [100]byte{}, [100]byte{}
	if _, err := w.elvenmereTeleport(make([]byte, 25)); err != nil {
		t.Fatal(err)
	}
	reopened, err := f.Reopen(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	saved, err := reopened.AdminCharacter(context.Background(), stale.ID)
	if err != nil {
		t.Fatal(err)
	}
	bag, err := inventory.ReadBag(saved.State)
	if err != nil || bag.Gold != 1234 || len(bag.Items) != 2 {
		t.Fatal("assets changed unexpectedly", bag, err)
	}
	counts := map[uint32]uint32{}
	for _, row := range bag.Items {
		counts[row.Template] += row.Amount
	}
	if counts[10336354] != 5 || counts[10333027] != 1 {
		t.Fatal("duplicate or missing floor rewards", counts)
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(saved.State, &fields); err != nil {
		t.Fatal(err)
	}
	if string(fields["concurrent"]) != "99" || string(fields["keep"]) != `{"value":42}` {
		t.Fatal("stale reward overwrote another request or unknown fields", fields)
	}
}

func TestElvenmereStorageFailureKeepsSessionUnchanged(t *testing.T) {
	w, f := elvenmereRewardFixture(t, false)
	before := string(w.role.State)
	f.Storage().Close()
	plan, err := w.elvenmereTeleport(make([]byte, 25))
	if err == nil || len(plan) != 0 || w.activeDungeon.Extra != 5 || w.activeDungeon.WeeklyRewards[4] != 0 || string(w.role.State) != before {
		t.Fatal("storage failure produced success", plan, err)
	}
}

func TestElvenmerePartialGrantOrGrowthFailureRollsBackAll(t *testing.T) {
	for _, reason := range []string{"second item full", "growth invalid"} {
		t.Run(reason, func(t *testing.T) {
			w, f := elvenmereRewardFixture(t, false)
			if reason == "second item full" {
				w.loot.BagRules.Slots["[material]"] = [2]uint16{121, 121}
			} else {
				w.progression = &character.ProgressionService{}
				w.dungeons.Elvenmere = elvenmereFixtureRules(7)
			}
			before := string(w.role.State)
			plan, err := w.elvenmereTeleport(make([]byte, 25))
			if err == nil || len(plan) != 0 || w.activeDungeon.Extra != 5 || w.activeDungeon.WeeklyRewards[4] != 0 || w.activeDungeon.SeasonRewards[4] != 0 {
				t.Fatal("partial transaction reported success", plan, err)
			}
			saved, err := f.Storage().AdminCharacter(context.Background(), w.role.ID)
			if err != nil || string(saved.State) != before || string(w.role.State) != before {
				t.Fatal("first reward survived a later failure", err)
			}
			if n, err := f.EventCount(context.Background(), w.role.ID); err != nil || n != 0 {
				t.Fatal("rolled back floor left receipt", n, err)
			}
		})
	}
}

func TestElvenmereItemsAndExperienceCommitTogether(t *testing.T) {
	svc, store, role := awakeningSQLiteFixture(t, 1)
	ctx := context.Background()
	if err := store.MigrateCharacterEvents(ctx); err != nil {
		t.Fatal(err)
	}
	growth, err := catalog.LoadProgression(testfixture.ProgressionPath(t))
	if err != nil {
		t.Fatal(err)
	}
	growth.Source = svc.Catalog.Source
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(role.State, &fields); err != nil {
		t.Fatal(err)
	}
	fields["level"] = json.RawMessage(`110`)
	fields["experience"], _ = json.Marshal(growth.Thresholds[108])
	raw, _ := json.Marshal(fields)
	raw, err = inventory.SaveBag(raw, inventory.Bag{Version: "ordinary-bag-v1", Gold: 1234})
	if err != nil {
		t.Fatal(err)
	}
	role, _, err = store.CommitCharacterEvent(ctx, role.AccountID, role.ID, role.ConfigVersion, "fixture:elvenmere", "fixture", func(database.Character) (json.RawMessage, json.RawMessage, error) {
		return raw, json.RawMessage(`{}`), nil
	})
	if err != nil {
		t.Fatal(err)
	}
	role.WireID = 37
	w := &worldSession{account: role.AccountID, role: role, level: 110, store: store,
		progression: &character.ProgressionService{Catalog: growth, Professions: svc.Catalog, Rules: character.GrowthRules{LevelCap: 115}},
		loot: &loot.Service{Catalog: catalog.LootCatalog{Source: svc.Catalog.Source, Items: map[uint32]catalog.LootItem{
			10336354: {ID: 10336354, Kind: "stackable", StackableType: "[material]", StackLimit: 100},
			10333027: {ID: 10333027, Kind: "stackable", StackableType: "[material]", StackLimit: 100},
		}}, BagRules: inventory.BagRules{Source: svc.Catalog.Source.Checksum, Slots: map[string][2]uint16{"[material]": {121, 122}}, MissingStackLimit: 100}},
		dungeons:      &catalog.DungeonCatalog{Elvenmere: elvenmereFixtureRules(210000)},
		activeDungeon: &dungeon.Session{RunID: strings.Repeat("f", 32), Definition: catalog.DungeonDefinition{ID: 100003126}, Extra: 5},
	}
	w.activeDungeon.MarkSceneCompleted()
	plan, err := w.elvenmereTeleport(make([]byte, 25))
	if err != nil || len(plan) != 4 || plan[2].ID != 37 {
		t.Fatal("items and experience response", plan, err)
	}
	saved, err := store.AdminCharacter(ctx, role.ID)
	if err != nil {
		t.Fatal(err)
	}
	var after character.State
	if err := json.Unmarshal(saved.State, &after); err != nil {
		t.Fatal(err)
	}
	if after.Experience <= growth.Thresholds[108] {
		t.Fatal("floor experience was not saved")
	}
	w.role = role
	w.activeDungeon = &dungeon.Session{RunID: strings.Repeat("f", 32), Definition: catalog.DungeonDefinition{ID: 100003126}, Extra: 5}
	w.activeDungeon.MarkSceneCompleted()
	w.activeDungeon.WeeklyRewards, w.activeDungeon.SeasonRewards = [100]byte{}, [100]byte{}
	if _, err := w.elvenmereTeleport(make([]byte, 25)); err != nil {
		t.Fatal(err)
	}
	again, err := store.AdminCharacter(ctx, role.ID)
	if err != nil || string(again.State) != string(saved.State) {
		t.Fatal("replay duplicated experience or items", err)
	}
}

func elvenmereFixtureRules(experience uint64) *catalog.ElvenmereRules {
	return &catalog.ElvenmereRules{MaxFloor: 100, Floors: map[byte]catalog.ElvenmereFloorRule{5: {Weekly: catalog.ElvenmereItemReward{Template: 10336354, Count: 5}, Season: catalog.ElvenmereItemReward{Template: 10333027, Count: 1}, Experience: experience}}}
}

func TestElvenmereZeroSourceExperienceDoesNotApplyGrowth(t *testing.T) {
	w, _ := elvenmereRewardFixture(t, false)
	w.progression = &character.ProgressionService{}
	plan, err := w.elvenmereTeleport(make([]byte, 25))
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range plan {
		if p.ID == 37 {
			t.Fatal("zero source experience emitted a growth update")
		}
	}
}

// Review assertions describe the expected prevention, not the buggy result.
func TestReviewDuplicateTeleportMustNotClearNextFloor(t *testing.T) {
	w, f := elvenmereRewardFixture(t, false)
	w.dungeons.Elvenmere.Floors[6] = catalog.ElvenmereFloorRule{Weekly: catalog.ElvenmereItemReward{Template: 10336354, Count: 5}}
	body := make([]byte, 25)
	if _, err := w.elvenmereTeleport(body); err != nil {
		t.Fatal(err)
	}
	saved, err := f.Storage().AdminCharacter(context.Background(), w.role.ID)
	if err != nil {
		t.Fatal(err)
	}
	before := string(saved.State)
	// No new boss/death/load/clear event happened between these requests.
	_, secondErr := w.elvenmereTeleport(body)
	saved, err = f.Storage().AdminCharacter(context.Background(), w.role.ID)
	if err != nil {
		t.Fatal(err)
	}
	count, err := f.EventCount(context.Background(), w.role.ID)
	if err != nil {
		t.Fatal(err)
	}
	bag, err := inventory.ReadBag(saved.State)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("repeat_error=%v floor=%d completed=%v events=%d bag=%+v", secondErr, w.activeDungeon.Extra, w.activeDungeon.Completed(), count, bag.Items)
	if w.activeDungeon.Extra != 6 || string(saved.State) != before || count != 1 {
		t.Fatal("a repeated teleport cleared and rewarded the next floor without a new completion")
	}
}

func TestReviewNewRunMustNotReclaimWeeklyAndSeasonRewards(t *testing.T) {
	w, f := elvenmereRewardFixture(t, false)
	body := make([]byte, 25)
	if _, err := w.elvenmereTeleport(body); err != nil {
		t.Fatal(err)
	}
	saved, err := f.Storage().AdminCharacter(context.Background(), w.role.ID)
	if err != nil {
		t.Fatal(err)
	}
	before := string(saved.State)
	// Same character, source and moment; emulate a freshly selected run.
	w.activeDungeon = &dungeon.Session{RunID: strings.Repeat("d", 32), Definition: catalog.DungeonDefinition{ID: 100003126}, Extra: 5}
	w.activeDungeon.MarkSceneCompleted()
	_, secondErr := w.elvenmereTeleport(body)
	saved, err = f.Storage().AdminCharacter(context.Background(), w.role.ID)
	if err != nil {
		t.Fatal(err)
	}
	count, err := f.EventCount(context.Background(), w.role.ID)
	if err != nil {
		t.Fatal(err)
	}
	bag, err := inventory.ReadBag(saved.State)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("new_run_error=%v events=%d bag=%+v", secondErr, count, bag.Items)
	if string(saved.State) != before || count != 1 {
		t.Fatal("a new run reclaimed weekly and season rewards without a cycle change")
	}
}

func TestElvenmereNewCompletionAllowsNextFloor(t *testing.T) {
	w, f := elvenmereRewardFixture(t, false)
	w.dungeons.Elvenmere.Floors[6] = catalog.ElvenmereFloorRule{Weekly: catalog.ElvenmereItemReward{Template: 10336354, Count: 5}}
	body := make([]byte, 25)
	if _, err := w.elvenmereTeleport(body); err != nil {
		t.Fatal(err)
	}
	if w.activeDungeon.Completed() || w.activeDungeon.Loaded {
		t.Fatal("prior completion/load was not consumed")
	}
	// Emulate the next owned load and new completion, not a repeated teleport.
	w.activeDungeon.Loaded = true
	w.activeDungeon.MarkSceneCompleted()
	if _, err := w.elvenmereTeleport(body); err != nil {
		t.Fatal(err)
	}
	saved, err := f.Storage().AdminCharacter(context.Background(), w.role.ID)
	if err != nil {
		t.Fatal(err)
	}
	bag, err := inventory.ReadBag(saved.State)
	if err != nil {
		t.Fatal(err)
	}
	if w.activeDungeon.Extra != 7 || bag.Items[0].Amount != 10 || w.activeDungeon.WeeklyRewards[5] != 1 {
		t.Fatal("new completion was blocked or lost", bag)
	}
}

func TestElvenmereConcurrentRunsHoldOnlyOneReward(t *testing.T) {
	w, f := elvenmereRewardFixture(t, false)
	second, err := f.Reopen(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer second.Close()
	stores := []*database.Store{w.store, second}
	errors := make(chan error, 2)
	var wg sync.WaitGroup
	for i, store := range stores {
		wg.Add(1)
		go func(i int, store *database.Store) {
			defer wg.Done()
			run := strings.Repeat("c", 32)
			if i == 1 {
				run = strings.Repeat("d", 32)
			}
			_, _, err := (&workflow.LootService{Store: store, Loot: w.loot}).CommitElvenmereFloor(context.Background(), w.role, workflow.ElvenmereFloorGrant{Run: run, Floor: 5, WeeklyItem: 10336354, WeeklyCount: 5, SeasonItem: 10333027, SeasonCount: 1}, nil)
			errors <- err
		}(i, store)
	}
	wg.Wait()
	close(errors)
	for err := range errors {
		if err != nil {
			t.Fatal(err)
		}
	}
	saved, err := w.store.AdminCharacter(context.Background(), w.role.ID)
	if err != nil {
		t.Fatal(err)
	}
	bag, err := inventory.ReadBag(saved.State)
	if err != nil {
		t.Fatal(err)
	}
	if len(bag.Items) != 2 || bag.Items[0].Amount != 5 || bag.Items[1].Amount != 1 {
		t.Fatal("concurrent duplicate grant", bag)
	}
	if count, err := f.EventCount(context.Background(), w.role.ID); err != nil || count != 1 {
		t.Fatal("unexpected concurrent events", count, err)
	}
}

func TestElvenmereHoldsSurviveReopenAndRestore(t *testing.T) {
	w, f := elvenmereRewardFixture(t, false)
	if _, err := w.elvenmereTeleport(make([]byte, 25)); err != nil {
		t.Fatal(err)
	}
	reopened, err := f.Reopen(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	saved, err := (&workflow.LootService{Store: reopened, Loot: w.loot}).RestoreElvenmereProgress(context.Background(), w.role)
	if err != nil {
		t.Fatal(err)
	}
	p, exists, err := dungeon.ReadElvenmereProgress(saved.State)
	if err != nil || !exists || p.MaxCleared != 5 {
		t.Fatal("lost durable progress", p, err)
	}
	weekly, season := p.RewardFlags()
	if weekly[4] != 1 || season[4] != 1 {
		t.Fatal("lost flags")
	}
	w.role, w.store = saved, reopened
	w.activeDungeon = &dungeon.Session{RunID: strings.Repeat("d", 32), Definition: catalog.DungeonDefinition{ID: 100003126}, Extra: 5}
	w.activeDungeon.MarkSceneCompleted()
	if _, err := w.elvenmereTeleport(make([]byte, 25)); err != nil {
		t.Fatal(err)
	}
	again, err := reopened.AdminCharacter(context.Background(), saved.ID)
	if err != nil || string(again.State) != string(saved.State) {
		t.Fatal("reopen allowed repeat grant", err)
	}
}

func TestElvenmereRestoresLegacyR1WithoutGrant(t *testing.T) {
	w, f := elvenmereRewardFixture(t, false)
	grant := workflow.ElvenmereFloorGrant{Run: strings.Repeat("a", 32), Floor: 5, WeeklyItem: 10336354, WeeklyCount: 5, SeasonItem: 10333027, SeasonCount: 1}
	key := "elvenmere-floor:" + grant.Run + ":5"
	saved, _, err := w.store.CommitCharacterEvent(context.Background(), w.account, w.role.ID, w.role.ConfigVersion, key, "elvenmere-floor-v1", func(cur database.Character) (json.RawMessage, json.RawMessage, error) {
		awarder := &inventory.Awarder{Catalog: w.loot.Catalog, Rules: w.loot.BagRules}
		state, _, err := awarder.Grant(cur.State, 10336354, 5)
		if err != nil {
			return nil, nil, err
		}
		state, _, err = awarder.Grant(state, 10333027, 1)
		if err != nil {
			return nil, nil, err
		}
		proof, err := json.Marshal(workflow.ElvenmereFloorReceipt{Grant: grant})
		return state, proof, err
	})
	if err != nil {
		t.Fatal(err)
	}
	service := &workflow.LootService{Store: w.store, Loot: w.loot}
	restored, err := service.RestoreElvenmereProgress(context.Background(), w.role)
	if err != nil {
		t.Fatal(err)
	}
	beforeBag, err := inventory.ReadBag(saved.State)
	if err != nil {
		t.Fatal(err)
	}
	afterBag, err := inventory.ReadBag(restored.State)
	if err != nil || len(inventory.ChangedItemRows(beforeBag, afterBag)) != 0 {
		t.Fatal("migration paid again", err)
	}
	p, exists, err := dungeon.ReadElvenmereProgress(restored.State)
	if err != nil || !exists || p.Weekly[5].EventKey != key || p.Season[5].EventKey != key {
		t.Fatal("legacy holds missing", p, err)
	}
	w.role = restored
	if _, err := w.elvenmereTeleport(make([]byte, 25)); err != nil {
		t.Fatal(err)
	}
	after, err := w.store.AdminCharacter(context.Background(), w.role.ID)
	if err != nil || string(after.State) != string(restored.State) {
		t.Fatal("legacy receipt did not block repeat", err)
	}
	if count, err := f.EventCount(context.Background(), w.role.ID); err != nil || count != 2 {
		t.Fatal("legacy/restoration event count", count, err)
	}
	// Archiving the character must not remove its account's legacy claim holds.
	ctx := context.Background()
	if id, err := w.store.DeleteCharacter(ctx, w.account, 0, w.role.Name); err != nil || id != w.role.ID {
		t.Fatal("archive fixture", id, err)
	}
	sibling := elvenmereSibling(t, w, w.account, "LegacyAccountSibling")
	grant.Run = strings.Repeat("b", 32)
	shared, _, err := service.CommitElvenmereFloor(ctx, sibling, grant, nil)
	if err != nil {
		t.Fatal(err)
	}
	if weekly, season := elvenmereHeldItems(t, shared); weekly != 0 || season != 0 {
		t.Fatal("archived legacy character allowed account regrant", weekly, season)
	}
}

func TestElvenmereInvalidStoredProgressFailsClosed(t *testing.T) {
	for _, invalid := range []string{`null`, `{"version":2}`, `{"version":1,"max_cleared":5,"weekly":{"0":{"event_key":"x"}}}`} {
		t.Run(invalid, func(t *testing.T) {
			w, f := elvenmereRewardFixture(t, false)
			saved, _, err := w.store.CommitCharacterEvent(context.Background(), w.account, w.role.ID, w.role.ConfigVersion, "fixture:bad-progress", "fixture", func(cur database.Character) (json.RawMessage, json.RawMessage, error) {
				var fields map[string]json.RawMessage
				if err := json.Unmarshal(cur.State, &fields); err != nil {
					return nil, nil, err
				}
				fields["elvenmere_progress"] = json.RawMessage(invalid)
				state, err := json.Marshal(fields)
				return state, json.RawMessage(`{}`), err
			})
			if err != nil {
				t.Fatal(err)
			}
			w.role = saved
			if plan, err := w.elvenmereTeleport(make([]byte, 25)); err == nil || len(plan) != 0 || w.activeDungeon.Extra != 5 {
				t.Fatal("invalid ledger advanced", plan, err)
			}
			after, err := w.store.AdminCharacter(context.Background(), w.role.ID)
			if err != nil || string(after.State) != string(saved.State) {
				t.Fatal("invalid ledger overwritten", err)
			}
			if count, err := f.EventCount(context.Background(), w.role.ID); err != nil || count != 1 {
				t.Fatal("invalid ledger created reward", count, err)
			}
		})
	}
}

func elvenmereSibling(t *testing.T, w *worldSession, account int64, name string) database.Character {
	t.Helper()
	raw, err := inventory.SaveBag(json.RawMessage(`{"level":115,"keep":{"value":42}}`), inventory.Bag{Version: "ordinary-bag-v1", Gold: 1234})
	if err != nil {
		t.Fatal(err)
	}
	role, err := w.store.CreateCharacter(context.Background(), database.Character{AccountID: account, Name: name, ConfigVersion: w.role.ConfigVersion, State: raw, Request: []byte{0}}, 24)
	if err != nil {
		t.Fatal(err)
	}
	return role
}

func elvenmereHeldItems(t *testing.T, role database.Character) (uint32, uint32) {
	t.Helper()
	bag, err := inventory.ReadBag(role.State)
	if err != nil {
		t.Fatal(err)
	}
	var weekly, season uint32
	for _, row := range bag.Items {
		if row.Template == 10336354 {
			weekly += row.Amount
		}
		if row.Template == 10333027 {
			season += row.Amount
		}
	}
	return weekly, season
}

func TestElvenmereAccountProjectionRefreshesAndIsolatesAccounts(t *testing.T) {
	w, f := elvenmereRewardFixture(t, false)
	ctx := context.Background()
	other := elvenmereSibling(t, w, w.account, "SameAccount")
	svc := &workflow.LootService{Store: w.store, Loot: w.loot}
	first, err := svc.RestoreElvenmereProgress(ctx, other)
	if err != nil {
		t.Fatal(err)
	}
	p, _, err := dungeon.ReadElvenmereProgress(first.State)
	if err != nil || p.MaxCleared != 0 {
		t.Fatal(p, err)
	}
	durable, err := w.store.AdminCharacter(ctx, other.ID)
	if err != nil || string(durable.State) != string(other.State) {
		t.Fatal("entry wrote character state", err)
	}
	if n, err := f.EventCount(ctx, other.ID); err != nil || n != 0 {
		t.Fatal("entry wrote an event", n, err)
	}
	if _, err := w.elvenmereTeleport(make([]byte, 25)); err != nil {
		t.Fatal(err)
	}
	// A prior entry must not hide new clears by a sibling character.
	for i := 0; i < 2; i++ {
		projected, err := svc.RestoreElvenmereProgress(ctx, first)
		if err != nil {
			t.Fatal(err)
		}
		p, _, err := dungeon.ReadElvenmereProgress(projected.State)
		if err != nil || p.MaxCleared != 5 || p.Weekly[5].EventKey == "" || p.Season[5].EventKey == "" {
			t.Fatal("stale account projection", p, err)
		}
	}
	grant := workflow.ElvenmereFloorGrant{Run: strings.Repeat("b", 32), Floor: 6, WeeklyItem: 10336354, WeeklyCount: 5}
	if _, _, err := svc.CommitElvenmereFloor(ctx, other, grant, nil); err != nil {
		t.Fatal(err)
	}
	// Replaying A's old receipt must include B's new progress without paying A.
	replay, _, err := svc.CommitElvenmereFloor(ctx, w.role, workflow.ElvenmereFloorGrant{Run: strings.Repeat("e", 32), Floor: 5, WeeklyItem: 10336354, WeeklyCount: 5, SeasonItem: 10333027, SeasonCount: 1}, nil)
	if err != nil {
		t.Fatal(err)
	}
	p, _, err = dungeon.ReadElvenmereProgress(replay.State)
	if err != nil || p.MaxCleared != 6 || p.Weekly[6].EventKey == "" {
		t.Fatal("replay returned stale progress", p, err)
	}
	grant.Run, grant.Floor, grant.SeasonItem, grant.SeasonCount = strings.Repeat("c", 32), 5, 10333027, 1
	saved, _, err := svc.CommitElvenmereFloor(ctx, other, grant, nil)
	if err != nil {
		t.Fatal(err)
	}
	if weekly, season := elvenmereHeldItems(t, saved); weekly != 5 || season != 0 {
		t.Fatal("sibling reclaimed account reward", weekly, season)
	}
	// An existing role projection must merge later account progress durably.
	grant.Run = strings.Repeat("d", 32)
	saved, _, err = svc.CommitElvenmereFloor(ctx, w.role, grant, nil)
	if err != nil {
		t.Fatal(err)
	}
	p, _, err = dungeon.ReadElvenmereProgress(saved.State)
	if err != nil || p.MaxCleared != 6 || p.Weekly[6].EventKey == "" {
		t.Fatal("account merge was rolled back", p, err)
	}
	if weekly, season := elvenmereHeldItems(t, saved); weekly != 5 || season != 1 {
		t.Fatal("account merge awarded again", weekly, season)
	}
	account, err := w.store.DevelopmentAccount(ctx, "another-elvenmere-account")
	if err != nil {
		t.Fatal(err)
	}
	independent := elvenmereSibling(t, w, account, "OtherAccount")
	grant.Run = strings.Repeat("f", 32)
	saved, _, err = svc.CommitElvenmereFloor(ctx, independent, grant, nil)
	if err != nil {
		t.Fatal(err)
	}
	if weekly, season := elvenmereHeldItems(t, saved); weekly != 5 || season != 1 {
		t.Fatal("other account incorrectly held", weekly, season)
	}
}

func TestElvenmereConcurrentAccountCharactersClaimOnce(t *testing.T) {
	w, f := elvenmereRewardFixture(t, false)
	ctx := context.Background()
	other := elvenmereSibling(t, w, w.account, "ConcurrentSibling")
	second, err := f.Reopen(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer second.Close()
	stores := []*database.Store{w.store, second}
	roles := []database.Character{w.role, other}
	errors := make(chan error, 2)
	var wg sync.WaitGroup
	for i := range roles {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_, _, err := (&workflow.LootService{Store: stores[i], Loot: w.loot}).CommitElvenmereFloor(ctx, roles[i], workflow.ElvenmereFloorGrant{Run: strings.Repeat("a", 32), Floor: 5, WeeklyItem: 10336354, WeeklyCount: 5, SeasonItem: 10333027, SeasonCount: 1}, nil)
			errors <- err
		}(i)
	}
	wg.Wait()
	close(errors)
	for err := range errors {
		if err != nil {
			t.Fatal(err)
		}
	}
	var weekly, season uint32
	for _, role := range roles {
		saved, err := w.store.AdminCharacter(ctx, role.ID)
		if err != nil {
			t.Fatal(err)
		}
		w, s := elvenmereHeldItems(t, saved)
		weekly, season = weekly+w, season+s
	}
	if weekly != 5 || season != 1 {
		t.Fatal("concurrent account reward duplicated", weekly, season)
	}
}

func TestElvenmereMalformedLegacyReceiptFailsClosed(t *testing.T) {
	w, f := elvenmereRewardFixture(t, false)
	ctx := context.Background()
	saved, _, err := w.store.CommitCharacterEvent(ctx, w.account, w.role.ID, w.role.ConfigVersion, "elvenmere-floor:malformed:5", "elvenmere-floor-v1", func(cur database.Character) (json.RawMessage, json.RawMessage, error) {
		return cur.State, json.RawMessage(`{"grant":{"Run":"malformed","Floor":5,"WeeklyItem":10336354,"WeeklyCount":5}}`), nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if plan, err := w.elvenmereTeleport(make([]byte, 25)); err == nil || len(plan) != 0 || w.activeDungeon.Extra != 5 {
		t.Fatal("malformed receipt was ignored", plan, err)
	}
	after, err := w.store.AdminCharacter(ctx, w.role.ID)
	if err != nil || string(after.State) != string(saved.State) {
		t.Fatal("malformed history changed state", err)
	}
	if n, err := f.EventCount(ctx, w.role.ID); err != nil || n != 1 {
		t.Fatal("malformed history paid reward", n, err)
	}
}
