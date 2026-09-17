package character

import (
	"testing"
	"time"
)

func TestFatigueDailyBoundary(t *testing.T) {
	s := &FatigueService{Rules: FatigueRules{DailyLimit: 1056, ResetHour: 9}, Location: time.UTC}
	for _, tc := range []struct{ at, want string }{
		{"2026-09-17T08:59:59Z", "2026-09-16"},
		{"2026-09-17T09:00:00Z", "2026-09-17"},
		{"2026-09-18T00:00:00Z", "2026-09-17"},
		{"2026-09-18T09:00:00Z", "2026-09-18"},
	} {
		now, err := time.Parse(time.RFC3339, tc.at)
		if err != nil {
			t.Fatal(err)
		}
		if got := s.day(now); got != tc.want {
			t.Fatalf("%s: got %s want %s", tc.at, got, tc.want)
		}
	}
}
