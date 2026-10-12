package main

import (
	"context"
	"dfolan/internal/catalog"
	"dfolan/internal/catalog/pvf"
	"dfolan/internal/character"
	"dfolan/internal/database"
	"dfolan/internal/dungeon"
	"dfolan/internal/game/protocol"
	"dfolan/internal/game/wire"
	"dfolan/internal/inventory"
	"dfolan/internal/loot"
	"dfolan/internal/quest"
	"dfolan/internal/testfixture"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func townArrivalCompletionClient(t *testing.T) (*gameConnection, *[]map[string]any) {
	t.Helper()
	client, events := townArrivalLoadingClient(t)
	w := client.worldState
	version := (pvf.ArchiveSnapshot{}).SaveIdentity()
	err := w.store.MigrateQuests(context.Background())
	require.NoError(t, err)
	_, err = w.store.MigrateSaveIdentity(context.Background(), version)
	require.NoError(t, err)
	w.role.ConfigVersion = version
	number := func(n int32) pvf.Token { return pvf.Token{Type: 0, Value: n} }
	header := func(s string) pvf.Token { return pvf.Token{Type: 3, Text: s} }
	// The queen's source shape: a cinematic quest, an arrival rule for town
	// 39/1 and a quest-bound scenario maze. Run identity is disposable.
	d := catalog.QuestDefinition{ID: 21029, MinimumLevel: 1, MaximumLevel: 115,
		Kind: "[look cinematic]", ObjectiveCells: []pvf.Token{number(21000)},
		Script: catalog.ScriptRecord{Cells: []pvf.Token{
			header("[move to dungeon]"), {Type: 6, Text: "arrive in town"},
			header("[int datas]"), number(39), number(1), header("[dungeon]"), number(100004400), header("[/move to dungeon]"),
		}}}
	source := pvf.ArchiveSnapshot{Checksum: w.role.ConfigVersion}
	w.quests = &quest.Service{Store: w.store, Catalog: catalog.QuestCatalog{Source: source, Quests: map[uint32]catalog.QuestDefinition{d.ID: d}}}
	client.questService, client.bootstrapped = w.quests, true
	initial, model, err := quest.InitialProgress(d)
	require.NoError(t, err)
	_, err = w.store.AcceptQuestGroups(context.Background(), w.account, w.role.ID, uint16(d.ID), w.role.ConfigVersion, 1, 115, nil, initial, model)
	require.NoError(t, err)
	var issues []string
	w.townArrivalScenes, issues = catalog.TownArrivalSceneWhitelist(w.quests.Catalog, catalog.WorldCatalog{
		Source: source, Areas: w.service.Catalog.Areas, Dungeons: []catalog.IndexEntry{{ID: 100004400}},
	})
	require.Empty(t, issues)
	room := catalog.DungeonRoom{Map: 100014294}
	w.pendingTownArrival = nil
	w.activeDungeon = &dungeon.Session{
		Definition: catalog.DungeonDefinition{ID: 100004400, BasisLevel: 1, NoFatigue: true},
		Maze:       catalog.DungeonMaze{Quest: 21029, Rooms: []catalog.DungeonRoom{room}}, Room: room,
		Loaded: true, RunID: strings.Repeat("b", 32), StartedAt: time.Now().Add(-time.Second),
		Visited: map[uint32][]protocol.DungeonMonster{room.Map: {}},
	}
	return client, events
}

func TestTownArrivalCinematicCompletionPrecedesQuestACKAndIsIdempotent(t *testing.T) {
	client, events := townArrivalCompletionClient(t)
	w := client.worldState
	request, err := hex.DecodeString("21002552000000000000000000000000") // GF index 19873.
	require.NoError(t, err)
	for attempt := 0; attempt < 2; attempt++ {
		start := len(*events)
		require.Equal(t, dispatchHandled, client.dispatchWorldAndQuests(&clientRequest{frame: wire.Frame{Type: 1, ID: 33}, plaintext: request, verified: true}))
		var names []any
		for _, event := range (*events)[start:] {
			require.NotEqual(t, "quest_interaction_refused", event["kind"], event)
			names = append(names, event["kind"])
		}
		if attempt == 0 {
			require.GreaterOrEqual(t, len(names), 2)
			require.Equal(t, "dungeon_clear_enabled", names[0])
			require.Equal(t, "quest_interaction_accepted", names[1])
		} else {
			require.NotContains(t, names, "dungeon_clear_enabled")
			require.Contains(t, names, "quest_interaction_accepted")
		}
		require.True(t, w.activeDungeon.Completed())
		require.True(t, w.completionSent)
		require.False(t, w.resultSent)
		require.Nil(t, w.cardPlan)
	}
}

func TestTownArrivalCinematicCompletionRejectsUnrelatedOrUnreadyRun(t *testing.T) {
	for _, tc := range []struct {
		name   string
		change func(*worldSession, *protocol.ActiveQuest)
	}{
		{"not_loaded", func(w *worldSession, _ *protocol.ActiveQuest) { w.activeDungeon.Loaded = false }},
		{"no_active_run", func(w *worldSession, _ *protocol.ActiveQuest) { w.activeDungeon = nil }},
		{"unfinished_quest", func(_ *worldSession, q *protocol.ActiveQuest) { q.Progress = 1 }},
		{"other_quest", func(_ *worldSession, q *protocol.ActiveQuest) { q.ID++ }},
		{"other_dungeon", func(w *worldSession, _ *protocol.ActiveQuest) { w.activeDungeon.Definition.ID++ }},
		{"other_maze_quest", func(w *worldSession, _ *protocol.ActiveQuest) { w.activeDungeon.Maze.Quest++ }},
		{"left_source_area", func(w *worldSession, _ *protocol.ActiveQuest) { w.state.Position.Area = 0 }},
		{"ordinary_meet", func(w *worldSession, _ *protocol.ActiveQuest) {
			d := w.quests.Catalog.Quests[21029]
			d.Kind, d.ObjectiveCells = "[meet npc]", []pvf.Token{{Type: 0, Value: 553}}
			w.quests.Catalog.Quests[21029] = d
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			client, _ := townArrivalCompletionClient(t)
			w := client.worldState
			run := w.activeDungeon
			q := protocol.ActiveQuest{ID: 21029}
			tc.change(w, &q)
			plan, err := w.completeTownArrivalCinematic(q)
			require.NoError(t, err)
			require.Empty(t, plan)
			require.False(t, run.Completed())
			require.False(t, w.completionSent)
		})
	}
	client, _ := townArrivalCompletionClient(t)
	w := client.worldState
	require.NoError(t, w.store.AbandonQuest(context.Background(), w.account, w.role.ID, 21029))
	plan, err := w.questInteraction([]byte{33, 0, 37, 82, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0})
	require.Error(t, err)
	require.Empty(t, plan)
	require.False(t, w.activeDungeon.Completed(), "unowned CMD33 must never reach scene completion")
}

// Exercise the actual result transaction, durable card freeze, dispatcher flag
// consumers and return packets; completing the helper alone cannot prove exit.
func TestTownArrivalCinematicCompletionReachesOwnedSettlementAndTown(t *testing.T) {
	client, events := townArrivalCompletionClient(t)
	w := client.worldState
	pc, err := catalog.LoadProgression(testfixture.ProgressionPath(t))
	require.NoError(t, err)
	pc.Source = w.quests.Catalog.Source
	professions, err := catalog.LoadCharacters("../../configs/characters.skycastle-release.json")
	require.NoError(t, err)
	professions.Source = pc.Source
	rules, err := character.LoadGrowthRules("../../configs/experience.compat90.json")
	require.NoError(t, err)
	w.progression = &character.ProgressionService{Store: w.store, Catalog: pc, Professions: professions, Rules: rules}
	state := character.State{SourcePath: professions.Professions[0].Path, SourceSHA256: professions.Professions[0].RawSHA256, InitialSkills: professions.Professions[0].InitialSkills, Level: 22, Experience: pc.Thresholds[20], Attributes: professions.Professions[0].InitialAttributes}
	raw, err := json.Marshal(state)
	require.NoError(t, err)
	role, _, err := w.store.CommitCharacterEvent(context.Background(), w.account, w.role.ID, w.role.ConfigVersion, "fixture:attributes", "fixture-v1",
		func(database.Character) (json.RawMessage, json.RawMessage, error) {
			return raw, json.RawMessage(`{}`), nil
		})
	require.NoError(t, err)
	role.WireID = w.role.WireID
	w.role = role
	w.loot = &loot.Service{Catalog: catalog.LootCatalog{Source: pc.Source}, Tables: loot.Tables{Gold: []float64{1, 100, 0}},
		CardPolicy: &loot.CardRules{Model: "reference90-free-gold-v1", GoldNumerator: 1, GoldDenominator: 1, Difficulty: []float64{1, 1, 1, 1, 1}}}
	w.items = &inventory.ItemService{Catalog: w.loot.Catalog}
	result := make([]byte, 113)
	result[3] = 1
	binary.LittleEndian.PutUint16(result[4:], w.role.WireID)
	exit, err := hex.DecodeString("01020100000000000000000000000000") // GF index 19882.
	require.NoError(t, err)
	_, err = w.dungeonResult(result)
	require.Error(t, err, "CMD46 before clear remains refused")
	_, _, err = w.settlementExit(exit)
	require.Error(t, err, "CMD72 before clear remains refused")
	request, err := hex.DecodeString("21002552000000000000000000000000")
	require.NoError(t, err)
	require.Equal(t, dispatchHandled, client.dispatchWorldAndQuests(&clientRequest{frame: wire.Frame{Type: 1, ID: 33}, plaintext: request, verified: true}))
	_, _, err = w.settlementExit(exit)
	require.Error(t, err, "CMD72 must still await CMD46 and its card plan")
	start := len(*events)
	require.Equal(t, dispatchHandled, client.dispatchDungeon(&clientRequest{frame: wire.Frame{Type: 1, ID: 46}, plaintext: result, verified: true}))
	for _, event := range (*events)[start:] {
		require.NotEqual(t, "dungeon_request_refused", event["kind"], event)
	}
	require.True(t, w.resultSent)
	require.NotNil(t, w.cardPlan)
	require.Equal(t, w.activeDungeon.RunID, w.cardPlan.Run)
	require.Equal(t, dispatchHandled, client.dispatchDungeon(&clientRequest{frame: wire.Frame{Type: 1, ID: 72}, plaintext: exit, verified: true}))
	var names []any
	for _, event := range (*events)[start:] {
		require.NotEqual(t, "dungeon_request_refused", event["kind"], event)
		names = append(names, event["kind"])
	}
	for _, name := range []string{"dungeon_play_result", "dungeon_clear_reward", "settlement_exit_ack", "dungeon_return_area", "dungeon_return_users"} {
		require.Contains(t, names, name)
	}
	require.Nil(t, w.activeDungeon)
	require.Equal(t, uint32(39), w.state.Position.Town)
	require.Equal(t, uint32(1), w.state.Position.Area)
}
