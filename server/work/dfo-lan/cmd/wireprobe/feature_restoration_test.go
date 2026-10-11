package main

import (
	"context"
	"dfolan/internal/activity"
	"dfolan/internal/character"
	"dfolan/internal/featurebridge"
	"dfolan/internal/inventory"
	"dfolan/internal/workflow"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestRestoredActivityUsesR2CalendarInsteadOfFatigueReset(t *testing.T) {
	w, now, _ := weeklyGatewayFixture(t)
	w.attendanceCalendar = w.fatigue
	w.fatigue = &character.FatigueService{Location: time.UTC, Rules: character.FatigueRules{ResetHour: 0}}
	r := w.weekly.Rules.Weekly[0].Reward
	rules := activity.RoutineRules{Kind: "online", Table: "restoration-calendar", SourceChecksum: w.weekly.Rules.SourceChecksum, Dates: w.weekly.Rules.Dates, OnlineDaily: map[string][]activity.OnlineReward{}, OnlineMaximum: map[string]uint32{"weekday": 14400, "weekend": 14400}, OnlinePeriod: []activity.OnlineReward{{Index: 1, Milliseconds: 5000, Reward: r}}}
	for _, key := range []string{"weekday", "weekend"} {
		rules.OnlineDaily[key] = []activity.OnlineReward{{Index: 1, Milliseconds: 1000, Reward: r}}
	}
	var err error
	w.online, err = workflow.NewRoutineService(w.weekly.Store, rules, w.weekly.Items)
	require.NoError(t, err)
	ctx := context.Background()
	_, err = w.onlineStatus(ctx, now.Add(23*time.Hour+59*time.Minute), "", 0)
	require.NoError(t, err)
	p, err := w.onlineStatus(ctx, now.Add(24*time.Hour), "", 0)
	require.NoError(t, err)
	require.Equal(t, "2026-10-09", p.Day)
	require.Equal(t, "2026-10-08", w.fatigue.Day(now.Add(24*time.Hour)))
	require.Zero(t, w.fatigue.Rules.ResetHour)
}

func TestRestoredBridgeKeepsR2SymbolDispatchWithoutActivityCalendar(t *testing.T) {
	w, now, _ := weeklyGatewayFixture(t)
	w.fatigue = nil
	w.attendanceCalendar = nil
	w.items = &inventory.ItemService{SymbolSockets: &inventory.SymbolSocketLayout{}}
	client := &gameConnection{bootstrapped: true, selectedCharacterID: w.role.ID, worldState: w}
	call := featurebridge.Call{Request: featurebridge.Frame{Kind: featurebridge.SymbolSnapshot, ID: 11, Body: []byte{1}}, Reply: make(chan featurebridge.Frame, 1)}
	client.featureCall(call, now)
	reply := <-call.Reply
	// SymbolSnapshot must reach the existing symbol validator; the activity
	// calendar must not disable the native emblem route.
	require.Equal(t, featurebridge.StatusRefused, reply.Status)
	require.EqualValues(t, 11, reply.ID)
	call.Request.Kind = featurebridge.Snapshot
	call.Request.Body = nil
	client.featureCall(call, now)
	require.Equal(t, featurebridge.StatusNotReady, (<-call.Reply).Status)
}
