package database

import (
	"context"
	"dfolan/internal/quest"
	"encoding/json"
	"github.com/stretchr/testify/require"
	"strings"
	"testing"
	"time"
)

func TestTownArrivalCinematicMigrationUsesAcceptedLifetimeAndBoundClear(t *testing.T) {
	for _, mode := range []string{"unviewed", "clear-only", "wrong-dungeon", "wrong-run", "wrong-map", "wrong-source", "completed-scene", "old-acceptance", "completed-quest"} {
		t.Run(mode, func(t *testing.T) {
			s, _, role, version := sqliteAbsenceStore(t)
			ctx := context.Background()
			const id = 21029
			_, err := s.AcceptQuestGroups(ctx, role.AccountID, role.ID, id, version, 1, 115, nil, 0, quest.LookCinematic)
			require.NoError(t, err)
			time.Sleep(2 * time.Millisecond) // SQLite's historical receipt timestamp has millisecond resolution.
			run := strings.Repeat("c", 32)
			if mode != "unviewed" {
				mapID, clearSource := uint32(100014294), version
				if mode == "wrong-map" {
					mapID++
				}
				if mode == "wrong-source" {
					clearSource = strings.Repeat("e", 64)
				}
				_, err = s.RecordQuestMapClear(ctx, role.AccountID, role.ID, run, mapID, clearSource, "single", nil)
				require.NoError(t, err)
			}
			if mode != "unviewed" && mode != "clear-only" {
				dg := 100004400
				if mode == "wrong-dungeon" {
					dg++
				}
				proofRun := run
				if mode == "wrong-run" {
					proofRun = strings.Repeat("d", 32)
				}
				proof, err := json.Marshal(map[string]any{"Run": proofRun, "dungeon_id": dg})
				require.NoError(t, err)
				_, _, err = s.CommitCharacterEvent(ctx, role.AccountID, role.ID, role.ConfigVersion, "clear:"+run, "fixture-clear", func(c Character) (json.RawMessage, json.RawMessage, error) { return c.State, proof, nil })
				require.NoError(t, err)
			}
			if mode == "old-acceptance" {
				time.Sleep(2 * time.Millisecond)
				require.NoError(t, s.AbandonQuest(ctx, role.AccountID, role.ID, id))
				_, err = s.AcceptQuestGroups(ctx, role.AccountID, role.ID, id, version, 1, 115, nil, 0, quest.LookCinematic)
				require.NoError(t, err)
			}
			if mode == "completed-quest" {
				_, err = s.CommitQuestReward(ctx, role.AccountID, role.ID, id, version, quest.LookCinematic, "fixture-reward", func(c Character) (json.RawMessage, json.RawMessage, error) {
					return c.State, json.RawMessage(`{}`), nil
				})
				require.NoError(t, err)
			}
			for i := 0; i < 2; i++ {
				require.NoError(t, s.MigrateTownArrivalCinematic(ctx, role.AccountID, role.ID, id, version, quest.LookCinematic, quest.TownArrivalCinematic, 100004400, []uint32{100014294}))
			}
			states, err := s.Quests(ctx, role.AccountID, role.ID)
			require.NoError(t, err)
			require.Len(t, states, 1)
			if mode == "completed-quest" {
				require.Equal(t, "completed", states[0].Status)
				require.Equal(t, quest.LookCinematic, states[0].ProgressModel)
			} else {
				require.Equal(t, quest.TownArrivalCinematic, states[0].ProgressModel)
				want := uint32(1)
				if mode == "completed-scene" {
					want = 0
				}
				require.Equal(t, want, states[0].Progress)
			}
		})
	}
}

func TestTownArrivalCinematicCommitUsesNativeOwnerAndAcceptedLifetime(t *testing.T) {
	for _, mode := range []string{"success", "other-account", "wrong-source", "wrong-model", "run-before-accept", "reaccepted", "replay-before-accept"} {
		t.Run(mode, func(t *testing.T) {
			s, account, role, version := sqliteAbsenceStore(t)
			ctx := context.Background()
			_, err := s.AcceptQuest(ctx, account, role.ID, 21029, version, 1, 115, nil, 1, quest.TownArrivalCinematic)
			require.NoError(t, err)
			time.Sleep(2 * time.Millisecond)
			started := time.Now()
			model := quest.TownArrivalCinematic
			if mode == "reaccepted" {
				time.Sleep(2 * time.Millisecond)
				require.NoError(t, s.AbandonQuest(ctx, account, role.ID, 21029))
				_, err = s.AcceptQuest(ctx, account, role.ID, 21029, version, 1, 115, nil, 1, model)
				require.NoError(t, err)
			}
			if mode == "replay-before-accept" {
				changed, err := s.CompleteTownArrivalCinematic(ctx, account, role.ID, 21029, version, model, started)
				require.NoError(t, err)
				require.True(t, changed)
			}
			switch mode {
			case "other-account":
				account++
			case "wrong-source":
				version = strings.Repeat("e", 64)
			case "wrong-model":
				model = quest.LookCinematic
			case "run-before-accept", "replay-before-accept":
				started = started.Add(-time.Hour)
			}
			changed, err := s.CompleteTownArrivalCinematic(ctx, account, role.ID, 21029, version, model, started)
			if mode != "success" {
				require.Error(t, err)
				require.False(t, changed)
				return
			}
			require.NoError(t, err)
			require.True(t, changed)
			changed, err = s.CompleteTownArrivalCinematic(ctx, account, role.ID, 21029, version, model, started)
			require.NoError(t, err)
			require.False(t, changed)
			states, err := s.Quests(ctx, account, role.ID)
			require.NoError(t, err)
			require.Zero(t, states[0].Progress)
		})
	}
}
