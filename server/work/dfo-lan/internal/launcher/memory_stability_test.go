package launcher

import (
	"testing"
	"time"
)

// Replay a normal 1228 MiB gateway under sustained host pressure without
// exhausting the test computer. These readings previously stopped it at 30s
// (1675 MiB physical free) or 2s (400 MiB physical free).
func TestMemoryPressureStability(t *testing.T) {
	for _, tc := range []struct {
		name      string
		available uint64
	}{
		{"recorded_1675MiB", 1675 << 20},
		{"critical_400MiB", 400 << 20},
		{"physical_zero", 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var guard memoryPressureGuard
			start := time.Date(2026, 10, 8, 0, 0, 0, 0, time.UTC)
			for elapsed := time.Duration(0); elapsed <= 2*time.Hour; elapsed += 250 * time.Millisecond {
				got := guard.observe(start.Add(elapsed), 1228<<20, tc.available, serverMemoryBudget)
				if got.stopTrigger != "" {
					t.Fatalf("healthy gateway stopped at %s: %s", elapsed, got.stopTrigger)
				}
				want := ""
				if elapsed == 0 {
					want = "warning"
				}
				if got.event != want {
					t.Fatalf("at %s: event=%q want=%q", elapsed, got.event, want)
				}
			}
			recovered := guard.observe(start.Add(2*time.Hour+time.Second), 1228<<20, systemLowMemoryBytes, serverMemoryBudget)
			if recovered.event != "recovered" || recovered.lowFor != 2*time.Hour+time.Second {
				t.Fatalf("recovery lost: %+v", recovered)
			}
		})
	}
	for _, budget := range []uint64{serverMemoryBudget, clientMemoryBudget} {
		t.Run(time.Duration(budget).String(), func(t *testing.T) {
			var guard memoryPressureGuard
			now := time.Now()
			if got := guard.observe(now, budget-budget/16, 0, budget); got.stopTrigger != "" {
				t.Fatalf("boundary changed: %+v", got)
			}
			if got := guard.observe(now.Add(time.Second), budget-budget/16+1, 0, budget); got.stopTrigger != "process_commit_limit" {
				t.Fatalf("own commit protection lost: %+v", got)
			}
		})
	}
}
