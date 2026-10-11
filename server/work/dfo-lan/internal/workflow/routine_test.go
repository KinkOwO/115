package workflow

import (
	"context"
	"dfolan/internal/activity"
	"dfolan/internal/database"
	"dfolan/internal/mail"
	"encoding/json"
	"fmt"
	"github.com/stretchr/testify/require"
	"sync"
	"testing"
	"time"
)

func routineServiceFixture(t *testing.T) (*RoutineService, database.Character, database.Character) {
	login, a, b := attendanceFixture(t)
	r := login.Rules.Days[0].Rewards[0]
	r.MailTitle = "Bound source"
	r.MailMessage = "Bound source message"
	c := activity.RoutineRules{Kind: "weekly", Table: "routine-fixture", SourceChecksum: login.Rules.SourceChecksum, Dates: activity.Dates{Start: "2026-10-08 06:00:00", End: "2026-10-15 06:00:00"}, Weekly: []activity.Milestone{{Index: 1, Required: 60, Dates: activity.Dates{Start: "2026-10-08 06:00:00", End: "2026-10-15 06:00:00"}, Reward: r}}}
	s, e := NewRoutineService(login.Store, c, login.Items)
	require.NoError(t, e)
	return s, a, b
}
func TestRoutineConcurrentAccountClaimsAndReceiptReplay(t *testing.T) {
	s, a, b := routineServiceFixture(t)
	now := time.Date(2026, 10, 8, 8, 0, 0, 0, time.UTC)
	ctx := context.Background()
	var wg sync.WaitGroup
	results := make(chan RoutineResult, 2)
	errs := make(chan error, 2)
	for i, r := range []database.Character{a, b} {
		wg.Add(1)
		go func(i int, r database.Character) {
			defer wg.Done()
			v, e := s.Run(ctx, r, RoutineAction{From: now.Add(-time.Minute), Now: now, GameDay: "2026-10-08", ResetHour: 6, ObservationKey: fmt.Sprint(i), ClaimKind: "period", ClaimIndex: 1})
			results <- v
			errs <- e
		}(i, r)
	}
	wg.Wait()
	close(results)
	close(errs)
	for e := range errs {
		require.NoError(t, e)
	}
	created := 0
	for r := range results {
		if r.CreatedMail {
			created++
		}
		require.EqualValues(t, 60000, r.Progress.PeriodMillis[1])
	}
	require.Equal(t, 1, created)
	r, e := s.Run(ctx, a, RoutineAction{From: now.Add(-time.Minute), Now: now, GameDay: "2026-10-08", ResetHour: 6, ObservationKey: "0", ClaimKind: "period", ClaimIndex: 1})
	require.NoError(t, e)
	require.False(t, r.CreatedMail)
	ma, e := s.Store.Mailbox(ctx, a.AccountID, a.ID)
	require.NoError(t, e)
	mb, e := s.Store.Mailbox(ctx, b.AccountID, b.ID)
	require.NoError(t, e)
	require.Equal(t, 1, len(ma)+len(mb))
}
func TestRoutineFullMailboxRollsBackTimeAndClaim(t *testing.T) {
	s, a, _ := routineServiceFixture(t)
	ctx := context.Background()
	now := time.Date(2026, 10, 8, 8, 0, 0, 0, time.UTC)
	for i := 0; i < mail.MaxMessages; i++ {
		_, _, e := s.Store.CommitSystemMail(ctx, a.AccountID, a.ID, a.ConfigVersion, fmt.Sprintf("routine-fill:%d", i), "fixture", "fill", "fill", nil)
		require.NoError(t, e)
	}
	_, e := s.Run(ctx, a, RoutineAction{From: now.Add(-time.Minute), Now: now, GameDay: "2026-10-08", ResetHour: 6, ObservationKey: "full", ClaimKind: "period", ClaimIndex: 1})
	require.ErrorIs(t, e, database.ErrMailFull)
	p, e := s.Store.LoadAdventure(ctx, a.AccountID, a.ID, a.Name)
	require.NoError(t, e)
	require.Empty(t, p.Data.Attendance)
}

func TestRoutineObservationPreservesUnknownCampaignAndSiblingFields(t *testing.T) {
	s, role, _ := routineServiceFixture(t)
	ctx := context.Background()
	campaign := s.Rules.CampaignKey()
	_, _, _, e := s.Store.CommitAdventureTx(ctx, role.AccountID, role.ID, "seed-routine-extension", func(_ *database.Tx, current database.Character, p *database.AccountAdventure) (json.RawMessage, json.RawMessage, error) {
		p.Data.Attendance = map[string]json.RawMessage{
			campaign:             json.RawMessage(`{"version":0,"extension":{"future":17}}`),
			"unrelated-campaign": json.RawMessage(`{"untouched":[1,2,3]}`),
		}
		return current.State, json.RawMessage(`{}`), nil
	})
	require.NoError(t, e)
	now := time.Date(2026, 10, 8, 8, 0, 0, 0, time.UTC)
	_, e = s.Run(ctx, role, RoutineAction{From: now.Add(-time.Minute), Now: now, GameDay: "2026-10-08", ResetHour: 6, ObservationKey: "preserve"})
	require.NoError(t, e)
	p, e := s.Store.LoadAdventure(ctx, role.AccountID, role.ID, role.Name)
	require.NoError(t, e)
	require.JSONEq(t, `{"untouched":[1,2,3]}`, string(p.Data.Attendance["unrelated-campaign"]))
	var fields map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(p.Data.Attendance[campaign], &fields))
	require.JSONEq(t, `{"future":17}`, string(fields["extension"]))
	require.JSONEq(t, `{"1":60000}`, string(fields["period_millis"]))
}
