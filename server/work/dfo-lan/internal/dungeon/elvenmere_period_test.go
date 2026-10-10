package dungeon

import (
	"testing"
	"time"
)

func TestElvenmereWeekBoundaryIsUTCAndTuesday(t *testing.T) {
	boundary := time.Date(2026, 10, 13, 9, 0, 0, 0, time.UTC)
	for _, location := range []*time.Location{time.UTC, time.FixedZone("HK", 8*3600), time.FixedZone("Pacific", -7*3600)} {
		for _, delta := range []time.Duration{-time.Nanosecond, 0, time.Nanosecond, 6 * 24 * time.Hour} {
			now := boundary.Add(delta).In(location)
			want := boundary.Unix()
			if delta < 0 {
				want = boundary.AddDate(0, 0, -7).Unix()
			}
			if got := ElvenmereWeekStart(now); got != want {
				t.Fatalf("%v: got %d want %d", now, got, want)
			}
		}
	}
}
