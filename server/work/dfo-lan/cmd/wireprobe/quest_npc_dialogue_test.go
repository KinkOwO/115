package main

import (
	"bytes"
	"context"
	"dfolan/internal/catalog"
	"dfolan/internal/catalog/pvf"
	"dfolan/internal/database"
	"dfolan/internal/quest"
	"dfolan/internal/world"
	"encoding/hex"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
)

// The NPC and dialogue request are the witnessed quest3192 source shape.
// Range quests are synthetic controls for the same position consumer.
func npcDialogueFixture(t *testing.T) (*worldSession, *database.Store) {
	t.Helper()
	ctx := context.Background()
	store, err := database.Open(ctx, database.Config{Driver: database.DriverSQLite, SQLitePath: filepath.Join(t.TempDir(), "dialogue.sqlite")})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { store.Close() })
	if err = store.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	if err = store.MigrateQuests(ctx); err != nil {
		t.Fatal(err)
	}
	account, err := store.DevelopmentAccount(ctx, "dialogue-fixture")
	if err != nil {
		t.Fatal(err)
	}
	source := pvf.ArchiveSnapshot{Checksum: strings.Repeat("a", 64)}
	role, err := store.CreateCharacter(ctx, database.Character{AccountID: account, Name: "DialogueFixture", ConfigVersion: source.SaveIdentity(), State: json.RawMessage(`{"level":22,"future":{"keep":true},"inventory":{"version":"ordinary-bag-v1","items":[{"slot":95,"Template":10164777,"Amount":1}]}}`), Request: []byte{0, 255}}, 24)
	if err != nil {
		t.Fatal(err)
	}
	number := func(n int32) pvf.Token { return pvf.Token{Type: 0, Value: n} }
	header := func(s string) pvf.Token { return pvf.Token{Type: 3, Text: s} }
	defs := map[uint32]catalog.QuestDefinition{
		3192: {ID: 3192, MinimumLevel: 1, MaximumLevel: 100, Kind: "[meet npc]", ObjectiveCells: []pvf.Token{number(553)}, Script: catalog.ScriptRecord{Path: "contents/2022/new_scenario_renewal/season_1/skycastle/quest/skycastle_22.qst"}},
		9003: {ID: 9003, MinimumLevel: 1, MaximumLevel: 100, Kind: "[seek n meet npc]", ObjectiveCells: []pvf.Token{number(10164777), number(1), number(553)}},
		9001: {ID: 9001, MinimumLevel: 1, MaximumLevel: 100, Kind: "[reach the range]", ObjectiveCells: []pvf.Token{number(553), number(180), number(180)}, Script: catalog.ScriptRecord{Cells: []pvf.Token{header("[sub type]"), number(0)}}},
		9002: {ID: 9002, MinimumLevel: 1, MaximumLevel: 100, Kind: "[reach the range]", ObjectiveCells: []pvf.Token{number(39), number(0), number(1280), number(100), number(200), number(200)}, Script: catalog.ScriptRecord{Cells: []pvf.Token{header("[sub type]"), number(1)}}},
	}
	for id, def := range defs {
		initial, model, err := quest.InitialProgress(def)
		if err != nil {
			t.Fatal(err)
		}
		if _, err = store.AcceptQuestGroups(ctx, account, role.ID, uint16(id), source.SaveIdentity(), 1, 100, nil, initial, model); err != nil {
			t.Fatal(err)
		}
	}
	qs := &quest.Service{Store: store, Catalog: catalog.QuestCatalog{Source: source, Quests: defs}}
	area := catalog.WorldArea{Town: 39, Area: 0, Map: catalog.ScriptRecord{Cells: []pvf.Token{header("[NPC]"), number(553), {Type: 6, Text: "[right]"}, number(1324), number(149), number(0)}}}
	svc := &world.Service{Store: store, Catalog: catalog.WorldCatalog{Source: source, Areas: map[string]catalog.WorldArea{catalog.AreaKey(39, 0): area}}}
	w := &worldSession{store: store, service: svc, quests: qs, role: role, account: account, level: 22, state: database.WorldState{Position: database.WorldPosition{Town: 39, Area: 0, X: 1324, Y: 149}}}
	return w, store
}

func dialogueProgress(t *testing.T, w *worldSession, store *database.Store) map[uint16]uint32 {
	t.Helper()
	rows, err := store.Quests(context.Background(), w.role.AccountID, w.role.ID)
	if err != nil {
		t.Fatal(err)
	}
	progress := make(map[uint16]uint32)
	for _, q := range rows {
		progress[q.ID] = q.Progress
	}
	return progress
}

func TestNPCDialogueWaitsForInteractionAndKeepsRangeObjectives(t *testing.T) {
	for _, variant := range []string{"upstream-store"} {
		t.Run(variant, func(t *testing.T) {
			w, store := npcDialogueFixture(t)
			at := w.state.Position
			for i := 0; i < 2; i++ {
				_, err := w.quests.ProximityProgress(context.Background(), w.role, quest.Position{Town: at.Town, Area: at.Area, X: at.X, Y: at.Y}, func(npc uint32) ([2]uint16, bool) {
					return w.service.NPCPosition(at, npc)
				}, nil)
				if err != nil {
					t.Fatal(err)
				}
				got := dialogueProgress(t, w, store)
				if got[3192] != 1 || got[9003] != 1 || got[9001] != 0 || got[9002] != 0 {
					t.Fatalf("position update %d: dialogue/range progress=%v", i, got)
				}
			}
		})
	}
}

func TestNPCDialogueCMD33CommitsAndRepliesBeforeSnapshots(t *testing.T) {
	for _, variant := range []string{"upstream-store"} {
		t.Run(variant, func(t *testing.T) {
			w, store := npcDialogueFixture(t)
			request, _ := hex.DecodeString("2100780c000000000000000000000000") // GF frame29768.
			w.state.Position.Area = 1
			if _, err := w.questInteraction(request); err == nil {
				t.Fatal("dialogue completed without its source NPC")
			}
			if got := dialogueProgress(t, w, store); got[3192] != 1 {
				t.Fatal("refused request changed dialogue progress", got)
			}
			w.state.Position.Area = 0
			for i := 0; i < 2; i++ {
				plan, err := w.questInteraction(request)
				if err != nil {
					t.Fatal(err)
				}
				want, _ := hex.DecodeString("01780c00000000") // GF frame29769, seven logical bytes.
				if len(plan) == 0 || plan[0].Kind != 1 || plan[0].ID != 33 || !bytes.Equal(plan[0].Payload, want) {
					t.Fatalf("request %d lacks same-command reply before snapshots: %+v", i, plan)
				}
				got := dialogueProgress(t, w, store)
				if got[3192] != 0 || got[9003] != 1 || got[9001] != 1 || got[9002] != 1 {
					t.Fatal("dialogue affected unrelated objectives", got)
				}
			}
		})
	}
}
