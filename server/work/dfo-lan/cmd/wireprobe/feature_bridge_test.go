package main

import (
	"context"
	"dfolan/internal/featurebridge"
	"encoding/binary"
	"github.com/stretchr/testify/require"
	"strings"
	"testing"
	"time"
)

func TestFeatureBridgeBindsOwnedRoleAndSourceAndDeduplicates(t *testing.T) {
	w, now, n := weeklyGatewayFixture(t)
	client := &gameConnection{bootstrapped: true, selectedCharacterID: w.role.ID, worldState: w}
	run := func(kind uint16, body []byte, at time.Time) featurebridge.Frame {
		c := featurebridge.Call{Request: featurebridge.Frame{Kind: kind, ID: 1, Body: body}, Reply: make(chan featurebridge.Frame, 1)}
		client.featureCall(c, at)
		return <-c.Reply
	}
	r := run(featurebridge.Snapshot, nil, now)
	require.EqualValues(t, 0, r.Status)
	require.Contains(t, string(r.Body), w.weekly.Rules.SourceChecksum)
	claim := make([]byte, 104)
	claim[0] = featurebridge.Weekly
	claim[1] = featurebridge.Period
	binary.LittleEndian.PutUint32(claim[4:], 1)
	copy(claim[8:], w.weekly.Rules.SourceChecksum)
	copy(claim[72:], w.weekly.Rules.CampaignKey()[:32])
	r = run(featurebridge.Claim, claim, now.Add(time.Minute))
	require.EqualValues(t, 0, r.Status)
	require.Equal(t, 1, *n)
	r = run(featurebridge.Claim, claim, now.Add(61*time.Second))
	require.EqualValues(t, 0, r.Status)
	require.Equal(t, 1, *n)
	before, e := w.weekly.Store.Mailbox(context.Background(), w.account, w.role.ID)
	require.NoError(t, e)
	require.Len(t, before, 1)
	copy(claim[8:], strings.Repeat("f", 64))
	r = run(featurebridge.Claim, claim, now.Add(62*time.Second))
	require.Equal(t, featurebridge.StatusRefused, r.Status)
	client.selectedCharacterID++
	r = run(featurebridge.Snapshot, nil, now.Add(63*time.Second))
	require.Equal(t, featurebridge.StatusNotReady, r.Status)
}
