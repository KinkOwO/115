package workflow

import (
	"context"
	"dfolan/internal/character"
	"dfolan/internal/database"
	"encoding/json"
	"fmt"
	"github.com/stretchr/testify/require"
	"strings"
	"testing"
	"time"
)

func TestWeeklyClearBonusCommitsWithReceiptAndRollsBackTogether(t *testing.T) {
	s, role, _ := routineServiceFixture(t)
	s.Rules.SecondsPerProperClear = 30
	now := time.Date(2026, 10, 8, 8, 0, 0, 0, time.UTC)
	ctx := context.Background()
	require.NoError(t, s.Store.MigrateAdventure(ctx))
	require.NoError(t, s.BindCommittedClears(now.Location(), 6))
	_, err := s.Run(ctx, role, RoutineAction{From: now.Add(-20 * time.Second), Now: now, GameDay: "2026-10-08", ResetHour: 6, ObservationKey: "before-clear"})
	require.NoError(t, err)
	run := strings.Repeat("2", 32)
	receipt := character.ClearReceipt{Source: role.ConfigVersion, Run: run, RecommendedDungeonClear: true, DungeonID: 1001, CharacterLevel: 115, CompletedAt: now.Add(5 * time.Second)}
	broken := true
	require.NoError(t, s.Store.RegisterCharacterEventAccountObserver(func(_ database.Character, key string, _ json.RawMessage, _ *database.AccountAdventure) (bool, error) {
		if key == "clear:"+run && broken {
			return false, fmt.Errorf("injected transaction failure after bonus")
		}
		return false, nil
	}))
	calls := 0
	commit := func() (database.Character, bool, error) {
		return s.Store.CommitCharacterEvent(ctx, role.AccountID, role.ID, role.ConfigVersion, "clear:"+run, "weekly-clear-fixture", func(current database.Character) (json.RawMessage, json.RawMessage, error) {
			calls++
			raw, e := json.Marshal(receipt)
			return current.State, raw, e
		})
	}
	_, applied, err := commit()
	require.Error(t, err)
	require.False(t, applied)
	_, err = s.Store.CharacterEventReceipt(ctx, role.AccountID, role.ID, "clear:"+run)
	require.ErrorIs(t, err, database.ErrNotFound)
	load := func() uint64 {
		p, e := s.Store.LoadAdventure(ctx, role.AccountID, role.ID, role.Name)
		require.NoError(t, e)
		progress, e := routineProgress(p, s.Rules.CampaignKey())
		require.NoError(t, e)
		require.Equal(t, now, progress.HighWater)
		return progress.PeriodMillis[1]
	}
	require.EqualValues(t, 20000, load())
	broken = false
	_, applied, err = commit()
	require.NoError(t, err)
	require.True(t, applied)
	require.EqualValues(t, 50000, load())
	// Simulate the missing notification/reconnect: replay cannot add the bonus.
	_, applied, err = commit()
	require.NoError(t, err)
	require.False(t, applied)
	require.Equal(t, 2, calls)
	require.EqualValues(t, 50000, load())
	r, err := s.Run(ctx, role, RoutineAction{From: now, Now: now.Add(5 * time.Second), GameDay: "2026-10-08", ResetHour: 6, ObservationKey: "after-clear"})
	require.NoError(t, err)
	require.EqualValues(t, 55000, r.Progress.PeriodMillis[1])
}

func TestWeeklyClearBonusRejectsMissingDateAndPreservesOtherAccountFields(t *testing.T) {
	s, role, _ := routineServiceFixture(t)
	s.Rules.SecondsPerProperClear = 10
	ctx := context.Background()
	now := time.Date(2026, 10, 8, 8, 0, 0, 0, time.UTC)
	require.NoError(t, s.Store.MigrateAdventure(ctx))
	require.NoError(t, s.BindCommittedClears(now.Location(), 6))
	campaign := s.Rules.CampaignKey()
	_, _, _, err := s.Store.CommitAdventureTx(ctx, role.AccountID, role.ID, "seed-clear-extensions", func(_ *database.Tx, current database.Character, p *database.AccountAdventure) (json.RawMessage, json.RawMessage, error) {
		p.Data.Attendance = map[string]json.RawMessage{campaign: json.RawMessage(`{"version":0,"extension":{"preserved":true}}`), "other": json.RawMessage(`{"intact":17}`)}
		return current.State, json.RawMessage(`{}`), nil
	})
	require.NoError(t, err)
	run := strings.Repeat("3", 32)
	receipt := character.ClearReceipt{Source: role.ConfigVersion, Run: run, RecommendedDungeonClear: true, DungeonID: 1001, CharacterLevel: 115}
	commit := func() (database.Character, bool, error) {
		return s.Store.CommitCharacterEvent(ctx, role.AccountID, role.ID, role.ConfigVersion, "clear:"+run, "weekly-clear-fixture", func(current database.Character) (json.RawMessage, json.RawMessage, error) {
			raw, e := json.Marshal(receipt)
			return current.State, raw, e
		})
	}
	_, _, err = commit()
	require.ErrorContains(t, err, "dated source receipt")
	receipt.CompletedAt = now
	_, applied, err := commit()
	require.NoError(t, err)
	require.True(t, applied)
	p, err := s.Store.LoadAdventure(ctx, role.AccountID, role.ID, role.Name)
	require.NoError(t, err)
	require.JSONEq(t, `{"intact":17}`, string(p.Data.Attendance["other"]))
	var fields map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(p.Data.Attendance[campaign], &fields))
	require.JSONEq(t, `{"preserved":true}`, string(fields["extension"]))
	progress, err := routineProgress(p, campaign)
	require.NoError(t, err)
	require.EqualValues(t, 10000, progress.PeriodMillis[1])
}
