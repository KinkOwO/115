package activity

import (
	"github.com/stretchr/testify/require"
	"strings"
	"testing"
	"time"
)

func routineFixture(kind string) RoutineRules {
	r := Reward{Item: 10001, Count: 3, MailTitle: "source title", MailMessage: "source message"}
	c := RoutineRules{Kind: kind, Table: "fixture", SourceChecksum: strings.Repeat("a", 64), Dates: Dates{Start: "2026-10-08 06:00:00", End: "2026-10-22 06:00:00"}, MinimumLevel: 1}
	switch kind {
	case "weekly":
		c.SecondsPerProperClear = 60
		c.Weekly = []Milestone{{Index: 1, Required: 3600, Dates: Dates{Start: c.Dates.Start, End: "2026-10-15 06:00:00"}, Reward: r}, {Index: 2, Required: 3600, Dates: Dates{Start: "2026-10-15 06:00:00", End: c.Dates.End}, Reward: r}}
		c.Cumulative = []Milestone{{Index: 1, Required: 2, Reward: r}}
	case "daily":
		c.ProperDungeonCount = 2
		c.Daily = []Milestone{{Index: 1, Dates: Dates{Start: c.Dates.Start, End: "2026-10-09 06:00:00"}, Reward: r}, {Index: 2, Dates: Dates{Start: "2026-10-09 06:00:00", End: "2026-10-10 06:00:00"}, Reward: r}}
	case "online":
		c.OnlineDaily = map[string][]OnlineReward{"weekday": {{Milliseconds: 60000, Reward: r}}, "weekend": {{Milliseconds: 60000, Reward: r}}}
		c.OnlineMaximum = map[string]uint32{"weekday": 120, "weekend": 180}
		c.OnlinePeriod = []OnlineReward{{Milliseconds: 120000, Reward: r}, {Milliseconds: 120000, Reward: r}}
	}
	return c
}
func TestRoutineConnectedIntervalsAndResetCap(t *testing.T) {
	c := routineFixture("online")
	now := time.Date(2026, 10, 9, 5, 59, 30, 0, time.UTC)
	p, e := c.Observe(RoutineProgress{}, now.Add(-110*time.Second), now, "2026-10-08", 6, 115)
	require.NoError(t, e)
	require.EqualValues(t, 110000, p.DailyMillis)
	// Another account session reports the same interval; it cannot double credit.
	p, e = c.Observe(p, now.Add(-110*time.Second), now, "2026-10-08", 6, 115)
	require.NoError(t, e)
	require.EqualValues(t, 110000, p.TotalMillis)
	end := now.Add(time.Minute)
	p, e = c.Observe(p, now, end, "2026-10-09", 6, 115)
	require.NoError(t, e)
	require.EqualValues(t, 150000, p.TotalMillis)
	require.EqualValues(t, 30000, p.DailyMillis)
	// Reconnecting one hour later starts a fresh connected interval, not from save time.
	start := end.Add(time.Hour)
	p, e = c.Observe(p, start, start.Add(10*time.Second), "2026-10-09", 6, 115)
	require.NoError(t, e)
	require.EqualValues(t, 160000, p.TotalMillis)
	_, e = c.Observe(p, start, start, "2026-10-08", 6, 115)
	require.Error(t, e)
}
func TestRoutineWeeklyBoundaryAndCumulativeClaims(t *testing.T) {
	c := routineFixture("weekly")
	now := time.Date(2026, 10, 15, 6, 0, 30, 0, time.UTC)
	p, e := c.Observe(RoutineProgress{}, now.Add(-time.Minute), now, "2026-10-15", 6, 115)
	require.NoError(t, e)
	require.EqualValues(t, 30000, p.PeriodMillis[1])
	require.EqualValues(t, 30000, p.PeriodMillis[2])
	p = c.ProperClear(p, now)
	require.EqualValues(t, 90000, p.PeriodMillis[2])
	_, _, _, e = c.Claim(p, now, "cumulative", 1)
	require.Error(t, e)
	for _, i := range []uint32{1, 2} {
		p.PeriodMillis[i] = 3600000
		var granted bool
		p, _, granted, e = c.Claim(p, now, "period", i)
		require.NoError(t, e)
		require.True(t, granted)
	}
	p, r, granted, e := c.Claim(p, now, "cumulative", 1)
	require.NoError(t, e)
	require.True(t, granted)
	require.EqualValues(t, 3, r.Count)
	_, _, granted, e = c.Claim(p, now, "cumulative", 1)
	require.NoError(t, e)
	require.False(t, granted)
}
func TestRoutineDailyOfflineDaysDoNotBecomeEligible(t *testing.T) {
	c := routineFixture("daily")
	now := time.Date(2026, 10, 9, 8, 0, 0, 0, time.UTC)
	p, e := c.Observe(RoutineProgress{}, now, now, "2026-10-09", 6, 115)
	require.NoError(t, e)
	require.False(t, p.Observed[1])
	require.True(t, p.Observed[2])
	p = c.ProperClear(p, now)
	require.False(t, c.DailyEligible(p, 2))
	p = c.ProperClear(p, now)
	require.True(t, c.DailyEligible(p, 2))
	require.False(t, c.DailyEligible(p, 1))
}

func TestRoutineOnlineDailyRewardsRequireSuccessiveIntervals(t *testing.T) {
	c := routineFixture("online")
	r := c.OnlineDaily["weekday"][0].Reward
	for _, kind := range []string{"weekday", "weekend"} {
		c.OnlineDaily[kind] = []OnlineReward{{Milliseconds: 900000, Reward: r}, {Milliseconds: 900000, Reward: r}, {Milliseconds: 900000, Reward: r}, {Milliseconds: 900000, Reward: r}}
		c.OnlineMaximum[kind] = 3600
	}
	now := time.Date(2026, 10, 8, 7, 0, 0, 0, time.UTC)
	p, e := c.Observe(RoutineProgress{}, now.Add(-15*time.Minute), now, "2026-10-08", 6, 115)
	require.NoError(t, e)
	_, _, _, e = c.Claim(p, now, "day", 2)
	require.Error(t, e, "15 minutes must not unlock the 30-minute reward")
	p, _, granted, e := c.Claim(p, now, "day", 1)
	require.NoError(t, e)
	require.True(t, granted)
	end := now.Add(15 * time.Minute)
	p, e = c.Observe(p, now, end, "2026-10-08", 6, 115)
	require.NoError(t, e)
	p, _, granted, e = c.Claim(p, end, "day", 2)
	require.NoError(t, e)
	require.True(t, granted)
	_, _, _, e = c.Claim(p, end, "day", 3)
	require.Error(t, e)
}

func TestRoutineDamagedSaveCannotResetClockOrOverflowCounters(t *testing.T) {
	for _, kind := range []string{"weekly", "daily", "online"} {
		t.Run(kind, func(t *testing.T) {
			c := routineFixture(kind)
			now := time.Date(2026, 10, 8, 7, 0, 0, 0, time.UTC)
			for _, p := range []RoutineProgress{
				{Day: "2026-10-08", TotalMillis: 1},
				{Version: 1, Day: "2026-10-08"},
				{Version: 1, Day: "2026-10-07", HighWater: now},
				{Version: 2},
			} {
				_, err := c.Observe(p, now, now, "2026-10-08", 6, 115)
				require.Error(t, err)
			}
			p, err := c.Observe(RoutineProgress{}, now, now, "2026-10-08", 6, 115)
			require.NoError(t, err)
			switch kind {
			case "weekly":
				p.PeriodMillis[1] = ^uint64(0)
			case "daily":
				p.Clears[1] = c.ProperDungeonCount + 1
			case "online":
				p.DailyMillis = 1
				p.TotalMillis = 0
			}
			_, err = c.Observe(p, now, now.Add(time.Second), "2026-10-08", 6, 115)
			require.Error(t, err, "damaged counters must not be repaired by granting more time")
		})
	}
}

func TestRoutineClaimHandlesAbsentClaimMapAndClosedCampaign(t *testing.T) {
	c := routineFixture("weekly")
	now := time.Date(2026, 10, 8, 7, 0, 0, 0, time.UTC)
	p, err := c.Observe(RoutineProgress{}, now.Add(-time.Hour), now, "2026-10-08", 6, 115)
	require.NoError(t, err)
	// An older valid save may omit an empty claimed object. This used to panic
	// on the first successful claim when called outside the workflow initializer.
	p.Claimed = nil
	p, _, granted, err := c.Claim(p, now, "period", 1)
	require.NoError(t, err)
	require.True(t, granted)
	require.True(t, p.Claimed["period:1"])
	_, _, _, err = c.Claim(p, time.Date(2026, 10, 22, 6, 0, 0, 0, time.UTC), "period", 1)
	require.ErrorIs(t, err, ErrRoutineInactive)
	_, _, _, err = c.Claim(RoutineProgress{PeriodMillis: map[uint32]uint64{1: 3600000}}, now, "period", 1)
	require.Error(t, err, "unobserved counters cannot grant rewards")
}
