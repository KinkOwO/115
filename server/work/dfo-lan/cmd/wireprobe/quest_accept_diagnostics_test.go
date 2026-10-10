package main

import (
	"context"
	"dfolan/internal/catalog"
	"dfolan/internal/catalog/pvf"
	"dfolan/internal/database"
	"dfolan/internal/game/wire"
	"dfolan/internal/quest"
	"encoding/json"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestQuestAcceptCompletedReplayAndMigrationDiagnosticsPreserveState(t *testing.T) {
	for _, tc := range []struct {
		name, status, version, model, kind, message string
	}{
		{"completed-graduation", "completed", "current", "", "quest_accept_already_completed", ""},
		{"completed-old-source", "completed", "old", "", "quest_rejected", "quest requires source configuration migration"},
		{"accepted-old-source", "accepted", "old", quest.SingleMeetNPC, "quest_rejected", "quest requires source configuration migration"},
		{"accepted-old-progress", "accepted", "current", "old-objective-model", "quest_rejected", "quest requires progress model migration"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			fixture, err := database.OpenTestFixture(ctx)
			if err != nil {
				t.Fatal(err)
			}
			defer fixture.Close()
			store := fixture.Storage()
			account, err := store.DevelopmentAccount(ctx, "quest-diagnostic-fixture")
			if err != nil {
				t.Fatal(err)
			}
			source := pvf.ArchiveSnapshot{}
			version := source.SaveIdentity()
			role, err := store.CreateCharacter(ctx, database.Character{AccountID: account, Name: "ReplayFixture", ConfigVersion: version, Request: []byte{}, State: json.RawMessage(`{"level":115,"experience":123,"unknown":{"keep":true}}`)}, 24)
			if err != nil {
				t.Fatal(err)
			}
			storedVersion := version
			if tc.version == "old" {
				storedVersion = strings.Repeat("a", 64)
			}
			if tc.status == "completed" {
				_, err = store.ClearQuests(ctx, account, role.ID, storedVersion, []uint16{3145})
			} else {
				_, err = store.AcceptQuest(ctx, account, role.ID, 3145, storedVersion, 1, 200, nil, 1, tc.model)
			}
			if err != nil {
				t.Fatal(err)
			}
			before, err := store.Quests(ctx, account, role.ID)
			if err != nil {
				t.Fatal(err)
			}
			beforeEvents, err := fixture.EventCount(ctx, role.ID)
			if err != nil {
				t.Fatal(err)
			}
			client, conn, events := newDispatchTestClient()
			client.worldState.role = role
			client.questService = &quest.Service{Store: store, Catalog: catalog.QuestCatalog{Source: source, Quests: map[uint32]catalog.QuestDefinition{3145: {ID: 3145, MinimumLevel: 1, MaximumLevel: 200, Kind: "[meet npc]", ObjectiveCells: []pvf.Token{{Type: 0, Value: 1}}}}}}
			// The same verified C31 body observed during entry. Replays remain refusals;
			// diagnostics must not manufacture a success packet or alter quest state.
			for i := 0; i < 3; i++ {
				result := client.dispatchWorldAndQuests(&clientRequest{frame: wire.Frame{Type: 1, ID: 31}, verified: true, plaintext: []byte{31, 0, 0x49, 0x0c}})
				if result != dispatchHandled || conn.Len() != 0 {
					t.Fatal("rejected acceptance changed wire behavior", result, conn.Len())
				}
				if len(*events) != i+1 {
					t.Fatal("unexpected extra diagnostic or success", len(*events))
				}
				event := (*events)[i]
				if event["kind"] != tc.kind || event["quest"] != uint16(3145) || event["operation"] != uint16(31) {
					t.Fatal("wrong diagnostic classification", event)
				}
				if tc.message == "" {
					if _, ok := event["error"]; ok {
						t.Fatal("completed replay reported as migration failure", event)
					}
				} else if event["error"] != tc.message {
					t.Fatal("migration was obscured", event)
				}
			}
			after, err := store.Quests(ctx, account, role.ID)
			if err != nil || !reflect.DeepEqual(before, after) {
				t.Fatal("rejected replay mutated saved quests", err)
			}
			saved, err := store.AdminCharacter(ctx, role.ID)
			if err != nil || !reflect.DeepEqual(saved.State, role.State) {
				t.Fatal("rejected replay mutated assets/state", err)
			}
			afterEvents, err := fixture.EventCount(ctx, role.ID)
			if err != nil || afterEvents != beforeEvents {
				t.Fatal("rejected replay recorded an asset transaction", err)
			}
		})
	}
}
