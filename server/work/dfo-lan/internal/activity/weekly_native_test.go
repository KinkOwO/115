package activity

import (
	"github.com/stretchr/testify/require"
	"testing"
	"time"
)

func TestWeeklyNativeProjectionUsesSourceGoalsAndClaimMasks(t *testing.T) {
	c := routineFixture("weekly")
	c.Cumulative[0].Index = 2
	now := time.Date(2026, 10, 8, 7, 0, 0, 0, time.UTC)
	p, err := c.Observe(RoutineProgress{}, now.Add(-time.Hour), now, "2026-10-08", 6, 115)
	require.NoError(t, err)
	v, err := c.WeeklyNativeView(p, now, 6)
	require.NoError(t, err)
	require.EqualValues(t, 1, v.CurrentPeriod)
	require.EqualValues(t, 3600, v.Seconds)
	require.EqualValues(t, 2, v.EligiblePeriods)
	require.Zero(t, v.ClaimedCumulative)
	// Altering the selected source goal changes the eligibility projection.
	c.Weekly[0].Required = 3601
	v, err = c.WeeklyNativeView(p, now, 6)
	require.NoError(t, err)
	require.Zero(t, v.EligiblePeriods)
	c.Weekly[0].Required = 3600
	p, _, _, err = c.Claim(p, now, "period", 1)
	require.NoError(t, err)
	v, err = c.WeeklyNativeView(p, now, 6)
	require.NoError(t, err)
	require.EqualValues(t, 2, v.ClaimedPeriods)
	next := time.Date(2026, 10, 15, 7, 0, 0, 0, time.UTC)
	p, err = c.Observe(p, next.Add(-time.Hour), next, "2026-10-15", 6, 115)
	require.NoError(t, err)
	p, _, _, err = c.Claim(p, next, "period", 2)
	require.NoError(t, err)
	p, _, _, err = c.Claim(p, next, "cumulative", 2)
	require.NoError(t, err)
	v, err = c.WeeklyNativeView(p, next, 6)
	require.NoError(t, err)
	require.EqualValues(t, 2, v.CurrentPeriod)
	require.EqualValues(t, 6, v.ClaimedPeriods)
	require.EqualValues(t, 4, v.ClaimedCumulative)
	p.PeriodMillis[2] = uint64(^uint32(0))*1000 + 1
	_, err = c.WeeklyNativeView(p, next, 6)
	require.Error(t, err)
}

func TestWeeklyRejectsUnimplementedNativeSourceFamilies(t *testing.T) {
	for _, tag := range []string{"[accu period reward info]", "[bonus period reward info]"} {
		tokens := fixture(t, "weekly")
		tokens[0].Text = tag
		_, err := ParseWeekly(tokens)
		require.ErrorContains(t, err, "requires its own execution")
	}
}
