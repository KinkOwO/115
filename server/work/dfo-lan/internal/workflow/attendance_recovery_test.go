package workflow

import (
	"context"
	"dfolan/internal/activity"
	"dfolan/internal/database"
	"encoding/json"
	"github.com/stretchr/testify/require"
	"testing"
	"time"
)

func eraseCampaignForRegression(t *testing.T, store *database.Store, role database.Character, campaign string) {
	t.Helper()
	_, _, _, err := store.CommitAdventure(context.Background(), role.AccountID, role.ID, "simulate-old-writer:"+campaign, func(r database.Character, p *database.AccountAdventure) (json.RawMessage, json.RawMessage, error) {
		delete(p.Data.Attendance, campaign)
		p.Data.SymbolID = 31
		p.Data.Attendance["other-campaign"] = json.RawMessage(`{"keep":"later-progress"}`)
		return r.State, json.RawMessage(`{}`), nil
	})
	require.NoError(t, err)
}

func TestAttendanceRecoveryRepairsMissingProgressFromOtherCharacterReceipt(t *testing.T) {
	s, a, b := attendanceFixture(t)
	ctx := context.Background()
	first := time.Date(2026, 9, 1, 8, 0, 0, 0, time.UTC)
	_, err := s.Observe(ctx, a, first, "2026-09-01")
	require.NoError(t, err)
	grant, err := s.Claim(ctx, a, first, "2026-09-01", 0)
	require.NoError(t, err)
	require.True(t, grant.CreatedMail)
	eraseCampaignForRegression(t, s.Store, a, s.Rules.CampaignKey())
	require.NoError(t, s.RestoreMissingProgress(ctx, b, first, "2026-09-01"))
	p, err := s.Store.LoadAdventure(ctx, b.AccountID, b.ID, b.Name)
	require.NoError(t, err)
	restored, err := attendanceProgress(p, s.Rules.CampaignKey())
	require.NoError(t, err)
	require.EqualValues(t, 1, restored.Visits)
	require.True(t, restored.Claimed[0])
	require.EqualValues(t, 31, p.Data.SymbolID)
	require.JSONEq(t, `{"keep":"later-progress"}`, string(p.Data.Attendance["other-campaign"]))
	replay, err := s.Claim(ctx, b, first, "2026-09-01", 0)
	require.NoError(t, err)
	require.False(t, replay.CreatedMail)
	ma, err := s.Store.Mailbox(ctx, a.AccountID, a.ID)
	require.NoError(t, err)
	mb, err := s.Store.Mailbox(ctx, b.AccountID, b.ID)
	require.NoError(t, err)
	require.Len(t, ma, 1)
	require.Empty(t, mb)
	next, err := s.Observe(ctx, b, first.AddDate(0, 0, 1), "2026-09-02")
	require.NoError(t, err)
	require.EqualValues(t, 2, next.Progress.Visits)
	// Running repair again must not overwrite the newly accumulated day.
	require.NoError(t, s.RestoreMissingProgress(ctx, a, first.AddDate(0, 0, 1), "2026-09-02"))
	p, err = s.Store.LoadAdventure(ctx, a.AccountID, a.ID, a.Name)
	require.NoError(t, err)
	restored, err = attendanceProgress(p, s.Rules.CampaignKey())
	require.NoError(t, err)
	require.EqualValues(t, 2, restored.Visits)
}

func TestWeeklyRecoveryRestoresPaidPeriodAndCumulativeWithoutNewMail(t *testing.T) {
	s, a, b := routineServiceFixture(t)
	ctx := context.Background()
	s.Rules.Cumulative = []activity.Milestone{{Index: 1, Required: 1, Reward: s.Rules.Weekly[0].Reward}}
	first := time.Date(2026, 10, 8, 8, 0, 0, 0, time.UTC)
	r, err := s.Run(ctx, a, RoutineAction{From: first.Add(-time.Minute), Now: first, GameDay: "2026-10-08", ResetHour: 6, ObservationKey: "native:2:100:101:period:1", ClaimKind: "period", ClaimIndex: 1})
	require.NoError(t, err)
	require.True(t, r.CreatedMail)
	r, err = s.Run(ctx, a, RoutineAction{From: first, Now: first.Add(time.Second), GameDay: "2026-10-08", ResetHour: 6, ObservationKey: "native:2:101:102:cumulative:1", ClaimKind: "cumulative", ClaimIndex: 1})
	require.NoError(t, err)
	require.True(t, r.CreatedMail)
	eraseCampaignForRegression(t, s.Store, a, s.Rules.CampaignKey())
	now := first.AddDate(0, 0, 1)
	require.NoError(t, s.RestoreMissingWeeklyProgress(ctx, b, now, 6))
	p, err := s.Store.LoadAdventure(ctx, b.AccountID, b.ID, b.Name)
	require.NoError(t, err)
	progress, err := routineProgress(p, s.Rules.CampaignKey())
	require.NoError(t, err)
	require.True(t, progress.Claimed["period:1"])
	require.True(t, progress.Claimed["cumulative:1"])
	require.EqualValues(t, 60000, progress.PeriodMillis[1])
	require.Equal(t, now, progress.HighWater)
	r, err = s.Run(ctx, b, RoutineAction{From: now, Now: now, GameDay: "2026-10-09", ResetHour: 6, ObservationKey: "native:3:200:201:period:1", ClaimKind: "period", ClaimIndex: 1})
	require.NoError(t, err)
	require.False(t, r.CreatedMail)
	ma, err := s.Store.Mailbox(ctx, a.AccountID, a.ID)
	require.NoError(t, err)
	mb, err := s.Store.Mailbox(ctx, b.AccountID, b.ID)
	require.NoError(t, err)
	require.Len(t, ma, 2)
	require.Empty(t, mb)
	require.EqualValues(t, 31, p.Data.SymbolID)
}

func TestAttendanceRecoveryCannotReadAnotherAccountsReceipts(t *testing.T) {
	s, a, _ := attendanceFixture(t)
	ctx := context.Background()
	now := time.Date(2026, 9, 1, 8, 0, 0, 0, time.UTC)
	_, err := s.Observe(ctx, a, now, "2026-09-01")
	require.NoError(t, err)
	_, err = s.Claim(ctx, a, now, "2026-09-01", 0)
	require.NoError(t, err)
	account, err := s.Store.DevelopmentAccount(ctx, "unrelated-account")
	require.NoError(t, err)
	other, err := s.Store.CreateCharacter(ctx, database.Character{AccountID: account, Name: "unrelated", Profession: 0, Request: []byte{}, ConfigVersion: a.ConfigVersion, State: json.RawMessage(`{"level":115}`)}, 100)
	require.NoError(t, err)
	require.NoError(t, s.RestoreMissingProgress(ctx, other, now, "2026-09-01"))
	p, err := s.Store.LoadAdventure(ctx, other.AccountID, other.ID, other.Name)
	require.NoError(t, err)
	require.Empty(t, p.Data.Attendance[s.Rules.CampaignKey()])
}
