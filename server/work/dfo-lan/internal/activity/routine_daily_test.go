package activity

import (
	"github.com/stretchr/testify/require"
	"testing"
	"time"
)

func TestSequentialDailyResetSkipAndOneCompletionPerDay(t *testing.T) {
	c := routineFixture("daily")
	c.SequentialDaily = true
	for i := range c.Daily {
		c.Daily[i].Dates = Dates{}
	}
	require.NoError(t, c.Validate())
	now := time.Date(2026, 10, 8, 8, 0, 0, 0, time.UTC)
	var p RoutineProgress
	observe := func(at time.Time) {
		var e error
		p, e = c.Observe(p, at, at, at.Add(-6*time.Hour).Format("2006-01-02"), 6, 115)
		require.NoError(t, e)
	}
	observe(now)
	p = c.ProperClear(p, now)
	require.EqualValues(t, 1, p.Clears[1])
	require.False(t, c.DailyEligible(p, 1))
	// Missed days retain completed sequence only, never incomplete daily clears.
	now = now.AddDate(0, 0, 2)
	observe(now)
	require.Zero(t, p.Clears[1])
	p = c.ProperClear(p, now)
	p = c.ProperClear(p, now)
	require.True(t, c.DailyEligible(p, 1))
	require.Equal(t, "2026-10-10", p.SequenceDay)
	for i := 0; i < 10; i++ {
		p = c.ProperClear(p, now)
	}
	require.Zero(t, p.Clears[2])
	require.NoError(t, c.ValidateProgress(p, time.UTC, 6))
	beforeReset := time.Date(2026, 10, 11, 5, 59, 59, 0, time.UTC)
	observe(beforeReset)
	p = c.ProperClear(p, beforeReset)
	require.Zero(t, p.Clears[2])
	atReset := beforeReset.Add(time.Second)
	observe(atReset)
	p = c.ProperClear(p, atReset)
	p = c.ProperClear(p, atReset)
	require.True(t, c.DailyEligible(p, 2))
	// Completed rewards remain claimable after a later missed day; each is once.
	now = atReset.AddDate(0, 0, 1)
	observe(now)
	var grant bool
	var e error
	p, _, grant, e = c.Claim(p, now, "day", 1)
	require.NoError(t, e)
	require.True(t, grant)
	_, _, grant, e = c.Claim(p, now, "day", 1)
	require.NoError(t, e)
	require.False(t, grant)
}
func TestSequentialDailySourceThresholdAndCorruptSave(t *testing.T) {
	c := routineFixture("daily")
	c.SequentialDaily = true
	c.ProperDungeonCount = 3
	for i := range c.Daily {
		c.Daily[i].Dates = Dates{}
	}
	now := time.Date(2026, 10, 8, 8, 0, 0, 0, time.UTC)
	p, e := c.Observe(RoutineProgress{}, now, now, "2026-10-08", 6, 115)
	require.NoError(t, e)
	p = c.ProperClear(p, now)
	p = c.ProperClear(p, now)
	require.False(t, c.DailyEligible(p, 1))
	p = c.ProperClear(p, now)
	require.True(t, c.DailyEligible(p, 1))
	require.NoError(t, c.ValidateProgress(p, time.UTC, 6))
	p.Observed[2] = true
	require.Error(t, c.ValidateProgress(p, time.UTC, 6))
	p.Observed[2] = false
	p.SequenceDay = "2026-10-09"
	require.Error(t, c.ValidateProgress(p, time.UTC, 6))
}
