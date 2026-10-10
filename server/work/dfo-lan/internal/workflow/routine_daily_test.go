package workflow

import (
	"context"
	"dfolan/internal/activity"
	"dfolan/internal/character"
	"dfolan/internal/database"
	"encoding/json"
	"fmt"
	"github.com/stretchr/testify/require"
	"testing"
	"time"
)

func TestSequentialDailyClearAtomicRollbackAndCrossRoleReplay(t *testing.T) {
	s, a, b := routineServiceFixture(t)
	reward := s.Rules.Weekly[0].Reward
	s.Rules.Kind = "daily"
	s.Rules.SequentialDaily = true
	s.Rules.ProperDungeonCount = 2
	s.Rules.Weekly = nil
	s.Rules.Daily = []activity.Milestone{{Index: 1, Reward: reward}, {Index: 2, Reward: reward}}
	ctx := context.Background()
	now := time.Date(2026, 10, 8, 8, 0, 0, 0, time.UTC)
	require.NoError(t, s.Store.MigrateAdventure(ctx))
	require.NoError(t, s.BindSequentialClears(time.UTC, 6))
	broken := true
	require.NoError(t, s.Store.RegisterCharacterEventAccountObserver(func(_ database.Character, key string, _ json.RawMessage, _ *database.AccountAdventure) (bool, error) {
		if key == "clear:daily-rollback" && broken {
			return false, fmt.Errorf("injected failure")
		}
		return false, nil
	}))
	commit := func(role database.Character, run string, at time.Time) (bool, error) {
		receipt := character.ClearReceipt{Source: role.ConfigVersion, Run: run, RecommendedDungeonClear: true, DungeonID: 1001, CharacterLevel: 115, CompletedAt: at}
		_, applied, e := s.Store.CommitCharacterEvent(ctx, role.AccountID, role.ID, role.ConfigVersion, "clear:"+run, "daily-test", func(current database.Character) (json.RawMessage, json.RawMessage, error) {
			raw, e := json.Marshal(receipt)
			return current.State, raw, e
		})
		return applied, e
	}
	_, e := commit(a, "daily-rollback", now)
	require.Error(t, e)
	broken = false
	applied, e := commit(a, "daily-rollback", now)
	require.NoError(t, e)
	require.True(t, applied)
	applied, e = commit(a, "daily-rollback", now)
	require.NoError(t, e)
	require.False(t, applied)
	_, e = commit(b, "daily-other-role", now.Add(time.Second))
	require.NoError(t, e)
	result, e := s.Run(ctx, a, RoutineAction{From: now.Add(2 * time.Second), Now: now.Add(2 * time.Second), GameDay: "2026-10-08", ResetHour: 6, ObservationKey: "view"})
	require.NoError(t, e)
	require.True(t, s.Rules.DailyEligible(result.Progress, 1))
	require.Zero(t, result.Progress.Clears[2])
	for i, role := range []database.Character{a, b} {
		result, e = s.Run(ctx, role, RoutineAction{From: now.Add(3 * time.Second), Now: now.Add(3 * time.Second), GameDay: "2026-10-08", ResetHour: 6, ObservationKey: fmt.Sprint("claim", i), ClaimKind: "day", ClaimIndex: 1})
		require.NoError(t, e)
		require.Equal(t, i == 0, result.CreatedMail)
	}
	ma, e := s.Store.Mailbox(ctx, a.AccountID, a.ID)
	require.NoError(t, e)
	mb, e := s.Store.Mailbox(ctx, b.AccountID, b.ID)
	require.NoError(t, e)
	require.Equal(t, 1, len(ma)+len(mb))
}
