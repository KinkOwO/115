package main

import (
	"context"
	"dfolan/internal/activity"
	"dfolan/internal/character"
	"dfolan/internal/database"
	"dfolan/internal/featurebridge"
	"dfolan/internal/gamedata"
	"dfolan/internal/workflow"
	"encoding/json"
	"github.com/stretchr/testify/require"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestFeatureOnlineCreditsClosedWindowAndCapsAndReconnects(t *testing.T) {
	w, now, n := weeklyGatewayFixture(t)
	r := w.weekly.Rules.Weekly[0].Reward
	rules := activity.RoutineRules{Kind: "online", Table: "online-extension-fixture", SourceChecksum: w.weekly.Rules.SourceChecksum, Dates: w.weekly.Rules.Dates, OnlineDaily: map[string][]activity.OnlineReward{}, OnlineMaximum: map[string]uint32{"weekday": 4, "weekend": 4}, OnlinePeriod: []activity.OnlineReward{{Index: 1, Milliseconds: 5000, Reward: r}}}
	for _, key := range []string{"weekday", "weekend"} {
		rules.OnlineDaily[key] = []activity.OnlineReward{{Index: 1, Milliseconds: 1000, Reward: r}, {Index: 1, Milliseconds: 1000, Reward: r}}
	}
	var e error
	w.online, e = workflow.NewRoutineService(w.weekly.Store, rules, w.weekly.Items)
	require.NoError(t, e)
	ctx := context.Background()
	_, e = w.onlineStatus(ctx, now, "", 0)
	require.NoError(t, e)
	c, e := w.onlineView(ctx, now.Add(10*time.Second))
	require.NoError(t, e)
	require.EqualValues(t, 4, c.Rows[0].Progress)
	require.Equal(t, featurebridge.Claimable, c.Rows[0].State)
	require.Equal(t, featurebridge.Locked, c.Rows[2].State)
	_, e = w.onlineStatus(ctx, now.Add(11*time.Second), "day", 1)
	require.NoError(t, e)
	require.Equal(t, 1, *n)
	_, e = w.onlineStatus(ctx, now.Add(12*time.Second), "day", 1)
	require.NoError(t, e)
	require.Equal(t, 1, *n)
	w.onlineClock = time.Time{}
	w.onlineRole = 0
	p, e := w.onlineStatus(ctx, now.Add(time.Hour), "", 0)
	require.NoError(t, e)
	require.EqualValues(t, 4000, p.TotalMillis)
	next := now.AddDate(0, 0, 1)
	w.onlineClock = time.Time{}
	_, e = w.onlineStatus(ctx, next, "", 0)
	require.NoError(t, e)
	c, e = w.onlineView(ctx, next.Add(2*time.Second))
	require.NoError(t, e)
	require.EqualValues(t, 2, c.Rows[0].Progress)
	require.Equal(t, featurebridge.Claimable, c.Rows[0].State)
	require.Equal(t, featurebridge.Claimable, c.Rows[2].State)
	_, e = w.onlineStatus(ctx, next.Add(3*time.Second), "period", 1)
	require.NoError(t, e)
	require.Equal(t, 2, *n)
}

func TestFeatureOnlineMaterialSourceAllClaims(t *testing.T) {
	archive := os.Getenv("US115_FEATURE_PVF")
	if archive == "" {
		t.Skip("set US115_FEATURE_PVF to this task's isolated extension PVF")
	}
	c, e := gamedata.PrepareCatalogs(gamedata.CatalogInputs{Selection: "items,attendance-online,attendance-weekly", ArchivePath: archive, DerivedCacheDir: "-"}, gamedata.CatalogAdapters{})
	require.NoError(t, e)
	ctx := context.Background()
	store, e := database.Open(ctx, database.Config{SQLitePath: filepath.Join(t.TempDir(), "actual-online.sqlite3")})
	require.NoError(t, e)
	defer store.Close()
	account, e := store.DevelopmentAccount(ctx, "online-source")
	require.NoError(t, e)
	role, e := store.CreateCharacter(ctx, database.Character{AccountID: account, Name: "OnlineSource", Request: []byte{}, ConfigVersion: c.Items.Source.SaveIdentity(), State: json.RawMessage(`{"level":115}`)}, 100)
	require.NoError(t, e)
	svc, e := workflow.NewRoutineService(store, *c.AttendanceOnline, *c.Items)
	require.NoError(t, e)
	loc := time.FixedZone("HK", 8*3600)
	w := &worldSession{online: svc, account: account, role: role, fatigue: &character.FatigueService{Rules: character.FatigueRules{ResetHour: 6}, Location: loc}}
	start, e := time.ParseInLocation("2006-01-02 15:04:05", svc.Rules.Dates.Start, loc)
	require.NoError(t, e)
	claims := 0
	period := uint64(0)
	claimed := map[uint32]bool{}
	for d := 0; d < 8; d++ {
		now := start.AddDate(0, 0, d)
		w.onlineClock = time.Time{}
		w.onlineRole = 0
		_, e = w.onlineStatus(ctx, now, "", 0)
		require.NoError(t, e)
		now = now.Add(4 * time.Hour)
		p, e := w.onlineStatus(ctx, now, "", 0)
		require.NoError(t, e)
		require.EqualValues(t, 14400000, p.DailyMillis)
		view, e := w.onlineView(ctx, now)
		require.NoError(t, e)
		for _, r := range view.Rows {
			if r.Group == featurebridge.Day {
				_, e = w.onlineStatus(ctx, now, "day", r.Index)
				require.NoError(t, e)
				claims++
				_, e = w.onlineStatus(ctx, now, "day", r.Index)
				require.NoError(t, e)
			}
		}
		period = 0
		for i, r := range svc.Rules.OnlinePeriod {
			period += uint64(r.Milliseconds)
			index := uint32(i + 1)
			if p.TotalMillis >= period && !claimed[index] {
				_, e = w.onlineStatus(ctx, now, "period", index)
				require.NoError(t, e)
				claimed[index] = true
				claims++
			}
		}
	}
	box, e := store.Mailbox(ctx, account, role.ID)
	require.NoError(t, e)
	require.Len(t, box, 36)
	require.Equal(t, 36, claims)
	t.Logf("online source=%s daily_claims=32 period_claims=4 mails=%d offline_time_credited=false storage=temporary-sqlite", svc.Rules.SourceChecksum, len(box))
}
