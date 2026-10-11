package main

import (
	"context"
	"dfolan/internal/boostup"
	"dfolan/internal/catalog"
	"dfolan/internal/catalog/pvf"
	"dfolan/internal/character"
	"dfolan/internal/database"
	"dfolan/internal/gamedata"
	"dfolan/internal/inventory"
	"dfolan/internal/loot"
	"dfolan/internal/quest"
	"dfolan/internal/workflow"
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"
)

// Uses the current archive and disposable SQLite, never runtime/storage.
func TestBoostGraduationCurrentPVFSQLite(t *testing.T) {
	path := os.Getenv("DFO_HANDOFF_PVF")
	if path == "" {
		t.Skip("requires current read-only PVF")
	}
	source, err := gamedata.Open(gamedata.Options{Mode: gamedata.PVF, ArchivePath: path})
	must115(t, err)
	defer source.Close()
	// Import just this feature's dependencies through the shared Source; other
	// independently evolving gateway domains are outside this regression.
	index, err := source.ItemIndex("")
	must115(t, err)
	quests, err := source.Quests("")
	must115(t, err)
	skip, err := source.QuestSkipRules()
	must115(t, err)
	professions, err := source.Characters("")
	must115(t, err)
	progression, err := source.Progression("")
	must115(t, err)
	full, err := source.Equipment(index)
	must115(t, err)
	defer full.Close()
	must115(t, source.EnableRuntimeDetails(&quests, nil, nil, nil))
	defer quests.Close()
	c := &gamedata.Catalogs{Items: &index, Quests: &quests, QuestSkip: &skip, Characters: &professions, Progression: &progression, Equipment: full, SourceChecksum: source.Snapshot().Checksum}
	_, content, err := source.BoostUp(*c.Items)
	must115(t, err)
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	fixture, err := database.OpenTestFixture(ctx)
	must115(t, err)
	defer fixture.Close()
	store := fixture.Storage()
	for _, migrate := range []func(context.Context) error{store.Migrate, store.MigrateQuests, store.MigrateQuestObjectives, store.MigrateQuestRewards, store.MigrateCharacterEvents} {
		must115(t, migrate(ctx))
	}
	account, err := store.DevelopmentAccount(ctx, "boost-mainline-test")
	must115(t, err)
	var profession byte
	for id, p := range c.Characters.Professions {
		if p.Job == "[archer]" {
			profession = id
			break
		}
	}
	if profession == 0 {
		t.Fatal("source archer absent")
	}
	rules, err := inventory.LoadBagRules("../../configs/inventory.next29.json", c.SourceChecksum)
	must115(t, err)
	items := catalogLootForMainline(c)
	equipment := &inventory.EquipmentCatalog{Source: c.Items.Source, Full: c.Equipment}
	service, err := newRuntimeQuestService(quest.Service{Store: store, Catalog: *c.Quests, Professions: *c.Characters, Inventory: &inventory.Awarder{Catalog: items, Equipment: equipment, Rules: rules}, Progression: &character.ProgressionService{Catalog: *c.Progression, Rules: character.GrowthRules{Model: "boost-mainline-native-test"}}}, c.QuestSkip)
	must115(t, err)
	flow := &workflow.QuestService{Store: store, Quest: service}
	create := func(name string, training boostup.Training) database.Character {
		t.Helper()
		if training.Step == 0 {
			training.Step = 1
			if training.Finished {
				training.Step = byte(len(content.Steps) + 1)
			}
		}
		state, e := inventory.SaveBag(json.RawMessage(`{"level":115,"advancement":3,"awakening":2,"unknown":{"keep":true}}`), inventory.Bag{Version: "ordinary-bag-v1", Gold: 12345})
		must115(t, e)
		state, e = boostup.WriteState(state, boostup.State{Version: 1, Activated: true, Training: training})
		must115(t, e)
		r, e := store.CreateCharacter(ctx, database.Character{AccountID: account, Name: name, Profession: profession, ConfigVersion: c.Quests.Source.SaveIdentity(), Request: []byte{0}, State: state}, 24)
		must115(t, e)
		r.WireID = 37
		return r
	}
	finished := create("OldGraduate", boostup.Training{Step: byte(len(content.Steps) + 1), Finished: true})
	_, err = service.Accept(ctx, finished, 3145)
	must115(t, err)
	result, err := flow.ReconcileBoostMainline(ctx, finished, content)
	must115(t, err)
	if len(result.Settled) == 0 || len(result.KeyItems) != len(c.QuestSkip.Items) || result.Role.WireID != finished.WireID {
		t.Fatalf("graduation not settled: quests=%d keys=%v wire=%d", len(result.Settled), result.KeyItems, result.Role.WireID)
	}
	firstCompleted := false
	for _, row := range result.Settled {
		if row.Receipt.Quest == 3145 {
			firstCompleted = true
		} // Screenshot's stuck quest, assertion only.
		if c.QuestSkip.Blocked[uint32(row.Receipt.Quest)] || row.Receipt.Gold != 0 || row.Receipt.Experience != 0 {
			t.Fatalf("source exclusion or zero-currency contract violated: %+v", row.Receipt)
		}
	}
	if !firstCompleted {
		t.Fatal("screenshot's first mainline remains uncleared")
	}
	bag, err := inventory.ReadBag(result.Role.State)
	must115(t, err)
	if bag.Gold != 12345 {
		t.Fatal("graduation changed gold")
	}
	var kept struct {
		Level   byte
		Unknown struct{ Keep bool }
		Boost   boostup.State `json:"boost_up115"`
	}
	must115(t, json.Unmarshal(result.Role.State, &kept))
	if kept.Level != content.GoalLevel || !kept.Unknown.Keep || !kept.Boost.Training.Finished {
		t.Fatal("save fields lost")
	}
	repeat, err := flow.ReconcileBoostMainline(ctx, finished, content) // Deliberately stale role.
	must115(t, err)
	if len(repeat.Settled) != 0 || len(repeat.KeyItems) != 0 {
		t.Fatal("relogin duplicated settlement/rewards")
	}
	t.Logf("source=%s settled=%d keys=%v stopped=%s blocked=%v", c.SourceChecksum, len(result.Settled), result.KeyItems, result.Stopped, result.Blocked)

	manual := create("ManualSkip", boostup.Training{})
	untouched, err := flow.ReconcileBoostMainline(ctx, manual, content)
	must115(t, err)
	if len(untouched.Settled) != 0 {
		t.Fatal("unfinished training auto-cleared story")
	}
	manualResult, err := flow.SkipMainlineNow(ctx, manual, "native-test", "manual button", 0)
	must115(t, err)
	if len(manualResult.Settled) != len(result.Settled) {
		t.Fatal("manual runtime rules differ from graduation")
	}

	blocked := create("RetryGraduate", boostup.Training{Finished: true})
	awarder := service.Inventory
	fullRules := rules
	fullRules.Slots = map[string][2]uint16{}
	for kind := range rules.Slots {
		fullRules.Slots[kind] = [2]uint16{65, 65}
	}
	for _, id := range c.QuestSkip.Items {
		fullRules.Slots[strings.ToLower(strings.TrimSpace(items.Items[id].StackableType))] = [2]uint16{65, 65}
	}
	service.Inventory = &inventory.Awarder{Catalog: items, Equipment: equipment, Rules: fullRules}
	if _, err = flow.ReconcileBoostMainline(ctx, blocked, content); err == nil || !strings.Contains(err.Error(), "full") {
		t.Fatalf("key-item bag-full was not rejected: %v", err)
	}
	done, err := service.Completed(ctx, blocked)
	must115(t, err)
	if len(done) != 0 {
		t.Fatal("failed settlement partially committed quests")
	}
	service.Inventory = awarder
	retry, err := flow.ReconcileBoostMainline(ctx, blocked, content)
	must115(t, err)
	if len(retry.Settled) != len(result.Settled) {
		t.Fatal("failed graduation could not retry")
	}

	// Exercise the actual final-claim gateway with a one-step training fixture;
	// story rules, source goal and DB remain the real runtime dependencies.
	final := create("FinalClaim", boostup.Training{Step: 1, Phase: 1})
	training := &boostup.Catalog{GoalLevel: content.GoalLevel, Town: content.Town, Steps: []boostup.Step{{Number: 1, Guide: "normal", Mission: "none", Rewards: []boostup.Reward{{Item: c.QuestSkip.Items[0], Count: 1}}}}}
	w := &worldSession{account: account, role: final, level: content.GoalLevel, store: store, quests: service, loot: &loot.Service{Catalog: items, BagRules: rules, Equipment: equipment}, boostup: training, state: database.WorldState{Position: database.WorldPosition{Town: content.Town}}}
	plan, err := w.boostStepRequest(ctx, []byte{0x96, 2, 0, 0, 1, 0, 0, 0}, 680)
	must115(t, err)
	if countPacket(plan, 342) != 1 || countPacket(plan, 291) != 1 || countPacket(plan, 21) != 1 || countPacket(plan, 680) != 1 {
		t.Fatalf("final claim omitted quest snapshots: %+v", plan)
	}
	if countPacket(plan, 34) != 0 || countPacket(plan, 1422) != 0 {
		t.Fatal("unsolicited command replies during graduation")
	}
	if n := len(plan); n < 3 || plan[n-3].ID != 2639 || plan[n-2].ID != 2638 || plan[n-1].ID != 680 {
		t.Fatal("native final claim order changed")
	}
}

func TestRuntimeQuestServiceRequiresMatchingSkipSource(t *testing.T) {
	if _, err := newRuntimeQuestService(quest.Service{}, nil); err == nil {
		t.Fatal("missing runtime rules accepted")
	}
	if _, err := newRuntimeQuestService(quest.Service{Catalog: catalog.QuestCatalog{Source: pvf.ArchiveSnapshot{Checksum: "source-a"}}}, &catalog.QuestSkip{Source: pvf.ArchiveSnapshot{Checksum: "source-b"}}); err == nil {
		t.Fatal("mismatched runtime rules accepted")
	}
}
