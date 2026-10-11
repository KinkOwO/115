package main

import (
	"context"
	"dfolan/internal/activity"
	"dfolan/internal/character"
	"dfolan/internal/database"
	"dfolan/internal/featurebridge"
	"dfolan/internal/gamedata"
	"dfolan/internal/workflow"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"github.com/stretchr/testify/require"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestFeatureDailyBridgeIndependentOfWeeklyAndBindsClaims(t *testing.T) {
	w, now, n := weeklyGatewayFixture(t)
	r := w.weekly.Rules.Weekly[0].Reward
	rules := activity.RoutineRules{Kind: "daily", Table: "daily-owned", SourceChecksum: w.weekly.Rules.SourceChecksum, Dates: w.weekly.Rules.Dates, SequentialDaily: true, ProperDungeonCount: 2, Daily: []activity.Milestone{{Index: 1, Reward: r}, {Index: 2, Reward: r}}}
	var e error
	w.daily, e = workflow.NewRoutineService(w.weekly.Store, rules, w.weekly.Items)
	require.NoError(t, e)
	w.weekly = nil
	client := &gameConnection{bootstrapped: true, selectedCharacterID: w.role.ID, worldState: w}
	run := func(kind uint16, body []byte, at time.Time) featurebridge.Frame {
		c := featurebridge.Call{Request: featurebridge.Frame{Kind: kind, ID: 1, Body: body}, Reply: make(chan featurebridge.Frame, 1)}
		client.featureCall(c, at)
		return <-c.Reply
	}
	require.EqualValues(t, 0, run(featurebridge.Snapshot, nil, now).Status)
	claim := make([]byte, 104)
	claim[0] = featurebridge.Daily
	claim[1] = featurebridge.Day
	binary.LittleEndian.PutUint32(claim[4:], 1)
	copy(claim[8:], rules.SourceChecksum)
	copy(claim[72:], rules.CampaignKey()[:32])
	require.Equal(t, featurebridge.StatusRefused, run(featurebridge.Claim, claim, now.Add(time.Second)).Status)
	require.Zero(t, *n)
	require.NoError(t, w.daily.Store.MigrateAdventure(context.Background()))
	require.NoError(t, w.daily.BindSequentialClears(w.fatigue.Location, 6))
	for i := 0; i < 2; i++ {
		at := now.Add(time.Duration(i+2) * time.Second)
		runID := fmt.Sprint("daily-bridge", i)
		receipt := character.ClearReceipt{Source: w.role.ConfigVersion, Run: runID, RecommendedDungeonClear: true, DungeonID: 1001, CharacterLevel: 115, CompletedAt: at}
		_, _, e = w.daily.Store.CommitCharacterEvent(context.Background(), w.account, w.role.ID, w.role.ConfigVersion, "clear:"+runID, "daily-bridge", func(current database.Character) (json.RawMessage, json.RawMessage, error) {
			raw, e := json.Marshal(receipt)
			return current.State, raw, e
		})
		require.NoError(t, e)
	}
	require.EqualValues(t, 0, run(featurebridge.Claim, claim, now.Add(4*time.Second)).Status)
	require.Equal(t, 1, *n)
	require.EqualValues(t, 0, run(featurebridge.Claim, claim, now.Add(5*time.Second)).Status)
	require.Equal(t, 1, *n)
	claim[1] = featurebridge.Period
	require.Equal(t, featurebridge.StatusRefused, run(featurebridge.Claim, claim, now.Add(6*time.Second)).Status)
}
func TestFeatureDailyMaterialSourceAllClaims(t *testing.T) {
	archive := os.Getenv("US115_FEATURE_DAILY_PVF")
	if archive == "" {
		t.Skip("set US115_FEATURE_DAILY_PVF to this task's isolated daily extension PVF")
	}
	c, e := gamedata.PrepareCatalogs(gamedata.CatalogInputs{Selection: "items,attendance-daily", ArchivePath: archive, DerivedCacheDir: "-"}, gamedata.CatalogAdapters{})
	require.NoError(t, e)
	ctx := context.Background()
	store, e := database.Open(ctx, database.Config{SQLitePath: filepath.Join(t.TempDir(), "daily-source.sqlite3")})
	require.NoError(t, e)
	defer store.Close()
	account, e := store.DevelopmentAccount(ctx, "daily-source")
	require.NoError(t, e)
	role, e := store.CreateCharacter(ctx, database.Character{AccountID: account, Name: "DailySource", Request: []byte{}, ConfigVersion: c.Items.Source.SaveIdentity(), State: json.RawMessage(`{"level":115}`)}, 100)
	require.NoError(t, e)
	svc, e := workflow.NewRoutineService(store, *c.AttendanceDaily, *c.Items)
	require.NoError(t, e)
	loc := time.FixedZone("HK", 8*3600)
	require.NoError(t, store.MigrateAdventure(ctx))
	require.NoError(t, svc.BindSequentialClears(loc, 6))
	w := &worldSession{daily: svc, account: account, role: role, fatigue: &character.FatigueService{Rules: character.FatigueRules{ResetHour: 6}, Location: loc}}
	start, e := time.ParseInLocation("2006-01-02 15:04:05", svc.Rules.Dates.Start, loc)
	require.NoError(t, e)
	claims := 0
	for d := 0; d < len(svc.Rules.Daily); d++ {
		now := start.AddDate(0, 0, d)
		for i := uint32(0); i < svc.Rules.ProperDungeonCount; i++ {
			at := now.Add(time.Duration(i+1) * time.Second)
			run := fmt.Sprintf("source-day-%d-clear-%d", d, i)
			receipt := character.ClearReceipt{Source: role.ConfigVersion, Run: run, RecommendedDungeonClear: true, DungeonID: 1001, CharacterLevel: 115, CompletedAt: at}
			commit := func() (database.Character, bool, error) {
				return store.CommitCharacterEvent(ctx, account, role.ID, role.ConfigVersion, "clear:"+run, "daily-source", func(current database.Character) (json.RawMessage, json.RawMessage, error) {
					raw, e := json.Marshal(receipt)
					return current.State, raw, e
				})
			}
			_, applied, e := commit()
			require.NoError(t, e)
			require.True(t, applied)
			_, applied, e = commit()
			require.NoError(t, e)
			require.False(t, applied)
		}
		now = now.Add(time.Minute)
		view, e := w.dailyView(ctx, now)
		require.NoError(t, e)
		require.Equal(t, featurebridge.Claimable, view.Rows[d].State)
		for _, row := range view.Rows {
			if row.State != featurebridge.Claimable {
				continue
			}
			kind := "day"
			if row.Group == featurebridge.Cumulative {
				kind = "cumulative"
			}
			_, e = w.dailyStatus(ctx, now, kind, row.Index)
			require.NoError(t, e)
			_, e = w.dailyStatus(ctx, now, kind, row.Index)
			require.NoError(t, e)
			claims++
		}
		// Refresh cumulative goals after today's day reward commits.
		view, e = w.dailyView(ctx, now)
		require.NoError(t, e)
		for _, row := range view.Rows {
			if row.State == featurebridge.Claimable {
				require.Equal(t, featurebridge.Cumulative, row.Group)
				_, e = w.dailyStatus(ctx, now, "cumulative", row.Index)
				require.NoError(t, e)
				claims++
			}
		}
	}
	box, e := store.Mailbox(ctx, account, role.ID)
	require.NoError(t, e)
	require.Len(t, box, 13)
	require.Equal(t, 13, claims)
	t.Logf("daily source=%s days=10 cumulative=3 mails=%d clear_replays_no_credit=true storage=temporary-sqlite", svc.Rules.SourceChecksum, len(box))
}
